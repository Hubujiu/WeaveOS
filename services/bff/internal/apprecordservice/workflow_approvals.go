package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

type WorkflowApprovalsRequest struct {
	Page, PageSize int
	QueryVersion   string
}
type WorkflowApprovalsItem struct {
	AppID    string               `json:"appId"`
	ViewID   string               `json:"viewId"`
	RecordID string               `json:"recordId"`
	Event    WorkflowEventSummary `json:"event"`
}
type WorkflowApprovalsResult struct {
	Items        []WorkflowApprovalsItem `json:"items"`
	Total        int64                   `json:"total"`
	Page         int                     `json:"page"`
	PageSize     int                     `json:"pageSize"`
	QueryVersion string                  `json:"queryVersion"`
}

func (s *Service) SearchWorkflowApprovals(ctx context.Context, p session.Principal, q WorkflowApprovalsRequest) (WorkflowApprovalsResult, error) {
	empty := WorkflowApprovalsResult{}
	if s == nil || s.Pool == nil || s.WorkflowApprovals == nil {
		return empty, ErrUnavailable
	}
	if !workflowID(p.UserID) {
		return empty, session.ErrUnauthorized
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	if _, _, err := appquery.PageWindow(int64(q.Page), int64(q.PageSize)); err != nil {
		return empty, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	binding := inboxBinding{ActorID: p.UserID}
	raw, _ := json.Marshal(binding)
	strategy := &approvalsStrategy{service: s, principal: p, binding: binding, page: querycontext.Page{Number: q.Page, Size: q.PageSize}}
	r, err := querycontext.Execute(ctx, s.WorkflowApprovals, p.SessionRef, q.QueryVersion, raw, strategy.page, strategy)
	if err != nil {
		return empty, err
	}
	return WorkflowApprovalsResult{Items: r.Items, Total: r.Total, Page: q.Page, PageSize: q.PageSize, QueryVersion: r.Version}, nil
}

func newWorkflowApprovalsStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-approvals", generation, querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			var b inboxBinding
			r, ok := validInboxRevision(m.Revision)
			return ok && workflowClosedMetadata(m.Criteria, 128, 1, &b) && workflowID(b.ActorID) && m.View == "workflow-approvals:"+b.ActorID && m.Fingerprint == r.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			_, a := validInboxRevision(old)
			_, b := validInboxRevision(next)
			return a && b && strings.HasPrefix(resource, "workflow-approvals:") && workflowID(strings.TrimPrefix(resource, "workflow-approvals:"))
		},
	})
}

type approvalsStrategy struct {
	service     *Service
	principal   session.Principal
	binding     inboxBinding
	read        *applications.PersonalRead
	page        querycontext.Page
	observation querycontext.Observation[[]WorkflowApprovalsItem]
}

func (s *approvalsStrategy) Resource() string { return "workflow-approvals:" + s.binding.ActorID }
func (s *approvalsStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	r, e := (&applications.Application{Pool: s.service.Pool}).BeginPersonalRead(ctx, s.principal)
	if e != nil {
		return nil, e
	}
	s.read = r
	return r.Tx, nil
}
func (s *approvalsStrategy) validCriteria(raw json.RawMessage) bool {
	var b inboxBinding
	return workflowClosedMetadata(raw, 128, 1, &b) && b == s.binding
}
func (s *approvalsStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	if len(saved) > 0 && !s.validCriteria(saved) {
		return nil, nil, querycontext.ErrExpired
	}
	if !s.validCriteria(incoming) {
		return nil, nil, querycontext.ErrInvalid
	}
	return saved, incoming, nil
}

type approvedCandidate struct {
	item    WorkflowApprovalsItem
	tableID string
	row     historyEventRow
	created time.Time
}

