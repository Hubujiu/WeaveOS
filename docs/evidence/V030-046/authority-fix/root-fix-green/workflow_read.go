package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type WorkflowInstanceSearchRequest struct {
	AppID, ViewID, RecordID, QueryVersion string
	Page, PageSize                        int
}
type WorkflowInstanceSummary struct {
	ID                string `json:"id"`
	FlowID            string `json:"flowId"`
	Name              string `json:"name"`
	InitiatorID       string `json:"initiatorId"`
	State             string `json:"state"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
	DefinitionVersion int64  `json:"definitionVersion"`
	Sequence          int64  `json:"sequence"`
}
type WorkflowInstanceSearchResult struct {
	Items        []WorkflowInstanceSummary `json:"items"`
	Total        int64                     `json:"total"`
	Page         int                       `json:"page"`
	PageSize     int                       `json:"pageSize"`
	QueryVersion string                    `json:"queryVersion"`
}

func (s *Service) SearchRecordWorkflows(ctx context.Context, p session.Principal, req WorkflowInstanceSearchRequest) (WorkflowInstanceSearchResult, error) {
	empty := WorkflowInstanceSearchResult{}
	if s == nil || s.Pool == nil || s.WorkflowReads == nil {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if _, _, err := appquery.PageWindow(int64(req.Page), int64(req.PageSize)); err != nil {
		return empty, err
	}
	binding := workflowReadBinding{AppID: req.AppID, ViewID: req.ViewID, RecordID: req.RecordID, ActorID: p.UserID}
	raw, err := json.Marshal(binding)
	if err != nil {
		return empty, ErrUnavailable
	}
	strategy := &workflowReadStrategy{service: s, principal: p, binding: binding}
	result, err := querycontext.Execute(ctx, s.WorkflowReads, p.SessionRef, req.QueryVersion, raw,
		querycontext.Page{Number: req.Page, Size: req.PageSize}, strategy)
	if err != nil {
		return empty, err
	}
	return WorkflowInstanceSearchResult{Items: result.Items, Total: result.Total, Page: req.Page,
		PageSize: req.PageSize, QueryVersion: result.Version}, nil
}

type workflowReadBinding struct {
	AppID    string `json:"appId"`
	ViewID   string `json:"viewId"`
	RecordID string `json:"recordId"`
	ActorID  string `json:"actorId"`
}

type workflowReadRevision struct {
	SchemaVersion  int64  `json:"schemaVersion"`
	ViewVersion    int64  `json:"viewVersion"`
	PolicyRevision int64  `json:"policyRevision"`
	Fingerprint    string `json:"fingerprint"`
}

func workflowReadFingerprintValid(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size && hex.EncodeToString(raw) == value
}

func workflowReadRevisionValid(raw json.RawMessage) (workflowReadRevision, bool) {
	var r workflowReadRevision
	if !workflowClosedMetadata(raw, 512, 4, &r) {
		return r, false
	}
	return r, workflowPositive(r.SchemaVersion) && workflowPositive(r.PolicyRevision) &&
		r.ViewVersion >= 0 && r.ViewVersion <= workflowMaxCounter && workflowReadFingerprintValid(r.Fingerprint)
}

func workflowReadResourceValid(resource string) ([]string, bool) {
	parts := strings.Split(resource, ":")
	if len(parts) != 5 || parts[0] != "workflow-read" {
		return nil, false
	}
	for _, id := range parts[1:] {
		if !workflowID(id) {
			return nil, false
		}
	}
	return parts, true
}

func newWorkflowReadStore(client redis.UniversalClient, generation string) *querycontext.Store {
	policy := querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			parts, ok := workflowReadResourceValid(m.View)
			if !ok {
				return false
			}
			var binding workflowReadBinding
			if !workflowClosedMetadata(m.Criteria, 1024, 4, &binding) || !workflowID(binding.ActorID) ||
				binding.AppID != parts[1] || binding.ViewID != parts[3] || binding.RecordID != parts[4] {
				return false
			}
			r, ok := workflowReadRevisionValid(m.Revision)
			return ok && r.Fingerprint == m.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			if _, ok := workflowReadResourceValid(resource); !ok {
				return false
			}
			a, aok := workflowReadRevisionValid(old)
			b, bok := workflowReadRevisionValid(next)
			return aok && bok && b.SchemaVersion >= a.SchemaVersion && b.ViewVersion >= a.ViewVersion &&
				b.PolicyRevision >= a.PolicyRevision
		},
	}
	return querycontext.NewStore(client, "workflow-read", generation, policy)
}

type workflowReadStrategy struct {
	service     *Service
	principal   session.Principal
	binding     workflowReadBinding
	facts       applications.RecordContext
	authority   [sha256.Size]byte
	observation querycontext.Observation[[]WorkflowInstanceSummary]
}

func (s *workflowReadStrategy) Resource() string {
	return "workflow-read:" + s.binding.AppID + ":" + s.facts.TableID + ":" + s.binding.ViewID + ":" + s.binding.RecordID
}

func (s *workflowReadStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	tx, facts, err := (&applications.Application{Pool: s.service.Pool}).BeginRecordRead(ctx, s.principal, s.binding.AppID, s.binding.ViewID)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (pgx.Tx, error) {
		_ = tx.Rollback(context.Background())
		return nil, e
	}
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None {
		return fail(applications.ErrDenied)
	}
	if !facts.SchemaReady {
		return fail(apprecords.ErrNotReady)
	}
	if !workflowID(facts.TableID) || facts.Actor.ID != s.binding.ActorID {
		return fail(ErrUnavailable)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
	var creator string
	err = tx.QueryRow(ctx, "SELECT created_by::text FROM "+relation+" WHERE id=$1::uuid", s.binding.RecordID).Scan(&creator)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && policy.VisibleScope() == appaccess.Own && creator != facts.Actor.ID {
		return fail(applications.ErrMissing)
	}
	if err != nil {
		return fail(err)
	}
	if !workflowID(creator) {
		return fail(ErrUnavailable)
	}
	// Live menu and effective row readability were checked above on every call.
	// This summary exposes no business fields or edit capabilities: unrelated
	// grant changes must not alter its projection fingerprint. Bind only the
	// authorized actor/resource identity, not the entire grant representation.
	authority, err := json.Marshal(struct {
		Binding workflowReadBinding
		TableID string
	}{s.binding, facts.TableID})
	if err != nil {
		return fail(ErrUnavailable)
	}
	s.facts, s.authority = facts, sha256.Sum256(authority)
	return tx, nil
}

func (s *workflowReadStrategy) validCriteria(raw json.RawMessage) bool {
	var binding workflowReadBinding
	return workflowClosedMetadata(raw, 1024, 4, &binding) && binding == s.binding
}

func (s *workflowReadStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	var old json.RawMessage
	if len(saved) > 0 {
		if !s.validCriteria(saved) {
			return nil, nil, querycontext.ErrExpired
		}
		old, _ = json.Marshal(s.binding)
	}
	if !s.validCriteria(incoming) {
		return nil, nil, querycontext.ErrInvalid
	}
	current, err := json.Marshal(s.binding)
	return old, current, err
}

const workflowReadSelect = `SELECT jsonb_build_object(
 'id',i.id,'flowId',i.flow_id,'name',d.name,'initiatorId',i.initiator_id,
 'state',i.state,'createdAt',i.created_at,'updatedAt',i.updated_at,
 'definitionVersion',i.definition_version,'sequence',i.sequence)
 FROM applications.workflow_instances i
 JOIN applications.workflow_definitions d ON d.app_id=i.app_id AND d.id=i.flow_id
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.record_id=$3`

const workflowReadOrder = " ORDER BY i.created_at DESC,i.id DESC"

func (s *workflowReadStrategy) args() []any {
	return []any{s.binding.AppID, s.facts.TableID, s.binding.RecordID}
}

func (s *workflowReadStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	// Instances have no dedicated revision counter. Observe this one record's
	// ordered summaries once per RR read, retaining only their hash and count.
	// This costs O(M) streamed time and constant summary observation space.
	rows, err := tx.Query(ctx, workflowReadSelect+workflowReadOrder, s.args()...)
	if err != nil {
		return nil, err
	}
	projection, err := appquery.FingerprintRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("weaveos/workflow-read/projection/v1\x00"))
	_, _ = hash.Write(s.authority[:])
	_, _ = hash.Write([]byte(projection.Fingerprint))
	fingerprint := hex.EncodeToString(hash.Sum(nil))
	s.observation = querycontext.Observation[[]WorkflowInstanceSummary]{Total: projection.Total, Fingerprint: fingerprint}
	return json.Marshal(workflowReadRevision{SchemaVersion: s.facts.SchemaVersion, ViewVersion: s.facts.ViewVersion,
		PolicyRevision: s.facts.App.PolicyRevision, Fingerprint: fingerprint})
}

func (s *workflowReadStrategy) Page(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]WorkflowInstanceSummary, error) {
	if !s.validCriteria(raw) {
		return nil, querycontext.ErrInvalid
	}
	limit, offset, err := appquery.PageWindow(int64(page.Number), int64(page.Size))
	if err != nil {
		return nil, err
	}
	args := append(s.args(), limit, offset)
	rows, err := tx.Query(ctx, workflowReadSelect+workflowReadOrder+" LIMIT $4 OFFSET $5", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WorkflowInstanceSummary, 0, page.Size)
	for rows.Next() {
		var encoded []byte
		var item WorkflowInstanceSummary
		if err = rows.Scan(&encoded); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(encoded, &item); err != nil {
			return nil, fmt.Errorf("workflow summary projection: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *workflowReadStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]WorkflowInstanceSummary], error) {
	items, err := s.Page(ctx, tx, raw, page)
	if err != nil {
		return querycontext.Observation[[]WorkflowInstanceSummary]{}, err
	}
	observed := s.observation
	observed.Items = items
	return observed, nil
}
