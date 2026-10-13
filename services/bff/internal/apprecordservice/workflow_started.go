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
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

// Current-runtime slice. Retained rounds after catalog deletion remain separate.
type WorkflowStartedRequest struct {
	Page, PageSize int
	QueryVersion   string
}
type WorkflowStartedItem struct {
	AppID    string                  `json:"appId"`
	ViewID   string                  `json:"viewId"`
	RecordID string                  `json:"recordId"`
	Instance WorkflowInstanceSummary `json:"instance"`
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
		q := `SELECT i.app_id::text,i.view_id::text,i.table_id::text,i.record_id::text,
 i.id::text,i.flow_id::text,d.name,i.initiator_id::text,i.state,i.created_at,i.updated_at,i.definition_version,i.sequence
 FROM applications.workflow_instances i
 JOIN applications.workflow_definitions d ON d.app_id=i.app_id AND d.id=i.flow_id
 WHERE i.initiator_id=$1::uuid`
		args := []any{s.binding.ActorID}
		if beforeID != "" {
			q += " AND (i.created_at,i.id)<($2::timestamptz,$3::uuid)"
			args = append(args, beforeTime, beforeID)
		}
		q += " ORDER BY i.created_at DESC,i.id DESC LIMIT 64"
		rows, e := tx.Query(ctx, q, args...)
		if e != nil {
			return nil, e
		}
		batch := make([]startedCandidate, 0, 64)
		for rows.Next() {
			var c startedCandidate
			v := &c.item.Instance
			var updated time.Time
			if e = rows.Scan(&c.item.AppID, &c.item.ViewID, &c.tableID, &c.item.RecordID, &v.ID, &v.FlowID, &v.Name, &v.InitiatorID, &v.State, &c.created, &updated, &v.DefinitionVersion, &v.Sequence); e != nil {
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
	if !menu || policy.VisibleScope() == appaccess.None {
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
	if len(policy.ReadFields(creator, ids)) == 0 && !full {
		return false, nil
	}
	instance := v.Instance
	if !workflowID(creator) || !workflowID(instance.ID) || !workflowID(instance.FlowID) || instance.InitiatorID != s.binding.ActorID ||
		!workflowPositive(instance.DefinitionVersion) || instance.Sequence < 0 || instance.Sequence > workflowMaxCounter || c.created.IsZero() {
		return false, ErrUnavailable
	}
	switch instance.State {
	case "starting", "active", "completed", "rejected", "withdrawn", "no_effect":
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
