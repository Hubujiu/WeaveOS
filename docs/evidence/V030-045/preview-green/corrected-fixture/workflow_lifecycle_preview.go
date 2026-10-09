package apprecordservice

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type WorkflowLifecycleRequest struct{ AppID, ViewID, RecordID, InstanceID string }
type WorkflowLifecycleInfo struct {
	ID       string `json:"id"`
	Sequence int64  `json:"sequence"`
	State    string `json:"state"`
}
type WorkflowReturnTarget struct {
	NodeID          string `json:"nodeId"`
	TaskID          string `json:"taskId"`
	ActivationEpoch int64  `json:"activationEpoch"`
}
type WorkflowLifecyclePreview struct {
	BasisToken    string                 `json:"basisToken"`
	Instance      WorkflowLifecycleInfo  `json:"instance"`
	Record        Record                 `json:"record"`
	Fields        []appfields.Field      `json:"fields"`
	CanWithdraw   bool                   `json:"canWithdraw"`
	ReturnTargets []WorkflowReturnTarget `json:"returnTargets"`
}

// The Store's Session-specific key binds this metadata to the original Session.
// These facts are observations, never an authorization grant to the next write.
type workflowLifecycleBinding struct {
	ActorID           string                 `json:"actorId"`
	AppID             string                 `json:"appId"`
	TableID           string                 `json:"tableId"`
	ViewID            string                 `json:"viewId"`
	RecordID          string                 `json:"recordId"`
	InstanceID        string                 `json:"instanceId"`
	FlowID            string                 `json:"flowId"`
	VersionID         string                 `json:"versionId"`
	DefinitionVersion int64                  `json:"definitionVersion"`
	Sequence          int64                  `json:"sequence"`
	State             string                 `json:"state"`
	CanWithdraw       bool                   `json:"canWithdraw"`
	ReturnTargets     []WorkflowReturnTarget `json:"returnTargets"`
}

type workflowLifecycleLoaded struct {
	Binding   workflowLifecycleBinding
	Revision  workflowBasisRevision
	Bundle    ev.Bundle
	Preview   WorkflowLifecyclePreview
	GraphJSON []byte
}

func newWorkflowLifecycleStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-lifecycle", generation, querycontext.Policy{Validate: func(m querycontext.Metadata) bool {
		if m.View != "workflow-lifecycle" || m.Total != 0 || m.ProtocolVersion != 1 {
			return false
		}
		var binding workflowLifecycleBinding
		var revision workflowBasisRevision
		if !workflowClosedMetadata(m.Criteria, 65536, 13, &binding) || !workflowClosedMetadata(m.Revision, 128, 2, &revision) {
			return false
		}
		for _, id := range []string{binding.ActorID, binding.AppID, binding.TableID, binding.ViewID, binding.RecordID, binding.InstanceID, binding.FlowID, binding.VersionID} {
			if !workflowID(id) {
				return false
			}
		}
		if binding.State != "active" || !workflowPositive(binding.DefinitionVersion) || binding.Sequence < 0 || binding.Sequence > workflowMaxCounter || !workflowPositive(revision.SchemaVersion) || !workflowPositive(revision.RecordVersion) || len(binding.ReturnTargets) > 100 || (!binding.CanWithdraw && len(binding.ReturnTargets) == 0) {
			return false
		}
		for i, target := range binding.ReturnTargets {
			if !workflowID(target.NodeID) || !workflowID(target.TaskID) || !workflowPositive(target.ActivationEpoch) || i > 0 && binding.ReturnTargets[i-1].NodeID >= target.NodeID {
				return false
			}
		}
		return true
	}})
}

type workflowLifecycleObservedTask struct {
	workflowLifecycleTask
	ID              string
	ActivationEpoch int64
}

func newerWorkflowLifecycleTask(task, previous workflowLifecycleObservedTask) bool {
	return task.ActivationEpoch > previous.ActivationEpoch || task.ActivationEpoch == previous.ActivationEpoch && task.ID < previous.ID
}

