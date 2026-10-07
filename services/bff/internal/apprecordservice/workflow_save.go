package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
)

type WorkflowTaskSaveRequest struct {
	WorkflowTaskRequest
	OperationID, BasisToken string
	Changes                 map[string]any
}

// Keep the existing typed ports and replace only the history origin for CAS.
type workflowSaveDML struct {
	controlledDML
	taskRef string
}

func (d workflowSaveDML) UpdateCAS(ctx context.Context, tx pgx.Tx, table apprecords.Table, in apprecords.Edit, ids []string) (apprecords.StoredHeader, error) {
	h, err := (appstructure.RecordHistoryDML{History: appstructure.RecordHistoryStore{}, Origin: "task_save", OpaqueTaskRef: &d.taskRef}).UpdateCAS(ctx, tx, appstructure.RecordTable(table), appstructure.RecordEdit(in), ids)
	return apprecords.StoredHeader(h), err
}

func (s *Service) SaveWorkflowTask(ctx context.Context, principal session.Principal, req WorkflowTaskSaveRequest, metadata applications.Metadata) (MutationResult, error) {
	empty := MutationResult{}
	if s == nil || s.Pool == nil || s.WorkflowBases == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || strings.TrimSpace(metadata.RequestID) == "" || len(req.Changes) == 0 || len(req.Changes) > 200 {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.InstanceID, req.TaskID, req.OperationID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	selected := make([]string, 0, len(req.Changes))
	for id := range req.Changes {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
		selected = append(selected, id)
	}
	sort.Strings(selected)
	if len(req.BasisToken) > 2048 {
		return empty, ErrWorkflowBasisExpired
	}
	canonical, err := json.Marshal(struct {
		Kind string
		Body WorkflowTaskSaveRequest
	}{Kind: "workflow.task.save", Body: req})
	if err != nil {
		return empty, applications.ErrInvalid
	}
	fingerprint := sha256.Sum256(canonical)
	var basis querycontext.Metadata
	loadErr := querycontext.ErrExpired
	if workflowActionToken(req.BasisToken) {
		basis, loadErr = s.WorkflowBases.Load(ctx, principal.SessionRef, req.BasisToken)
	}
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: req.OperationID, Kind: "record.edit", Fingerprint: fingerprint,
		Authorize: func(c context.Context, tx pgx.Tx, facts applications.RecordContext) error {
			policy, menu := policyFor(facts)
			if !menu || policy.VisibleScope() == appaccess.None {
				return applications.ErrDenied
			}
			if !facts.SchemaReady {
				return &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
			}
			// Authorize this actual row before disclosing task state. This read
			// takes no row lock; write locks still follow table, task, record.
			relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
			var owner string
			if e := tx.QueryRow(c, "SELECT created_by::text FROM "+relation+" WHERE id=$1", req.RecordID).Scan(&owner); errors.Is(e, pgx.ErrNoRows) {
				return applications.ErrMissing
			} else if e != nil {
				return e
			}
			if policy.VisibleScope() == appaccess.Own && owner != facts.Actor.ID {
				return applications.ErrMissing
			}
			return nil
		}}
	write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, principal, req.AppID, req.ViewID, options)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = write.Rollback(cleanup)
	}()
	old, err := write.Replay(ctx, req.OperationID, "record.edit", fingerprint)
	if err != nil {
		return empty, err
	}
	if old != nil {
		// RecordWrite deliberately recovers confirmed results before action
		// checks. Supplement that with the established current row read path;
		// an own-only grant must not disclose a different owner's receipt.
		if _, err = s.GetRecord(ctx, principal, req.AppID, req.ViewID, req.RecordID); err != nil {
			return empty, err
		}
		var result MutationResult
		if !closedJSON(old.Data, &result) || old.Status != 200 || result.ID != req.RecordID || result.OperationID != req.OperationID {
			return empty, ErrUnavailable
		}
		if err = write.Commit(ctx); err != nil {
			return empty, err
		}
		return result, nil
	}
	if loadErr != nil {
		if errors.Is(loadErr, querycontext.ErrExpired) || errors.Is(loadErr, querycontext.ErrInvalid) {
			return empty, ErrWorkflowBasisExpired
		}
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		return empty, ErrUnavailable
	}
	var binding workflowBasisBinding
	var revision workflowBasisRevision
	if !workflowClosedMetadata(basis.Criteria, 2048, 11, &binding) || !workflowClosedMetadata(basis.Revision, 128, 2, &revision) {
		return empty, ErrWorkflowBasisExpired
	}
	if binding.AppID != req.AppID || binding.ViewID != req.ViewID || binding.RecordID != req.RecordID || binding.InstanceID != req.InstanceID || binding.TaskID != req.TaskID {
		return empty, ErrWorkflowBasisChanged
	}
	if err = write.Claim(ctx, req.OperationID, "record.edit", fingerprint); err != nil {
		return empty, err
	}
	facts := write.Context()
	tx := write.Tx()
	if binding.TableID != facts.TableID {
		return empty, ErrWorkflowBasisChanged
	}
	gate := appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}
	if err = gate.LockTable(ctx, tx, facts.TableID, facts.SchemaVersion); err != nil {
		return empty, err
	}
	var flowID, versionID, nodeID, assignee, state string
	var definitionVersion, sequence, epoch int64
	var closed bool
	var graphJSON []byte
	err = tx.QueryRow(ctx, `SELECT i.flow_id::text,i.definition_version,i.sequence,i.state,v.version_id::text,v.graph_json,
 t.node_id::text,t.assignee_id::text,t.activation_epoch,t.closed_command_id IS NOT NULL
 FROM applications.workflow_instances i
 JOIN applications.workflow_versions v ON v.app_id=i.app_id AND v.flow_id=i.flow_id AND v.version=i.definition_version
 JOIN applications.workflow_tasks t ON t.app_id=i.app_id AND t.instance_id=i.id
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.view_id=$3 AND i.record_id=$4 AND i.id=$5 AND t.id=$6
 FOR UPDATE OF i,t`, req.AppID, facts.TableID, req.ViewID, req.RecordID, req.InstanceID, req.TaskID).Scan(&flowID, &definitionVersion, &sequence, &state, &versionID, &graphJSON, &nodeID, &assignee, &epoch, &closed)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrWorkflowTaskChanged
	}
	if err != nil {
		return empty, err
	}
	if assignee != facts.Actor.ID {
		return empty, applications.ErrDenied
	}
	editable, valid := workflowApprovalEditableFields(graphJSON, nodeID, assignee)
	if !workflowID(flowID) || !valid {
		return empty, ErrUnavailable
	}
	if closed || state != "active" || epoch != binding.ActivationEpoch || sequence != binding.Sequence || definitionVersion != binding.DefinitionVersion || versionID != binding.VersionID || nodeID != binding.NodeID {
		return empty, ErrWorkflowTaskChanged
	}
	fields, active, err := fieldsInContext(facts)
	if err != nil {
		return empty, err
	}
	table := apprecords.Table{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID, Namespace: "appdata", SchemaVersion: facts.SchemaVersion, Ready: facts.SchemaReady, ActiveFieldIDs: active}
	if _, err = (controlledDML{}).LockHeader(ctx, tx, table, req.RecordID); err != nil {
		return empty, err
	}
	bundle, err := captureWorkflowEvidence(ctx, tx, facts, req.RecordID)
	if err != nil {
		return empty, err
	}
	manifest, err := ev.DecodeManifest(bundle.Manifest)
	if err != nil {
		return empty, ErrUnavailable
	}
	h := manifest.Header
	if hex.EncodeToString(bundle.Hash[:]) != basis.Fingerprint || h.SchemaVersion != revision.SchemaVersion || h.RecordVersion != revision.RecordVersion {
		return empty, ErrWorkflowBasisChanged
	}
	fence := appstructure.RecordFence{AppID: req.AppID, TableID: facts.TableID}
	if err = fence.Check(ctx, tx, facts.TableID, req.RecordID, h.RecordVersion); err != nil {
		return empty, err
	}
	policy, _ := policyFor(facts)
	for _, id := range selected {
		if !editable[id] {
			return empty, applications.ErrDenied
		}
	}
	if !policy.CanEdit(h.CreatedBy, selected) {
		return empty, applications.ErrDenied
	}
	values, err := normalizeRecordValues(req.Changes, fields, false)
	if err != nil {
		return empty, err
	}
	if err = validateNewReferences(ctx, tx, values, fields, false); err != nil {
		return empty, err
	}
	ref, err := json.Marshal(struct {
		Version           int    `json:"version"`
		FlowID            string `json:"flowId"`
		InstanceID        string `json:"instanceId"`
		TaskID            string `json:"taskId"`
		NodeID            string `json:"nodeId"`
		DefinitionVersion int64  `json:"definitionVersion"`
		ActivationEpoch   int64  `json:"activationEpoch"`
	}{1, flowID, req.InstanceID, req.TaskID, nodeID, definitionVersion, epoch})
	if err != nil {
		return empty, ErrUnavailable
	}
	writer := apprecords.Writer{Gate: gate, Authorization: recordAuthorization{policy: policy}, Fence: fence,
		Audit: recordAudit{port: appstructure.RecordAudit{Context: facts, OperationID: req.OperationID, BeforeRecordVersion: h.RecordVersion, Metadata: metadata}},
		DML:   workflowSaveDML{taskRef: string(ref)}}
	stored, err := writer.EditInTx(ctx, tx, table, apprecords.Edit{OperationID: req.OperationID, ID: req.RecordID, ActorID: facts.Actor.ID, ExpectedSchemaVersion: h.SchemaVersion, ExpectedRecordVersion: h.RecordVersion, Changes: values})
	if err != nil {
		return empty, err
	}
	result := MutationResult{OperationID: stored.OperationID, ID: stored.ID, RecordVersion: stored.RecordVersion, SchemaVersion: stored.SchemaVersion, CreatedAt: stored.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: stored.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	raw, err := json.Marshal(result)
	if err != nil {
		return empty, ErrUnavailable
	}
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 200, Location: "", Data: raw}); err != nil {
		return empty, err
	}
	if err = write.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}
