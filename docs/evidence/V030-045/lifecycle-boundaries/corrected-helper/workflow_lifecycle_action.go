package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
)

type WorkflowLifecycleActionRequest struct {
	WorkflowLifecycleRequest
	OperationID, Action, BasisToken, TargetNodeID string
}

func (s *Service) AcceptWorkflowLifecycle(ctx context.Context, p session.Principal, req WorkflowLifecycleActionRequest, metadata applications.Metadata) (WorkflowOperationResult, error) {
	empty := WorkflowOperationResult{}
	if s == nil || s.Pool == nil || s.WorkflowLifecycleBases == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || strings.TrimSpace(metadata.RequestID) == "" || (req.Action != "withdraw" && req.Action != "return") {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.InstanceID, req.OperationID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	if req.Action == "withdraw" && req.TargetNodeID != "" || req.Action == "return" && !workflowID(req.TargetNodeID) {
		return empty, applications.ErrInvalid
	}
	if len(req.BasisToken) > 2048 {
		return empty, ErrWorkflowBasisExpired
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return empty, applications.ErrInvalid
	}
	fingerprint := sha256.Sum256(raw)
	// Dependency checks happen before the transaction. They gate only a new
	// acceptance; a previously persisted operation remains recoverable.
	runtimeReady := true
	if s.RuntimeReady != nil {
		runtimeReady = s.RuntimeReady(ctx) == nil
	}
	var basis querycontext.Metadata
	loadErr := querycontext.ErrExpired
	if workflowActionToken(req.BasisToken) {
		basis, loadErr = s.WorkflowLifecycleBases.Load(ctx, p.SessionRef, req.BasisToken)
	}
	kind := workflowOperationKind(req.Action)
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout,
		OperationID: req.OperationID, Kind: kind, Fingerprint: fingerprint,
		Authorize: func(_ context.Context, _ pgx.Tx, facts applications.RecordContext) error {
			policy, menu := policyFor(facts)
			if !menu || policy.VisibleScope() == appaccess.None {
				return applications.ErrDenied
			}
			return nil
		}}
	write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, p, req.AppID, req.ViewID, options)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = write.Rollback(cleanup)
	}()
	old, err := write.Replay(ctx, req.OperationID, kind, fingerprint)
	if err != nil {
		return empty, err
	}
	if old != nil {
		// Release the write connection before the ordinary operation read, also
		// when the pool has only one connection.
		if err = write.Commit(ctx); err != nil {
			return empty, err
		}
		return s.WorkflowOperation(ctx, p, req.OperationID)
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
	var binding workflowLifecycleBinding
	var revision workflowBasisRevision
	if !workflowClosedMetadata(basis.Criteria, 65536, 13, &binding) || !workflowClosedMetadata(basis.Revision, 128, 2, &revision) {
		return empty, ErrWorkflowBasisExpired
	}
	if binding.ActorID != p.UserID || binding.AppID != req.AppID || binding.ViewID != req.ViewID || binding.RecordID != req.RecordID || binding.InstanceID != req.InstanceID {
		return empty, ErrWorkflowBasisChanged
	}
	if err = write.Claim(ctx, req.OperationID, kind, fingerprint); err != nil {
		return empty, err
	}
	facts, tx := write.Context(), write.Tx()
	if binding.TableID != facts.TableID || binding.ActorID != facts.Actor.ID {
		return empty, ErrWorkflowBasisChanged
	}
	if err = (appstructure.RecordGate{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID}).LockTable(ctx, tx, facts.TableID, facts.SchemaVersion); err != nil {
		return empty, err
	}
	// All task/instance projection transitions use this instance lock. Take it
	// after the existing application and table locks before reloading facts.
	var instanceID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM applications.workflow_instances
 WHERE app_id=$1 AND table_id=$2 AND view_id=$3 AND record_id=$4 AND id=$5 FOR UPDATE`,
		req.AppID, facts.TableID, req.ViewID, req.RecordID, req.InstanceID).Scan(&instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrWorkflowTaskChanged
	}
	if err != nil {
		return empty, err
	}
	loaded, err := loadWorkflowLifecycle(ctx, tx, facts, req.WorkflowLifecycleRequest)
	if err != nil {
		return empty, err
	}
	var taskID string
	var taskEpoch int64
	if req.Action == "withdraw" {
		if !loaded.Binding.CanWithdraw {
			return empty, applications.ErrDenied
		}
	} else {
		for _, target := range loaded.Binding.ReturnTargets {
			if target.NodeID == req.TargetNodeID {
				taskID, taskEpoch = target.TaskID, target.ActivationEpoch
				break
			}
		}
		if taskID == "" {
			return empty, ErrWorkflowTaskChanged
		}
	}
	if !reflect.DeepEqual(binding, loaded.Binding) {
		return empty, ErrWorkflowTaskChanged
	}
	if revision != loaded.Revision || hex.EncodeToString(loaded.Bundle.Hash[:]) != basis.Fingerprint {
		return empty, ErrWorkflowBasisChanged
	}
	if err = (appstructure.RecordFence{AppID: req.AppID, TableID: facts.TableID}).Check(ctx, tx, facts.TableID, req.RecordID, loaded.Revision.RecordVersion); err != nil {
		return empty, err
	}
	// Even withdraw carries the complete pinned graph's condition keys, as
	// required by the execution payload contract.
	routes, err := workflowActionRoutes(ctx, tx, facts, req.RecordID, loaded.GraphJSON)
	if err != nil {
		return empty, err
	}
	if !runtimeReady {
		return empty, ErrUnavailable
	}
	commandID, err := workflowNewCommandID()
	if err != nil {
		return empty, err
	}
	if _, err = (ev.Store{}).PutInTx(ctx, tx, loaded.Bundle); err != nil {
		return empty, err
	}
	var fenceEpoch int64
	if err = tx.QueryRow(ctx, "SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)",
		req.AppID, facts.TableID, req.ViewID, req.RecordID, commandID, loaded.Revision.SchemaVersion, loaded.Revision.RecordVersion).Scan(&fenceEpoch); err != nil {
		return empty, err
	}
	payload := fc.ExecutionPayload{EvidenceHash: loaded.Bundle.Hash, Routes: routes}
	payloadBytes, err := fc.EncodeExecutionPayload(req.Action, payload)
	if err != nil {
		return empty, err
	}
	command := fc.Command{ProtocolVersion: 2, CommandID: commandID, AppID: req.AppID, TableID: facts.TableID,
		ViewID: req.ViewID, RecordID: req.RecordID, InstanceID: req.InstanceID, ActorID: facts.Actor.ID, Action: req.Action,
		TaskID: taskID, TaskEpoch: taskEpoch, TargetNodeID: req.TargetNodeID,
		RecordVersion: loaded.Revision.RecordVersion, SchemaVersion: loaded.Revision.SchemaVersion, FenceEpoch: fenceEpoch,
		ExpectedSequence: loaded.Binding.Sequence, FlowID: loaded.Binding.FlowID, VersionID: loaded.Binding.VersionID,
		DefinitionVersion: loaded.Binding.DefinitionVersion, PayloadHash: sha256.Sum256(payloadBytes)}
	if _, err = (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(ctx, tx, command, payload); err != nil {
		return empty, err
	}
	result := WorkflowOperationResult{OperationID: req.OperationID, CommandID: commandID, InstanceID: req.InstanceID, Status: "pending"}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return empty, err
	}
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 202, Location: "/api/v1/application-workflow-operations/" + req.OperationID, Data: resultBytes}); err != nil {
		return empty, err
	}
	if err = write.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}
