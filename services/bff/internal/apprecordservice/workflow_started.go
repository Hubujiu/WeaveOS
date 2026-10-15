package apprecordservice

import (
	"bytes"
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

// Permanent round identities survive cleanup of terminal runtime projections.
type WorkflowStartedRequest struct {
	Page, PageSize int
	QueryVersion   string
}
type WorkflowStartedItem struct {
	Archived       bool                    `json:"archived,omitempty"`
	FlowNameSource string                  `json:"flowNameSource,omitempty"`
	AppID          string                  `json:"appId"`
	ViewID         string                  `json:"viewId"`
	RecordID       string                  `json:"recordId"`
	Instance       WorkflowInstanceSummary `json:"instance"`
}
type WorkflowStartedResult struct {
	Items        []WorkflowStartedItem `json:"items"`
	Total        int64                 `json:"total"`
	Page         int                   `json:"page"`
	PageSize     int                   `json:"pageSize"`
	QueryVersion string                `json:"queryVersion"`
}

func (s *Service) SearchWorkflowStarted(ctx context.Context, p session.Principal, q WorkflowStartedRequest) (WorkflowStartedResult, error) {
	empty := WorkflowStartedResult{}
	if s == nil || s.Pool == nil || s.WorkflowStarted == nil {
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
	strategy := &startedStrategy{service: s, principal: p, binding: binding, page: querycontext.Page{Number: q.Page, Size: q.PageSize}}
	r, err := querycontext.Execute(ctx, s.WorkflowStarted, p.SessionRef, q.QueryVersion, raw, strategy.page, strategy)
	if err != nil {
		return empty, err
	}
	return WorkflowStartedResult{Items: r.Items, Total: r.Total, Page: q.Page, PageSize: q.PageSize, QueryVersion: r.Version}, nil
}

func newWorkflowStartedStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-started", generation, querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			var b inboxBinding
			r, ok := validInboxRevision(m.Revision)
			return ok && workflowClosedMetadata(m.Criteria, 128, 1, &b) && workflowID(b.ActorID) && m.View == "workflow-started:"+b.ActorID && m.Fingerprint == r.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			_, a := validInboxRevision(old)
			_, b := validInboxRevision(next)
			return a && b && strings.HasPrefix(resource, "workflow-started:") && workflowID(strings.TrimPrefix(resource, "workflow-started:"))
		},
	})
}

type startedStrategy struct {
	service     *Service
	principal   session.Principal
	binding     inboxBinding
	read        *applications.PersonalRead
	page        querycontext.Page
	observation querycontext.Observation[[]WorkflowStartedItem]
}

func (s *startedStrategy) Resource() string { return "workflow-started:" + s.binding.ActorID }
func (s *startedStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	r, e := (&applications.Application{Pool: s.service.Pool}).BeginPersonalRead(ctx, s.principal)
	if e != nil {
		return nil, e
	}
	s.read = r
	return r.Tx, nil
}
func (s *startedStrategy) validCriteria(raw json.RawMessage) bool {
	var b inboxBinding
	return workflowClosedMetadata(raw, 128, 1, &b) && b == s.binding
}
func (s *startedStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	if len(saved) > 0 && !s.validCriteria(saved) {
		return nil, nil, querycontext.ErrExpired
	}
	if !s.validCriteria(incoming) {
		return nil, nil, querycontext.ErrInvalid
	}
	return saved, incoming, nil
}

type startedCandidate struct {
	item    WorkflowStartedItem
	tableID string
	created time.Time
}

