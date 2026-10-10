package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"time"
)

type WorkflowRoundRequest struct{ AppID, ViewID, RecordID, InstanceID string }
type WorkflowRoundPreview struct {
	InstanceID        string   `json:"instanceId"`
	FlowID            string   `json:"flowId"`
	RoundNumber       int64    `json:"roundNumber"`
	LatestInstanceID  string   `json:"latestInstanceId"`
	State             string   `json:"state"`
	DefinitionVersion int64    `json:"definitionVersion"`
	WorkflowRevision  int64    `json:"workflowRevision"`
	SchemaVersion     int64    `json:"schemaVersion"`
	RecordVersion     int64    `json:"recordVersion"`
	CanRework         bool     `json:"canRework"`
	CanResubmit       bool     `json:"canResubmit"`
	CanReview         bool     `json:"canReview"`
	EditableFieldIDs  []string `json:"editableFieldIds"`
}
type WorkflowRoundStartRequest struct {
	WorkflowRoundRequest
	OperationID, Kind                                                      string
	ExpectedWorkflowRevision, ExpectedSchemaVersion, ExpectedRecordVersion int64
}
type WorkflowRoundStartResult struct {
	OperationID        string `json:"operationId"`
	FlowID             string `json:"flowId"`
	PreviousInstanceID string `json:"previousInstanceId"`
	InstanceID         string `json:"instanceId"`
	RoundKind          string `json:"roundKind"`
	RoundNumber        int64  `json:"roundNumber"`
	Status             string `json:"status"`
}
type WorkflowRoundReworkRequest struct {
	WorkflowRoundRequest
	OperationID                                  string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Changes                                      map[string]any
}

func roundRequestValid(q WorkflowRoundRequest) bool {
	for _, id := range []string{q.AppID, q.ViewID, q.RecordID, q.InstanceID} {
		if !workflowID(id) {
			return false
		}
	}
	return true
}

type loadedWorkflowRound struct {
	Preview   WorkflowRoundPreview
	Policy    appaccess.Policy
	Table     apprecords.Table
	Header    apprecords.StoredHeader
	Deleted   bool
	FlowState string
}

