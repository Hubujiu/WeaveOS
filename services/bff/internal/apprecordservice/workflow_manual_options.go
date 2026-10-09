package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

type WorkflowManualOptionsRequest struct {
	AppID, ViewID, RecordID, QueryVersion string
	Page, PageSize                        int
}
type WorkflowManualOption struct {
	FlowID            string `json:"flowId"`
	Name              string `json:"name"`
	WorkflowRevision  int64  `json:"workflowRevision"`
	DefinitionVersion int64  `json:"definitionVersion"`
	SchemaVersion     int64  `json:"schemaVersion"`
	RecordVersion     int64  `json:"recordVersion"`
}
type WorkflowManualOptionsResult struct {
	Items        []WorkflowManualOption `json:"items"`
	Total        int64                  `json:"total"`
	Page         int                    `json:"page"`
	PageSize     int                    `json:"pageSize"`
	QueryVersion string                 `json:"queryVersion"`
}

// SearchManualWorkflowOptions is an authorized read, not a reservation. The
// subsequent manual POST rechecks all live facts under its write locks.
func (s *Service) SearchManualWorkflowOptions(ctx context.Context, p session.Principal, req WorkflowManualOptionsRequest) (WorkflowManualOptionsResult, error) {
	empty := WorkflowManualOptionsResult{}
	if s == nil || s.Pool == nil || s.WorkflowManualOptions == nil {
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
	// Bound O(F) configuration/condition reads even for callers without a deadline.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	binding := workflowReadBinding{AppID: req.AppID, ViewID: req.ViewID, RecordID: req.RecordID, ActorID: p.UserID}
	raw, err := json.Marshal(binding)
	if err != nil {
		return empty, ErrUnavailable
	}
	page := querycontext.Page{Number: req.Page, Size: req.PageSize}
	strategy := &manualOptionsStrategy{workflowReadStrategy: workflowReadStrategy{service: s, principal: p, binding: binding}, requested: page}
	result, err := querycontext.Execute(ctx, s.WorkflowManualOptions, p.SessionRef, req.QueryVersion, raw, page, strategy)
	if err != nil {
		return empty, err
	}
	return WorkflowManualOptionsResult{Items: result.Items, Total: result.Total, Page: req.Page, PageSize: req.PageSize, QueryVersion: result.Version}, nil
}

func manualOptionsResourceValid(resource string) ([]string, bool) {
	if !strings.HasPrefix(resource, "workflow-manual-options:") {
		return nil, false
	}
	return workflowReadResourceValid("workflow-read:" + strings.TrimPrefix(resource, "workflow-manual-options:"))
}
func newManualOptionsStore(client redis.UniversalClient, generation string) *querycontext.Store {
	policy := querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			parts, ok := manualOptionsResourceValid(m.View)
			if !ok {
				return false
			}
			var binding workflowReadBinding
			if !workflowClosedMetadata(m.Criteria, 1024, 4, &binding) || !workflowID(binding.ActorID) || binding.AppID != parts[1] || binding.ViewID != parts[3] || binding.RecordID != parts[4] {
				return false
			}
			r, ok := workflowReadRevisionValid(m.Revision)
			return ok && r.Fingerprint == m.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			if _, ok := manualOptionsResourceValid(resource); !ok {
				return false
			}
			a, aok := workflowReadRevisionValid(old)
			b, bok := workflowReadRevisionValid(next)
			return aok && bok && b.SchemaVersion >= a.SchemaVersion && b.ViewVersion >= a.ViewVersion && b.PolicyRevision >= a.PolicyRevision
		},
	}
	return querycontext.NewStore(client, "workflow-manual-options", generation, policy)
}

type manualOptionsStrategy struct {
	workflowReadStrategy
	requested     querycontext.Page
	creator       string
	recordVersion int64
	fields        []appquery.Field
	options       querycontext.Observation[[]WorkflowManualOption]
}