func (s *startedStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	_, offset, err := appquery.PageWindow(int64(s.page.Number), int64(s.page.Size))
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/personal-started/v1\x00"))
	_, _ = h.Write([]byte(s.binding.ActorID))
	s.observation = querycontext.Observation[[]WorkflowStartedItem]{Items: make([]WorkflowStartedItem, 0, s.page.Size)}
	var beforeID string
	var beforeTime time.Time
	for {
		q := `SELECT r.app_id::text,r.view_id::text,r.table_id::text,r.record_id::text,
 r.instance_id::text,r.flow_id::text,COALESCE(d.name,''),r.initiator_id::text,COALESCE(i.state,''),
 r.created_at,COALESCE(i.updated_at,r.created_at),r.definition_version,COALESCE(i.sequence,0),i.id IS NULL
 FROM applications.workflow_rounds r
 LEFT JOIN applications.workflow_instances i ON i.app_id=r.app_id AND i.id=r.instance_id
 LEFT JOIN applications.workflow_definitions d ON d.app_id=i.app_id AND d.id=i.flow_id
 WHERE r.initiator_id=$1::uuid`
		args := []any{s.binding.ActorID}
		if beforeID != "" {
			q += " AND (r.created_at,r.instance_id)<($2::timestamptz,$3::uuid)"
			args = append(args, beforeTime, beforeID)
		}
		q += " ORDER BY r.created_at DESC,r.instance_id DESC LIMIT 64"
		rows, e := tx.Query(ctx, q, args...)
		if e != nil {
			return nil, e
		}
		batch := make([]startedCandidate, 0, 64)
		for rows.Next() {
			var c startedCandidate
			v := &c.item.Instance
			var updated time.Time
			if e = rows.Scan(&c.item.AppID, &c.item.ViewID, &c.tableID, &c.item.RecordID, &v.ID, &v.FlowID, &v.Name, &v.InitiatorID, &v.State, &c.created, &updated, &v.DefinitionVersion, &v.Sequence, &c.item.Archived); e != nil {
				rows.Close()
				return nil, ErrUnavailable
			}
			v.CreatedAt = c.created.UTC().Format(time.RFC3339Nano)
			v.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
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
		beforeID, beforeTime = last.item.Instance.ID, last.created
	}
	s.observation.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return json.Marshal(inboxRevision{Fingerprint: s.observation.Fingerprint})
}
func (s *startedStrategy) authorized(ctx context.Context, tx pgx.Tx, c *startedCandidate) (bool, error) {
	v := &c.item
	facts, err := s.read.RecordContext(ctx, v.AppID, v.ViewID)
	if errors.Is(err, applications.ErrDenied) || errors.Is(err, applications.ErrMissing) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None || v.Archived && policy.HistoryScope() == appaccess.None {
		return false, nil
	}
	if facts.Actor.ID != s.binding.ActorID || facts.TableID != c.tableID || !facts.SchemaReady {
		return false, ErrUnavailable
	}
	_, ids, err := fieldsInContext(facts)
	if err != nil {
		return false, err
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(c.tableID, "-", "")}.Sanitize()
	var creator string
	err = tx.QueryRow(ctx, "SELECT created_by::text FROM "+relation+" WHERE id=$1::uuid", v.RecordID).Scan(&creator)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	full := facts.Actor.BootstrapAdmin || facts.Actor.ID == facts.App.OwnerUserID
	if (len(policy.ReadFields(creator, ids)) == 0 || v.Archived && len(policy.HistoryFields(creator, ids)) == 0) && !full {
		return false, nil
	}
	if v.Archived {
		if err = readStartedArchive(ctx, tx, v, c.tableID); err != nil {
			return false, err
		}
	}
	instance := v.Instance
	if !workflowID(creator) || !workflowID(instance.ID) || !workflowID(instance.FlowID) || instance.InitiatorID != s.binding.ActorID ||
		!workflowPositive(instance.DefinitionVersion) || instance.Sequence < 0 || instance.Sequence > workflowMaxCounter || c.created.IsZero() {
		return false, ErrUnavailable
	}
	switch instance.State {
	case "starting", "active":
		if v.Archived {
			return false, ErrUnavailable
		}
	case "completed", "rejected", "withdrawn", "no_effect":
	default:
		return false, ErrUnavailable
	}
	return true, nil
}
func (s *startedStrategy) Page(_ context.Context, _ pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]WorkflowStartedItem, error) {
	if !s.validCriteria(raw) || page != s.page {
		return nil, querycontext.ErrInvalid
	}
	return s.observation.Items, nil
}
func (s *startedStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]WorkflowStartedItem], error) {
	items, err := s.Page(ctx, tx, raw, page)
	if err != nil {
		return querycontext.Observation[[]WorkflowStartedItem]{}, err
	}
	r := s.observation
	r.Items = items
	return r, nil
}

