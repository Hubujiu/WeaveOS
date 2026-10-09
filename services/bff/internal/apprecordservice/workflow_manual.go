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
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type WorkflowManualStartRequest struct {
	AppID, ViewID, RecordID, FlowID, OperationID                           string
	ExpectedWorkflowRevision, ExpectedSchemaVersion, ExpectedRecordVersion int64
}
type WorkflowManualStartResult struct {
	OperationID string `json:"operationId"`
	FlowID      string `json:"flowId"`
	InstanceID  string `json:"instanceId"`
	Status      string `json:"status"`
	Ignored     bool   `json:"ignored"`
}

const manualStartKind = "workflow.manual.start"

func (s *Service) StartManualWorkflow(ctx context.Context, p session.Principal, req WorkflowManualStartRequest, metadata applications.Metadata) (WorkflowManualStartResult, error) {
	empty := WorkflowManualStartResult{}
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || strings.TrimSpace(metadata.RequestID) == "" {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.FlowID, req.OperationID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	for _, v := range []int64{req.ExpectedWorkflowRevision, req.ExpectedSchemaVersion, req.ExpectedRecordVersion} {
		if !workflowPositive(v) {
			return empty, applications.ErrInvalid
		}
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return empty, applications.ErrInvalid
	}
	fingerprint := sha256.Sum256(raw)
	var policy appaccess.Policy
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: req.OperationID, Kind: manualStartKind, Fingerprint: fingerprint,
		Authorize: func(_ context.Context, _ pgx.Tx, facts applications.RecordContext) error {
			var menu bool
			policy, menu = policyFor(facts)
			if !menu || policy.VisibleScope() == appaccess.None {
				return applications.ErrDenied
			}
			return nil
		},
	}
	write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, p, req.AppID, req.ViewID, options)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = write.Rollback(cleanup)
	}()
	old, err := write.Replay(ctx, req.OperationID, manualStartKind, fingerprint)
	if err != nil {
		return empty, err
	}
	if old != nil {
		if err = write.Commit(ctx); err != nil {
			return empty, err
		}
		var result WorkflowManualStartResult
		if json.Unmarshal(old.Data, &result) != nil {
			return empty, ErrUnavailable
		}
		return result, nil
	}
	if err = write.Claim(ctx, req.OperationID, manualStartKind, fingerprint); err != nil {
		return empty, err
	}
	facts, tx := write.Context(), write.Tx()
	if facts.SchemaVersion != req.ExpectedSchemaVersion {
		return empty, workflowcatalog.ErrConflict
	}
	if err = (appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}).LockTable(ctx, tx, facts.TableID, facts.SchemaVersion); err != nil {
		return empty, err
	}
	_, ids, err := fieldsInContext(facts)
	if err != nil {
		return empty, err
	}
	table := apprecords.Table{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID, Namespace: "appdata", SchemaVersion: facts.SchemaVersion, Ready: facts.SchemaReady, ActiveFieldIDs: ids}
	header, err := (controlledDML{}).LockHeader(ctx, tx, table, req.RecordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	if header.CreatedBy != facts.Actor.ID || len(policy.ReadFields(header.CreatedBy, ids)) == 0 {
		return empty, applications.ErrDenied
	}
	if header.RecordVersion != req.ExpectedRecordVersion {
		return empty, workflowcatalog.ErrConflict
	}
	id, err := workflowNewCommandID()
	if err != nil {
		return empty, err
	}
	reservation, err := (workflowcatalog.Catalog{}).ReserveManualInTx(ctx, tx, workflowcatalog.ManualReserveInput{TableID: facts.TableID, ReserveInput: workflowcatalog.ReserveInput{AppID: req.AppID, FlowID: req.FlowID, InstanceID: id, RecordID: req.RecordID, ActorID: facts.Actor.ID, ExpectedRevision: req.ExpectedWorkflowRevision, ExpectedSchemaVersion: req.ExpectedSchemaVersion, ExpectedRecordVersion: req.ExpectedRecordVersion}})
	if err != nil {
		return empty, err
	}
	result := WorkflowManualStartResult{OperationID: req.OperationID, FlowID: req.FlowID, InstanceID: reservation.Instance.ID, Status: "accepted", Ignored: reservation.Ignored}
	if result.Ignored {
		result.Status = "ignored"
	}
	body, err := json.Marshal(result)
	if err != nil {
		return empty, err
	}
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 202, Location: "/api/v1/application-operations/" + req.OperationID, Data: body}); err != nil {
		return empty, err
	}
	if err = write.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}