// loadWorkflowLifecycle uses only the caller-owned transaction and its live
// RecordContext. Preview calls it in RR; an accepting write must reload it under
// that write's existing authority/resource locks and compare the saved basis.
func loadWorkflowLifecycle(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, req WorkflowLifecycleRequest) (workflowLifecycleLoaded, error) {
	empty := workflowLifecycleLoaded{}
	if captureNilPort(ctx) || captureNilPort(tx) {
		return empty, ErrUnavailable
	}
	if facts.App.ID != req.AppID || facts.ViewID != req.ViewID {
		return empty, applications.ErrInvalid
	}
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None {
		return empty, applications.ErrDenied
	}
	loaded := workflowLifecycleLoaded{Binding: workflowLifecycleBinding{
		ActorID: facts.Actor.ID, AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID,
		RecordID: req.RecordID, InstanceID: req.InstanceID, ReturnTargets: []WorkflowReturnTarget{},
	}}
	binding := &loaded.Binding
	var initiator string
	var allowWithdraw bool
	err := tx.QueryRow(ctx, `SELECT i.initiator_id::text,i.state,i.sequence,i.flow_id::text,
 i.definition_version,v.version_id::text,v.allow_withdraw,v.graph_json
 FROM applications.workflow_instances i
 JOIN applications.workflow_versions v ON v.app_id=i.app_id AND v.flow_id=i.flow_id AND v.version=i.definition_version
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.view_id=$3 AND i.record_id=$4 AND i.id=$5`,
		req.AppID, facts.TableID, req.ViewID, req.RecordID, req.InstanceID).Scan(
		&initiator, &binding.State, &binding.Sequence, &binding.FlowID, &binding.DefinitionVersion,
		&binding.VersionID, &allowWithdraw, &loaded.GraphJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	if !workflowID(initiator) || !workflowID(binding.FlowID) || !workflowID(binding.VersionID) || !workflowPositive(binding.DefinitionVersion) || binding.Sequence < 0 || binding.Sequence > workflowMaxCounter {
		return empty, ErrUnavailable
	}
	loaded.Bundle, err = captureWorkflowEvidence(ctx, tx, facts, req.RecordID)
	if err != nil {
		return empty, err
	}
	if binding.State != "active" {
		return empty, ErrWorkflowTaskChanged
	}
	visible, revision, err := workflowVisiblePreview(loaded.Bundle)
	if err != nil {
		return empty, err
	}
	loaded.Revision = revision

	// A task closed by return/withdraw or another actor's command is not an
	// approval. Match the confirmed event and its exact original task command.
	rows, err := tx.Query(ctx, `SELECT t.id::text,t.node_id::text,t.assignee_id::text,t.activation_epoch,
 t.closed_command_id IS NOT NULL,
 COALESCE(e.action='agree' AND e.outcome='success' AND c.state='success'
 AND c.command_json->>'ProtocolVersion'='2'
 AND c.command_json->>'AppID'=t.app_id::text
 AND c.command_json->>'InstanceID'=t.instance_id::text
 AND c.command_json->>'TaskID'=t.id::text
 AND c.command_json->>'TaskEpoch'=t.activation_epoch::text
 AND c.command_json->>'ActorID'=t.assignee_id::text
 AND c.command_json->>'Action'='agree',false)
 FROM applications.workflow_tasks t
 LEFT JOIN applications.workflow_execution_events e ON e.command_id=t.closed_command_id
 AND e.app_id=t.app_id AND e.instance_id=t.instance_id AND e.actor_id=t.assignee_id
 LEFT JOIN applications.workflow_commands c ON c.command_id=e.command_id
 WHERE t.app_id=$1 AND t.instance_id=$2`, req.AppID, req.InstanceID)
	if err != nil {
		return empty, err
	}
	defer rows.Close()
	observed := []workflowLifecycleObservedTask{}
	lifecycle := workflowLifecycleFacts{ActorID: facts.Actor.ID, InitiatorID: initiator, InstanceID: req.InstanceID,
		State: binding.State, AllowWithdraw: allowWithdraw, CanRead: true}
	var current workflowLifecycleObservedTask
	historical := make(map[string]workflowLifecycleObservedTask)
	for rows.Next() {
		task := workflowLifecycleObservedTask{workflowLifecycleTask: workflowLifecycleTask{InstanceID: req.InstanceID}}
		var approved bool
		if err = rows.Scan(&task.ID, &task.NodeID, &task.ActorID, &task.ActivationEpoch, &task.Closed, &approved); err != nil {
			return empty, err
		}
		if !workflowID(task.ID) || !workflowID(task.NodeID) || !workflowID(task.ActorID) || !workflowPositive(task.ActivationEpoch) {
			return empty, ErrUnavailable
		}
		if approved {
			task.ClosedAction, task.ClosedOutcome = "agree", "success"
		}
		lifecycle.Tasks = append(lifecycle.Tasks, task.workflowLifecycleTask)
		observed = append(observed, task)
		if task.ActorID != facts.Actor.ID {
			continue
		}
		if !task.Closed {
			if current.ID == "" || newerWorkflowLifecycleTask(task, current) {
				current = task
			}
		} else if approved {
			previous, found := historical[task.NodeID]
			if !found || newerWorkflowLifecycleTask(task, previous) {
				historical[task.NodeID] = task
			}
		}
	}
	if err = rows.Err(); err != nil {
		return empty, err
	}
	rows.Close()
	capabilities := workflowLifecyclePolicy(lifecycle)
	if !capabilities.Withdraw && len(capabilities.ReturnTargets) == 0 {
		return empty, applications.ErrDenied
	}
	// Fixed definition rosters provide a consistency check on the projected
	// actor tasks; newer definitions cannot alter an in-flight instance.
	checked := make(map[string]bool)
	for _, task := range observed {
		if task.ActorID != facts.Actor.ID || task.Closed && task.ClosedOutcome != "success" || checked[task.NodeID] {
			continue
		}
		if !workflowApprovalRoster(loaded.GraphJSON, task.NodeID, facts.Actor.ID) {
			return empty, ErrUnavailable
		}
		checked[task.NodeID] = true
	}
	for _, nodeID := range capabilities.ReturnTargets {
		task := current
		if task.ID == "" {
			task = historical[nodeID]
		}
		if task.ID == "" {
			return empty, ErrUnavailable
		}
		binding.ReturnTargets = append(binding.ReturnTargets, WorkflowReturnTarget{NodeID: nodeID, TaskID: task.ID, ActivationEpoch: task.ActivationEpoch})
	}
	sort.Slice(binding.ReturnTargets, func(i, j int) bool { return binding.ReturnTargets[i].NodeID < binding.ReturnTargets[j].NodeID })
	binding.CanWithdraw = capabilities.Withdraw
	loaded.Preview = WorkflowLifecyclePreview{Instance: WorkflowLifecycleInfo{ID: req.InstanceID, Sequence: binding.Sequence, State: binding.State},
		Record: visible.Record, Fields: visible.Fields, CanWithdraw: binding.CanWithdraw, ReturnTargets: binding.ReturnTargets}
	return loaded, nil
}

func (s *Service) PreviewWorkflowLifecycle(ctx context.Context, p session.Principal, req WorkflowLifecycleRequest) (WorkflowLifecyclePreview, error) {
	empty := WorkflowLifecyclePreview{}
	if s == nil || s.Pool == nil || s.WorkflowLifecycleBases == nil || captureNilPort(ctx) {
		return empty, ErrUnavailable
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.InstanceID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	tx, facts, err := (&applications.Application{Pool: s.Pool}).BeginRecordRead(ctx, p, req.AppID, req.ViewID)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	loaded, err := loadWorkflowLifecycle(ctx, tx, facts, req)
	if err != nil {
		return empty, err
	}
	criteria, err := json.Marshal(loaded.Binding)
	if err != nil {
		return empty, ErrUnavailable
	}
	revision, err := json.Marshal(loaded.Revision)
	if err != nil {
		return empty, ErrUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	token, err := s.WorkflowLifecycleBases.Create(ctx, p.SessionRef, querycontext.Metadata{View: "workflow-lifecycle",
		Criteria: criteria, Revision: revision, Total: 0, Fingerprint: hex.EncodeToString(loaded.Bundle.Hash[:]), ProtocolVersion: 1})
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		return empty, ErrUnavailable
	}
	loaded.Preview.BasisToken = token
	return loaded.Preview, nil
}