// Read only a proven effect, never reinterpret a later no-effect as a state.
// All fields remain in the caller's authorized RR; no current catalog or graph
// and no evidence field blobs are necessary to describe a retained round.
func readStartedArchive(ctx context.Context, tx pgx.Tx, item *WorkflowStartedItem, tableID string) error {
	var e WorkflowEventSummary
	var version, proof, sourceView string
	var definition, epoch int64
	var task *string
	var at time.Time
	var evidence, payloadRaw, resultRaw []byte
	err := tx.QueryRow(ctx, `SELECT command_id::text,instance_id::text,flow_id::text,view_id::text,
 definition_version,actor_id::text,action,outcome,sequence,schema_version,record_version,
 created_at,evidence_hash,payload_bytes,result_bytes,proof_id::text,version_id::text,
 task_id::text,task_epoch,node_id::text,target_node_id::text,flow_name,flow_name_source
 FROM applications.workflow_execution_events
 WHERE app_id=$1::uuid AND table_id=$2::uuid AND record_id=$3::uuid AND instance_id=$4::uuid
 AND (outcome='success' OR (action='start' AND outcome='no_effect'))
 ORDER BY (outcome='success') DESC,sequence DESC,created_at DESC,command_id DESC LIMIT 1`,
		item.AppID, tableID, item.RecordID, item.Instance.ID).Scan(
		&e.ID, &e.InstanceID, &e.FlowID, &sourceView, &definition, &e.ActorID, &e.Action, &e.Outcome,
		&e.Sequence, &e.SchemaVersion, &e.RecordVersion, &at, &evidence, &payloadRaw, &resultRaw,
		&proof, &version, &task, &epoch, &e.NodeID, &e.TargetNodeID, &e.FlowName, &e.FlowNameSource)
	if err != nil {
		return ErrUnavailable
	}
	entry, err := (fc.Ledger{Namespace: "applications"}).GetInTx(ctx, tx, e.ID)
	if err != nil {
		return ErrUnavailable
	}
	c, r := entry.Command, entry.Receipt
	if !validWorkflowJournalMetadata(e, c, version, task, epoch) || c.ProtocolVersion != 2 ||
		c.AppID != item.AppID || c.TableID != tableID || c.ViewID != item.ViewID || sourceView != item.ViewID ||
		c.RecordID != item.RecordID || c.InstanceID != item.Instance.ID || e.InstanceID != item.Instance.ID ||
		c.FlowID != item.Instance.FlowID || e.FlowID != item.Instance.FlowID ||
		c.DefinitionVersion != item.Instance.DefinitionVersion || definition != item.Instance.DefinitionVersion ||
		c.ActorID != e.ActorID || c.Action != e.Action || c.SchemaVersion != e.SchemaVersion || c.RecordVersion != e.RecordVersion ||
		r == nil || entry.State != e.Outcome || r.Outcome != e.Outcome || r.Sequence != e.Sequence || r.ProofID != proof ||
		at.IsZero() || len(evidence) != 32 || sha256.Sum256(payloadRaw) != c.PayloadHash {
		return ErrUnavailable
	}
	// Only a start is performed by the initiator; later approval actors may differ.
	if c.Action == "start" && c.ActorID != item.Instance.InitiatorID {
		return ErrUnavailable
	}
	payload, err := fc.DecodeExecutionPayload(c.Action, payloadRaw)
	if err != nil || !bytes.Equal(evidence, payload.EvidenceHash[:]) {
		return ErrUnavailable
	}
	result, err := fc.DecodeExecutionResult(c, *r, resultRaw)
	if err != nil {
		return ErrUnavailable
	}
	state := result.State
	if e.Outcome == "no_effect" {
		if c.Action != "start" || state != "unchanged" || e.Sequence != 0 {
			return ErrUnavailable
		}
		state = "no_effect"
	} else {
		switch state {
		case "completed", "rejected", "withdrawn":
		default:
			return ErrUnavailable
		}
	}
	item.Instance.Name = e.FlowName
	item.FlowNameSource = e.FlowNameSource
	item.Instance.State = state
	item.Instance.Sequence = e.Sequence
	item.Instance.UpdatedAt = at.UTC().Format(time.RFC3339Nano)
	return nil
}