// Caller first authorizes the actual row. The write caller also owns the
// application/table/flow/record/instance locks; the preview caller owns one RR.
func loadWorkflowRound(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, q WorkflowRoundRequest) (loadedWorkflowRound, error) {
	out := loadedWorkflowRound{}
	_, ids, err := fieldsInContext(facts)
	if err != nil {
		return out, err
	}
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None {
		return out, applications.ErrDenied
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
	var creator string
	var recordVersion int64
	err = tx.QueryRow(ctx, "SELECT created_by::text,record_version FROM "+relation+" WHERE id=$1", q.RecordID).Scan(&creator, &recordVersion)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && len(policy.ReadFields(creator, ids)) == 0 {
		return out, applications.ErrMissing
	}
	if err != nil {
		return out, err
	}
	v := WorkflowRoundPreview{InstanceID: q.InstanceID, SchemaVersion: facts.SchemaVersion, RecordVersion: recordVersion, EditableFieldIDs: []string{}}
	var starter string
	var graphRaw []byte
	err = tx.QueryRow(ctx, `SELECT i.flow_id::text,i.initiator_id::text,i.state,i.definition_version,r.round_number,
  latest.instance_id::text,d.revision,d.state,v.graph_json,
  EXISTS(SELECT 1 FROM applications.workflow_deletions x WHERE x.flow_id=i.flow_id)
 FROM applications.workflow_instances i
 JOIN applications.workflow_rounds r ON r.instance_id=i.id AND r.app_id=i.app_id
 JOIN applications.workflow_definitions d ON d.app_id=i.app_id AND d.id=i.flow_id
 JOIN applications.workflow_versions v ON v.app_id=i.app_id AND v.flow_id=i.flow_id AND v.version=i.definition_version
 CROSS JOIN LATERAL(SELECT instance_id FROM applications.workflow_rounds WHERE app_id=i.app_id AND flow_id=i.flow_id AND record_id=i.record_id ORDER BY round_number DESC LIMIT 1) latest
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.record_id=$3 AND i.id=$4`, q.AppID, facts.TableID, q.RecordID, q.InstanceID).Scan(&v.FlowID, &starter, &v.State, &v.DefinitionVersion, &v.RoundNumber, &v.LatestInstanceID, &v.WorkflowRevision, &out.FlowState, &graphRaw, &out.Deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, applications.ErrMissing
	}
	if err != nil {
		return out, err
	}
	var graph flowgraph.Graph
	if json.Unmarshal(graphRaw, &graph) != nil {
		return out, ErrUnavailable
	}
	approvers := []string{}
	for _, node := range graph.Nodes {
		if node.Kind == "approval" && node.Approval != nil {
			approvers = append(approvers, node.Approval.AssigneeIDs...)
		}
	}
	editable := []string{}
	for _, id := range ids {
		if policy.CanEdit(creator, []string{id}) {
			editable = append(editable, id)
		}
	}
	sort.Strings(editable)
	target := workflowRoundIdentity{q.AppID, facts.TableID, q.RecordID, v.FlowID, q.InstanceID}
	latest := target
	latest.InstanceID = v.LatestInstanceID
	caps := workflowRoundPolicy(workflowRoundFacts{Target: target, Latest: latest, ActorID: facts.Actor.ID, InitiatorID: starter, State: v.State, CanRead: true, CanEdit: len(editable) > 0, DefinitionApprovers: approvers})
	if out.Deleted {
		caps = workflowRoundCapabilities{}
	}
	if out.FlowState != "enabled" {
		caps.Resubmit = false
		caps.Review = false
	}
	v.CanRework, v.CanResubmit, v.CanReview = caps.Rework, caps.Resubmit, caps.Review
	if caps.Rework {
		v.EditableFieldIDs = editable
	}
	out.Preview, out.Policy = v, policy
	out.Table = apprecords.Table{AppID: q.AppID, TableID: facts.TableID, ViewID: q.ViewID, Namespace: "appdata", SchemaVersion: facts.SchemaVersion, Ready: facts.SchemaReady, ActiveFieldIDs: ids}
	out.Header = apprecords.StoredHeader{ID: q.RecordID, CreatedBy: creator, RecordVersion: recordVersion}
	return out, nil
}
func (s *Service) PreviewWorkflowRound(ctx context.Context, p session.Principal, q WorkflowRoundRequest) (WorkflowRoundPreview, error) {
	empty := WorkflowRoundPreview{}
	if s == nil || s.Pool == nil {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || !roundRequestValid(q) {
		return empty, applications.ErrInvalid
	}
	strategy := workflowReadStrategy{service: s, principal: p, binding: workflowReadBinding{AppID: q.AppID, ViewID: q.ViewID, RecordID: q.RecordID, ActorID: p.UserID}}
	tx, err := strategy.OpenRead(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(context.Background())
	loaded, err := loadWorkflowRound(ctx, tx, strategy.facts, q)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return loaded.Preview, nil
}

func (s *Service) beginWorkflowRoundWrite(ctx context.Context, p session.Principal, q WorkflowRoundRequest, op, kind string, body any) (*applications.RecordWrite, *applications.Result, loadedWorkflowRound, error) {
	empty := loadedWorkflowRound{}
	raw, err := json.Marshal(struct {
		Kind string
		Body any
	}{kind, body})
	if err != nil {
		return nil, nil, empty, applications.ErrInvalid
	}
	fingerprint := sha256.Sum256(raw)
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: op, Kind: kind, Fingerprint: fingerprint, Authorize: func(c context.Context, tx pgx.Tx, facts applications.RecordContext) error {
		policy, menu := policyFor(facts)
		if !menu || policy.VisibleScope() == appaccess.None {
			return applications.ErrDenied
		}
		if !facts.SchemaReady {
			return apprecords.ErrNotReady
		}
		_, ids, e := fieldsInContext(facts)
		if e != nil {
			return e
		}
		relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
		var owner string
		e = tx.QueryRow(c, "SELECT created_by::text FROM "+relation+" WHERE id=$1", q.RecordID).Scan(&owner)
		if errors.Is(e, pgx.ErrNoRows) || e == nil && len(policy.ReadFields(owner, ids)) == 0 {
			return applications.ErrMissing
		}
		return e
	}}
	w, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, p, q.AppID, q.ViewID, options)
	if err != nil {
		return nil, nil, empty, err
	}
	fail := func(e error) (*applications.RecordWrite, *applications.Result, loadedWorkflowRound, error) {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Rollback(cleanup)
		return nil, nil, empty, e
	}
	old, err := w.Replay(ctx, op, kind, fingerprint)
	if err != nil {
		return fail(err)
	}
	if old != nil {
		return w, old, empty, nil
	}
	if err = w.Claim(ctx, op, kind, fingerprint); err != nil {
		return fail(err)
	}
	facts, tx := w.Context(), w.Tx()
	gate := appstructure.RecordGate{AppID: q.AppID, TableID: facts.TableID, ViewID: q.ViewID}
	if err = gate.LockTable(ctx, tx, facts.TableID, facts.SchemaVersion); err != nil {
		return fail(err)
	}
	var flowID string
	err = tx.QueryRow(ctx, `SELECT d.id::text FROM applications.workflow_definitions d JOIN applications.workflow_instances i ON i.app_id=d.app_id AND i.flow_id=d.id WHERE i.app_id=$1 AND i.table_id=$2 AND i.record_id=$3 AND i.id=$4 FOR UPDATE OF d`, q.AppID, facts.TableID, q.RecordID, q.InstanceID).Scan(&flowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(applications.ErrMissing)
	}
	if err != nil {
		return fail(err)
	}
	_, ids, err := fieldsInContext(facts)
	if err != nil {
		return fail(err)
	}
	table := apprecords.Table{AppID: q.AppID, TableID: facts.TableID, ViewID: q.ViewID, Namespace: "appdata", SchemaVersion: facts.SchemaVersion, Ready: facts.SchemaReady, ActiveFieldIDs: ids}
	if _, err = (controlledDML{}).LockHeader(ctx, tx, table, q.RecordID); err != nil {
		return fail(err)
	}
	var instance string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM applications.workflow_instances WHERE id=$1 FOR UPDATE", q.InstanceID).Scan(&instance); err != nil {
		return fail(err)
	}
	loaded, err := loadWorkflowRound(ctx, tx, facts, q)
	if err != nil {
		return fail(err)
	}
	if loaded.Preview.LatestInstanceID != q.InstanceID {
		return fail(workflowcatalog.ErrConflict)
	}
	if loaded.Deleted {
		return fail(workflowcatalog.ErrClosing)
	}
	if err = (appstructure.RecordFence{AppID: q.AppID, TableID: facts.TableID}).Check(ctx, tx, facts.TableID, q.RecordID, loaded.Header.RecordVersion); err != nil {
		return fail(err)
	}
	return w, nil, loaded, nil
}
func finishWorkflowRoundWrite(ctx context.Context, w *applications.RecordWrite, op string, status int, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return ErrUnavailable
	}
	location := ""
	if status == 202 {
		location = "/api/v1/application-operations/" + op
	}
	if err = w.Complete(ctx, op, applications.Result{Status: status, Location: location, Data: raw}); err != nil {
		return err
	}
	return w.Commit(ctx)
}
func (s *Service) StartWorkflowRound(ctx context.Context, p session.Principal, req WorkflowRoundStartRequest, metadata applications.Metadata) (WorkflowRoundStartResult, error) {
	empty := WorkflowRoundStartResult{}
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || !roundRequestValid(req.WorkflowRoundRequest) || !workflowID(req.OperationID) || strings.TrimSpace(metadata.RequestID) == "" || req.Kind != "resubmit" && req.Kind != "review" || !workflowPositive(req.ExpectedSchemaVersion) || !workflowPositive(req.ExpectedRecordVersion) || !workflowPositive(req.ExpectedWorkflowRevision) {
		return empty, applications.ErrInvalid
	}
	w, old, loaded, err := s.beginWorkflowRoundWrite(ctx, p, req.WorkflowRoundRequest, req.OperationID, "workflow.round."+req.Kind, req)
	if err != nil {
		return empty, err
	}
	defer w.Rollback(context.Background())
	if old != nil {
		var v WorkflowRoundStartResult
		if !closedJSON(old.Data, &v) || old.Status != 202 || v.OperationID != req.OperationID || v.PreviousInstanceID != req.InstanceID || v.RoundKind != req.Kind {
			return empty, ErrUnavailable
		}
		if err = w.Commit(ctx); err != nil {
			return empty, err
		}
		return v, nil
	}
	v := loaded.Preview
	if v.SchemaVersion != req.ExpectedSchemaVersion || v.RecordVersion != req.ExpectedRecordVersion || v.WorkflowRevision != req.ExpectedWorkflowRevision {
		return empty, workflowcatalog.ErrConflict
	}
	if loaded.FlowState == "closing" {
		return empty, workflowcatalog.ErrClosing
	}
	if loaded.FlowState != "enabled" {
		return empty, workflowcatalog.ErrNotReady
	}
	if req.Kind == "resubmit" && !v.CanResubmit || req.Kind == "review" && !v.CanReview {
		return empty, applications.ErrDenied
	}
	id, err := workflowNewCommandID()
	if err != nil {
		return empty, err
	}
	next, err := (workflowcatalog.Catalog{}).ReserveInTx(ctx, w.Tx(), workflowcatalog.ReserveInput{AppID: req.AppID, FlowID: v.FlowID, InstanceID: id, RecordID: req.RecordID, ActorID: p.UserID, PreviousInstanceID: req.InstanceID, RoundKind: req.Kind, ExpectedRevision: req.ExpectedWorkflowRevision, ExpectedSchemaVersion: req.ExpectedSchemaVersion, ExpectedRecordVersion: req.ExpectedRecordVersion})
	if err != nil {
		return empty, err
	}
	var number int64
	if err = w.Tx().QueryRow(ctx, "SELECT round_number FROM applications.workflow_rounds WHERE instance_id=$1", next.ID).Scan(&number); err != nil {
		return empty, err
	}
	result := WorkflowRoundStartResult{OperationID: req.OperationID, FlowID: v.FlowID, PreviousInstanceID: req.InstanceID, InstanceID: next.ID, RoundKind: req.Kind, RoundNumber: number, Status: "accepted"}
	if err = finishWorkflowRoundWrite(ctx, w, req.OperationID, 202, result); err != nil {
		return empty, err
	}
	return result, nil
}
func (s *Service) ReworkWorkflowRound(ctx context.Context, p session.Principal, req WorkflowRoundReworkRequest, metadata applications.Metadata) (MutationResult, error) {
	empty := MutationResult{}
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || !roundRequestValid(req.WorkflowRoundRequest) || !workflowID(req.OperationID) || strings.TrimSpace(metadata.RequestID) == "" || !workflowPositive(req.ExpectedSchemaVersion) || !workflowPositive(req.ExpectedRecordVersion) || len(req.Changes) < 1 || len(req.Changes) > 200 {
		return empty, applications.ErrInvalid
	}
	for id := range req.Changes {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	w, old, loaded, err := s.beginWorkflowRoundWrite(ctx, p, req.WorkflowRoundRequest, req.OperationID, "record.edit", struct {
		Action  string
		Request WorkflowRoundReworkRequest
	}{"workflow.rework", req})
	if err != nil {
		return empty, err
	}
	defer w.Rollback(context.Background())
	if old != nil {
		var v MutationResult
		if !closedJSON(old.Data, &v) || old.Status != 200 || v.ID != req.RecordID || v.OperationID != req.OperationID {
			return empty, ErrUnavailable
		}
		if err = w.Commit(ctx); err != nil {
			return empty, err
		}
		return v, nil
	}
	v := loaded.Preview
	if v.SchemaVersion != req.ExpectedSchemaVersion || v.RecordVersion != req.ExpectedRecordVersion {
		return empty, workflowcatalog.ErrConflict
	}
	if !v.CanRework {
		return empty, applications.ErrDenied
	}
	facts := w.Context()
	fields, _, err := fieldsInContext(facts)
	if err != nil {
		return empty, err
	}
	selected := make([]string, 0, len(req.Changes))
	for id := range req.Changes {
		selected = append(selected, id)
	}
	if !loaded.Policy.CanEdit(loaded.Header.CreatedBy, selected) {
		return empty, applications.ErrDenied
	}
	values, err := normalizeRecordValues(req.Changes, fields, false)
	if err != nil {
		return empty, err
	}
	if err = validateNewReferences(ctx, w.Tx(), values, fields, false); err != nil {
		return empty, err
	}
	writer := apprecords.Writer{Gate: appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}, Authorization: recordAuthorization{policy: loaded.Policy}, Fence: appstructure.RecordFence{AppID: req.AppID, TableID: facts.TableID}, Audit: recordAudit{port: appstructure.RecordAudit{Context: facts, OperationID: req.OperationID, BeforeRecordVersion: v.RecordVersion, Metadata: metadata}}, DML: controlledDML{}}
	stored, err := writer.EditInTx(ctx, w.Tx(), loaded.Table, apprecords.Edit{OperationID: req.OperationID, ID: req.RecordID, ActorID: facts.Actor.ID, ExpectedSchemaVersion: req.ExpectedSchemaVersion, ExpectedRecordVersion: req.ExpectedRecordVersion, Changes: values})
	if err != nil {
		return empty, err
	}
	result := MutationResult{OperationID: stored.OperationID, ID: stored.ID, RecordVersion: stored.RecordVersion, SchemaVersion: stored.SchemaVersion, CreatedAt: stored.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: stored.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	if err = finishWorkflowRoundWrite(ctx, w, req.OperationID, 200, result); err != nil {
		return empty, err
	}
	return result, nil
}