func (s *approvalsStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	_, offset, err := appquery.PageWindow(int64(s.page.Number), int64(s.page.Size))
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/personal-approved/v1\x00"))
	_, _ = h.Write([]byte(s.binding.ActorID))
	s.observation = querycontext.Observation[[]WorkflowApprovalsItem]{Items: make([]WorkflowApprovalsItem, 0, s.page.Size)}
	var beforeID string
	var beforeTime time.Time
	for {
		q := `SELECT e.app_id::text,e.table_id::text,e.record_id::text,
 e.command_id::text,e.instance_id::text,e.flow_id::text,e.view_id::text,e.definition_version,
 e.actor_id::text,e.action,e.outcome,e.sequence,e.schema_version,e.record_version,e.created_at,
 e.proof_id::text,e.version_id::text,e.task_id::text,e.task_epoch,e.node_id::text,e.target_node_id::text,e.flow_name,e.flow_name_source
 FROM applications.workflow_execution_events e
 WHERE e.actor_id=$1::uuid AND e.action IN ('agree','reject','return') AND e.outcome='success'`
		args := []any{s.binding.ActorID}
		if beforeID != "" {
			q += " AND (e.created_at,e.command_id)<($2::timestamptz,$3::uuid)"
			args = append(args, beforeTime, beforeID)
		}
		q += " ORDER BY e.created_at DESC,e.command_id DESC LIMIT 64"
		rows, e := tx.Query(ctx, q, args...)
		if e != nil {
			return nil, e
		}
		batch := make([]approvedCandidate, 0, 64)
		for rows.Next() {
			var c approvedCandidate
			v := &c.row.event
			if e = rows.Scan(&c.item.AppID, &c.tableID, &c.item.RecordID,
				&v.ID, &v.InstanceID, &v.FlowID, &c.row.sourceView, &c.row.definition,
				&v.ActorID, &v.Action, &v.Outcome, &v.Sequence, &v.SchemaVersion, &v.RecordVersion, &c.row.at,
				&c.row.proof, &c.row.version, &c.row.task, &c.row.epoch, &v.NodeID, &v.TargetNodeID, &v.FlowName, &v.FlowNameSource); e != nil {
				rows.Close()
				return nil, ErrUnavailable
			}
			c.created = c.row.at
			v.OccurredAt = c.row.at.UTC().Format(time.RFC3339Nano)
			c.item.ViewID = c.row.sourceView
			c.item.Event = *v
			batch = append(batch, c)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		for _, c := range batch {
			visible, e := s.authorized(ctx, tx, &c)
			if e != nil {
				return nil, e
			}
			if !visible {
				continue
			}
			if s.observation.Total >= workflowMaxCounter {
				return nil, ErrUnavailable
			}
			raw, e := json.Marshal(c.item)
			if e != nil {
				return nil, e
			}
			_, _ = h.Write(raw)
			_, _ = h.Write([]byte{0})
			n := s.observation.Total
			s.observation.Total++
			if n >= offset && int64(len(s.observation.Items)) < int64(s.page.Size) {
				s.observation.Items = append(s.observation.Items, c.item)
			}
		}
		if len(batch) < 64 {
			break
		}
		last := batch[len(batch)-1]
		beforeID, beforeTime = last.item.Event.ID, last.created
	}
	s.observation.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return json.Marshal(inboxRevision{Fingerprint: s.observation.Fingerprint})
}
func (s *approvalsStrategy) authorized(ctx context.Context, tx pgx.Tx, candidate *approvedCandidate) (bool, error) {
	item := &candidate.item
	facts, err := s.read.RecordContext(ctx, item.AppID, item.ViewID)
	if errors.Is(err, applications.ErrDenied) || errors.Is(err, applications.ErrMissing) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None || policy.HistoryScope() == appaccess.None {
		return false, nil
	}
	if facts.Actor.ID != s.binding.ActorID || facts.TableID != candidate.tableID || !facts.SchemaReady {
		return false, ErrUnavailable
	}
	_, ids, err := fieldsInContext(facts)
	if err != nil {
		return false, err
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(candidate.tableID, "-", "")}.Sanitize()
	var creator string
	err = tx.QueryRow(ctx, "SELECT created_by::text FROM "+relation+" WHERE id=$1::uuid", item.RecordID).Scan(&creator)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, ErrUnavailable
	}
	full := facts.Actor.BootstrapAdmin || facts.Actor.ID == facts.App.OwnerUserID
	if (len(policy.ReadFields(creator, ids)) == 0 || len(policy.HistoryFields(creator, ids)) == 0) && !full {
		return false, nil
	}
	if !workflowID(creator) {
		return false, ErrUnavailable
	}
	row := candidate.row
	e := row.event
	entries, err := (fc.Ledger{Namespace: "applications"}).GetManyInTx(ctx, tx, []string{e.ID})
	if err != nil {
		return false, ErrUnavailable
	}
	entry := entries[e.ID]
	c, r := entry.Command, entry.Receipt
	if !validWorkflowJournalMetadata(e, c, row.version, row.task, row.epoch) || c.ProtocolVersion != 2 ||
		c.AppID != item.AppID || c.TableID != candidate.tableID || c.RecordID != item.RecordID || c.ViewID != row.sourceView ||
		c.InstanceID != e.InstanceID || c.FlowID != e.FlowID || c.DefinitionVersion != row.definition ||
		c.ActorID != s.binding.ActorID || e.ActorID != s.binding.ActorID || c.Action != e.Action ||
		c.SchemaVersion != e.SchemaVersion || c.RecordVersion != e.RecordVersion || r == nil || entry.State != e.Outcome ||
		r.Outcome != e.Outcome || r.Sequence != e.Sequence || r.ProofID != row.proof || row.at.IsZero() {
		return false, ErrUnavailable
	}
	return true, nil
}
func (s *approvalsStrategy) Page(_ context.Context, _ pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]WorkflowApprovalsItem, error) {
	if !s.validCriteria(raw) || page != s.page {
		return nil, querycontext.ErrInvalid
	}
	return s.observation.Items, nil
}
func (s *approvalsStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]WorkflowApprovalsItem], error) {
	items, err := s.Page(ctx, tx, raw, page)
	if err != nil {
		return querycontext.Observation[[]WorkflowApprovalsItem]{}, err
	}
	r := s.observation
	r.Items = items
	return r, nil
}