func (s *manualOptionsStrategy) Resource() string {
	return "workflow-manual-options:" + s.binding.AppID + ":" + s.facts.TableID + ":" + s.binding.ViewID + ":" + s.binding.RecordID
}
func (s *manualOptionsStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.workflowReadStrategy.OpenRead(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (pgx.Tx, error) { _ = tx.Rollback(context.Background()); return nil, e }
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.facts.TableID, "-", "")}.Sanitize()
	if err = tx.QueryRow(ctx, "SELECT created_by::text,record_version FROM "+relation+" WHERE id=$1::uuid", s.binding.RecordID).Scan(&s.creator, &s.recordVersion); err != nil {
		return fail(err)
	}
	defs, ids, err := fieldsInContext(s.facts)
	if err != nil {
		return fail(err)
	}
	policy, _ := policyFor(s.facts)
	if len(policy.ReadFields(s.creator, ids)) == 0 {
		return fail(applications.ErrDenied)
	}
	if !workflowPositive(s.recordVersion) {
		return fail(ErrUnavailable)
	}
	for _, f := range defs {
		s.fields = append(s.fields, appquery.Field{ID: f.ID, Kind: appquery.FieldKind(f.Kind)})
	}
	return tx, nil
}
func (s *manualOptionsStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	limit, offset, err := appquery.PageWindow(int64(s.requested.Number), int64(s.requested.Size))
	if err != nil {
		return nil, err
	}
	observed := querycontext.Observation[[]WorkflowManualOption]{Items: make([]WorkflowManualOption, 0, s.requested.Size)}
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/workflow-manual-options/projection/v1\x00"))
	_, _ = h.Write(s.authority[:])
	// No result collection is retained: one candidate and the requested page only.
	cursor := "00000000-0000-0000-0000-000000000000"
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.facts.TableID, "-", "")}.Sanitize()
	for s.creator == s.facts.Actor.ID {
		var item WorkflowManualOption
		err = tx.QueryRow(ctx, `SELECT d.id::text,d.name,d.revision,d.current_version
   FROM applications.workflow_definitions d
   JOIN applications.workflow_versions v ON v.app_id=d.app_id AND v.flow_id=d.id AND v.version=d.current_version
   JOIN applications.workflow_deployments p ON p.app_id=d.app_id AND p.flow_id=d.id AND p.version=d.current_version
   WHERE d.app_id=$1 AND d.table_id=$2 AND d.state='enabled' AND d.id>$3::uuid
    AND v.triggers_json @> '[{"event":"manual"}]'::jsonb
    AND NOT EXISTS(SELECT 1 FROM applications.workflow_instances i WHERE i.app_id=d.app_id AND i.table_id=d.table_id AND i.flow_id=d.id AND i.record_id=$4::uuid)
   ORDER BY d.id LIMIT 1`, s.binding.AppID, s.facts.TableID, cursor, s.binding.RecordID).Scan(&item.FlowID, &item.Name, &item.WorkflowRevision, &item.DefinitionVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return nil, err
		}
		cursor = item.FlowID
		triggers, e := (workflowcatalog.Catalog{}).TriggersForVersionInTx(ctx, tx, s.binding.AppID, item.FlowID, item.DefinitionVersion)
		if e != nil {
			return nil, e
		}
		triggers, e = workflowcatalog.NormalizeTriggers(triggers, s.fields)
		if e != nil {
			return nil, workflowcatalog.ErrNotReady
		}
		matched := false
		for _, trigger := range triggers {
			if trigger.Event != "manual" {
				continue
			}
			plan, e := appquery.Compile(trigger.Condition, nil, s.fields, 2)
			if e != nil {
				return nil, workflowcatalog.ErrNotReady
			}
			args := append([]any{s.binding.RecordID}, plan.Arguments...)
			if e = tx.QueryRow(ctx, "SELECT COALESCE(("+plan.Predicate+"),false) FROM "+relation+" r WHERE r.id=$1::uuid", args...).Scan(&matched); e != nil {
				return nil, e
			}
			break
		}
		if !matched {
			continue
		}
		item.SchemaVersion = s.facts.SchemaVersion
		item.RecordVersion = s.recordVersion
		encoded, e := json.Marshal(item)
		if e != nil {
			return nil, ErrUnavailable
		}
		_, _ = h.Write(encoded)
		_, _ = h.Write([]byte{0})
		if observed.Total >= offset && observed.Total-offset < limit {
			observed.Items = append(observed.Items, item)
		}
		observed.Total++
	}
	observed.Fingerprint = hex.EncodeToString(h.Sum(nil))
	s.options = observed
	return json.Marshal(workflowReadRevision{SchemaVersion: s.facts.SchemaVersion, ViewVersion: s.facts.ViewVersion, PolicyRevision: s.facts.App.PolicyRevision, Fingerprint: observed.Fingerprint})
}
func (s *manualOptionsStrategy) Page(_ context.Context, _ pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]WorkflowManualOption, error) {
	// Prepare rejects any criteria change, so Execute always validates the same
	// requested page. Explicitly reject a different page rather than return a cache.
	if !s.validCriteria(raw) || page != s.requested {
		return nil, querycontext.ErrInvalid
	}
	return s.options.Items, nil
}
func (s *manualOptionsStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]WorkflowManualOption], error) {
	if _, err := s.Page(ctx, tx, raw, page); err != nil {
		return querycontext.Observation[[]WorkflowManualOption]{}, err
	}
	return s.options, nil
}
