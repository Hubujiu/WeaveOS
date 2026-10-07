// Package workflowprojection persists a confirmed execution result in the
// caller's transaction. It is not a user authorization or RPC entry point.
package workflowprojection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/jackc/pgx/v5"
)

var ErrUnimplemented = errors.New("execution projection not implemented")
var ErrConflict = errors.New("execution projection conflicts with durable state")
var ErrInvalid = errors.New("invalid execution projection")

type Store struct{}
type Applied struct {
	Entry     flowcommands.Entry
	Result    flowcommands.ExecutionResult
	Duplicate bool
}

// ApplyInTx validates the original payload and result body, then uses a nested
// transaction/savepoint to make all projection writes atomic even if the caller
// mistakenly commits after an error. Success never commits the outer transaction.
// The caller must acquire any personnel authority locks before calling; this
// method does not reevaluate live identity while recovering accepted commands.
func (Store) ApplyInTx(ctx context.Context, tx pgx.Tx, command flowcommands.Command,
	payload flowcommands.ExecutionPayload, receipt flowcommands.Receipt, body []byte) (applied Applied, err error) {
	if nilPort(ctx) || nilPort(tx) || command.ProtocolVersion != 2 || len(body) < 8 || len(body) > 65536 {
		return Applied{}, ErrInvalid
	}
	encoded, err := flowcommands.EncodeExecutionPayload(command.Action, payload)
	if err != nil {
		return Applied{}, err
	}
	if sha256.Sum256(encoded) != command.PayloadHash {
		return Applied{}, ErrConflict
	}
	// Own the body used for both validation and durable event insertion.
	body = bytes.Clone(body)
	result, err := flowcommands.DecodeExecutionResult(command, receipt, body)
	if err != nil {
		return Applied{}, err
	}
	nested, err := tx.Begin(ctx)
	if err != nil {
		return Applied{}, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if rollbackErr := nested.Rollback(cleanup); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
			applied = Applied{}
		}
	}()

	instance, schema, ready, graph, err := lockInstance(ctx, nested, command)
	if err != nil {
		return Applied{}, err
	}
	// Acquire the command lock only after the application, logical table and instance.
	var commandID string
	if err = nested.QueryRow(ctx, `SELECT command_id::text FROM applications.workflow_commands
		WHERE command_id=$1 FOR UPDATE`, command.CommandID).Scan(&commandID); err != nil {
		return Applied{}, projectionDBError(err)
	}
	ledger := flowcommands.Ledger{Namespace: "applications"}
	existing, err := ledger.GetInTx(ctx, nested, command.CommandID)
	if err != nil {
		return Applied{}, err
	}
	if existing.Command != command {
		return Applied{}, ErrConflict
	}
	if existing.State != "pending" {
		// Later schema edits, instance transitions and new fences do not invalidate replay.
		entry, replayErr := ledger.ApplyInTx(ctx, nested, command, receipt, instance.sequence,
			func(context.Context, pgx.Tx, flowcommands.ApplyPlan) error { return ErrConflict })
		if replayErr != nil {
			return Applied{}, replayErr
		}
		if err = verifyEvent(ctx, nested, command, payload, receipt, encoded, body); err != nil {
			return Applied{}, err
		}
		if err = finalizeRequestedClose(ctx, nested, command); err != nil {
			return Applied{}, err
		}
		if err = nested.Commit(ctx); err != nil {
			return Applied{}, err
		}
		committed = true
		return Applied{Entry: entry, Result: result, Duplicate: true}, nil
	}
	if instance.sequence != command.ExpectedSequence || !ready || schema != command.SchemaVersion {
		return Applied{}, ErrConflict
	}
	if err = checkFence(ctx, nested, command); err != nil {
		return Applied{}, err
	}

	var changes taskChanges
	if receipt.Outcome == "no_effect" {
		if command.Action == "start" {
			if !initialInstance(instance, command) {
				return Applied{}, ErrConflict
			}
			switch result.Reason {
			case "cancelled", "deployment_missing", "deployment_mismatch":
			default:
				return Applied{}, ErrConflict
			}
		}
	} else {
		if err = validateSuccess(instance, command, result); err != nil {
			return Applied{}, err
		}
		active, loadErr := activeTasks(ctx, nested, command)
		if loadErr != nil {
			return Applied{}, loadErr
		}
		if err = validateAction(ctx, nested, ledger, instance, command, result, active); err != nil {
			return Applied{}, err
		}
		changes, err = planTasks(ctx, nested, command, result, graph, active)
		if err != nil {
			return Applied{}, err
		}
	}

	entry, err := ledger.ApplyInTx(ctx, nested, command, receipt, instance.sequence,
		func(ctx context.Context, tx pgx.Tx, plan flowcommands.ApplyPlan) error {
			if _, err := tx.Exec(ctx, `INSERT INTO applications.workflow_execution_events
				(command_id,app_id,instance_id,actor_id,action,outcome,sequence,schema_version,
				record_version,evidence_hash,payload_bytes,result_bytes,proof_id)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, command.CommandID,
				command.AppID, command.InstanceID, command.ActorID, command.Action, plan.Outcome,
				plan.Sequence, command.SchemaVersion, command.RecordVersion, payload.EvidenceHash[:],
				encoded, body, receipt.ProofID); err != nil {
				return err
			}
			if plan.Outcome == "success" {
				if err := writeTasks(ctx, tx, command, changes); err != nil {
					return err
				}
				tag, err := tx.Exec(ctx, `UPDATE applications.workflow_instances
					SET state=$3,sequence=$4,engine_process_id=$5,updated_at=clock_timestamp()
					WHERE app_id=$1 AND id=$2`, command.AppID, command.InstanceID, result.State,
					plan.Sequence, result.EngineProcessID)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return ErrConflict
				}
			} else if command.Action == "start" {
				tag, err := tx.Exec(ctx, `UPDATE applications.workflow_instances
					SET state='no_effect',updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`,
					command.AppID, command.InstanceID)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return ErrConflict
				}
			}
			var released bool
			if err := tx.QueryRow(ctx, `SELECT applications.release_record_command_fence($1,$2,$3,$4,$5,$6)`,
				command.AppID, command.TableID, command.RecordID, command.CommandID,
				command.FenceEpoch, command.RecordVersion).Scan(&released); err != nil {
				return err
			}
			if !released {
				return ErrConflict
			}
			return nil
		})
	if err != nil {
		return Applied{}, err
	}
	if err = finalizeRequestedClose(ctx, nested, command); err != nil {
		return Applied{}, err
	}
	if err = nested.Commit(ctx); err != nil {
		return Applied{}, err
	}
	committed = true
	return Applied{Entry: entry, Result: result}, nil
}

func nilPort(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func projectionDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	return err
}

type instanceSnapshot struct {
	initiator, state string
	sequence         int64
	engine           *string
}

func lockInstance(ctx context.Context, tx pgx.Tx, command flowcommands.Command) (instanceSnapshot, int64, bool, flowgraph.Graph, error) {
	var snapshot instanceSnapshot
	var graph flowgraph.Graph
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM applications.apps WHERE id=$1 FOR UPDATE`, command.AppID).Scan(&id); err != nil {
		return snapshot, 0, false, graph, projectionDBError(err)
	}
	var schema int64
	var ready bool
	if err := tx.QueryRow(ctx, `SELECT schema_version,schema_ready FROM applications.logical_tables
		WHERE app_id=$1 AND id=$2 FOR UPDATE`, command.AppID, command.TableID).Scan(&schema, &ready); err != nil {
		return snapshot, 0, false, graph, projectionDBError(err)
	}
	var app, table, view, record, flow string
	var version int64
	if err := tx.QueryRow(ctx, `SELECT app_id::text,table_id::text,view_id::text,record_id::text,
		flow_id::text,definition_version,initiator_id::text,state,sequence,engine_process_id
		FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 FOR UPDATE`, command.AppID, command.InstanceID).Scan(
		&app, &table, &view, &record, &flow, &version, &snapshot.initiator, &snapshot.state,
		&snapshot.sequence, &snapshot.engine); err != nil {
		return snapshot, 0, false, graph, projectionDBError(err)
	}
	if app != command.AppID || table != command.TableID || view != command.ViewID || record != command.RecordID ||
		flow != command.FlowID || version != command.DefinitionVersion {
		return snapshot, 0, false, graph, ErrConflict
	}
	var versionID string
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT version_id::text,graph_json FROM applications.workflow_versions
		WHERE app_id=$1 AND flow_id=$2 AND version=$3`, command.AppID, command.FlowID,
		command.DefinitionVersion).Scan(&versionID, &raw); err != nil {
		return snapshot, 0, false, graph, projectionDBError(err)
	}
	// Definition identity is fixed; its original schema_version need not equal today's record schema.
	if versionID != command.VersionID || json.Unmarshal(raw, &graph) != nil {
		return snapshot, 0, false, graph, ErrConflict
	}
	return snapshot, schema, ready, graph, nil
}

func verifyEvent(ctx context.Context, tx pgx.Tx, command flowcommands.Command, payload flowcommands.ExecutionPayload,
	receipt flowcommands.Receipt, encoded, body []byte) error {
	var app, instance, actor, action, outcome, proof string
	var sequence, schema, record int64
	var evidence, storedPayload, storedResult []byte
	if err := tx.QueryRow(ctx, `SELECT app_id::text,instance_id::text,actor_id::text,action,outcome,
		sequence,schema_version,record_version,evidence_hash,payload_bytes,result_bytes,proof_id::text
		FROM applications.workflow_execution_events WHERE command_id=$1`, command.CommandID).Scan(
		&app, &instance, &actor, &action, &outcome, &sequence, &schema, &record, &evidence,
		&storedPayload, &storedResult, &proof); err != nil {
		return projectionDBError(err)
	}
	if app != command.AppID || instance != command.InstanceID || actor != command.ActorID || action != command.Action ||
		outcome != receipt.Outcome || sequence != receipt.Sequence || schema != command.SchemaVersion ||
		record != command.RecordVersion || proof != receipt.ProofID || !bytes.Equal(evidence, payload.EvidenceHash[:]) ||
		!bytes.Equal(storedPayload, encoded) || !bytes.Equal(storedResult, body) {
		return ErrConflict
	}
	return nil
}

func checkFence(ctx context.Context, tx pgx.Tx, command flowcommands.Command) error {
	var owner, state string
	var version, epoch int64
	if err := tx.QueryRow(ctx, `SELECT command_id::text,expected_record_version,fence_epoch,state
		FROM applications.record_command_fences WHERE app_id=$1 AND table_id=$2 AND record_id=$3`,
		command.AppID, command.TableID, command.RecordID).Scan(&owner, &version, &epoch, &state); err != nil {
		return projectionDBError(err)
	}
	if owner != command.CommandID || version != command.RecordVersion || epoch != command.FenceEpoch || state != "pending" {
		return ErrConflict
	}
	var acquired int64
	if err := tx.QueryRow(ctx, `SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)`,
		command.AppID, command.TableID, command.ViewID, command.RecordID, command.CommandID,
		command.SchemaVersion, command.RecordVersion).Scan(&acquired); err != nil {
		return err
	}
	if acquired != command.FenceEpoch {
		return ErrConflict
	}
	return nil
}

func initialInstance(instance instanceSnapshot, command flowcommands.Command) bool {
	return instance.state == "starting" && instance.sequence == 0 && instance.engine == nil && instance.initiator == command.ActorID
}

func validateSuccess(instance instanceSnapshot, command flowcommands.Command, result flowcommands.ExecutionResult) error {
	if command.Action == "start" {
		if !initialInstance(instance, command) || (result.State != "active" && result.State != "completed") {
			return ErrConflict
		}
		return nil
	}
	if instance.state != "active" || instance.engine == nil || *instance.engine != result.EngineProcessID {
		return ErrConflict
	}
	switch command.Action {
	case "agree":
		if result.State != "active" && result.State != "completed" {
			return ErrConflict
		}
	case "reject":
		if result.State != "rejected" {
			return ErrConflict
		}
	case "withdraw":
		if result.State != "withdrawn" || instance.initiator != command.ActorID {
			return ErrConflict
		}
	case "return":
		if result.State != "active" || result.Tasks[0].NodeID != command.TargetNodeID {
			return ErrConflict
		}
	default:
		return ErrInvalid
	}
	return nil
}

type storedTask struct {
	flowcommands.ExecutionTask
	app, instance string
	closed        *string
}

const taskColumns = `id::text,node_id::text,assignee_id::text,engine_task_id,activation_epoch,
	app_id::text,instance_id::text,closed_command_id::text`

func scanTask(row pgx.Row) (storedTask, error) {
	var task storedTask
	err := row.Scan(&task.ID, &task.NodeID, &task.AssigneeID, &task.EngineTaskID, &task.ActivationEpoch,
		&task.app, &task.instance, &task.closed)
	return task, err
}

func activeTasks(ctx context.Context, tx pgx.Tx, command flowcommands.Command) (map[string]storedTask, error) {
	rows, err := tx.Query(ctx, `SELECT `+taskColumns+` FROM applications.workflow_tasks
		WHERE app_id=$1 AND instance_id=$2 AND closed_command_id IS NULL ORDER BY id LIMIT 51`, command.AppID, command.InstanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	active := make(map[string]storedTask)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		active[task.ID] = task
		if len(active) > 50 {
			return nil, ErrConflict
		}
	}
	return active, rows.Err()
}

func validateAction(ctx context.Context, tx pgx.Tx, ledger flowcommands.Ledger, instance instanceSnapshot,
	command flowcommands.Command, result flowcommands.ExecutionResult, active map[string]storedTask) error {
	switch command.Action {
	case "start":
		if len(active) != 0 {
			return ErrConflict
		}
		return nil
	case "withdraw":
		if instance.initiator != command.ActorID {
			return ErrConflict
		}
		return nil
	case "agree", "reject":
		task, ok := active[command.TaskID]
		if !ok || task.AssigneeID != command.ActorID || task.ActivationEpoch != command.TaskEpoch {
			return ErrConflict
		}
		for _, remaining := range result.Tasks {
			if remaining.ID == command.TaskID {
				return ErrConflict
			}
		}
		return nil
	case "return":
		task, err := scanTask(tx.QueryRow(ctx, `SELECT `+taskColumns+` FROM applications.workflow_tasks WHERE id=$1`, command.TaskID))
		if err != nil {
			return projectionDBError(err)
		}
		if task.app != command.AppID || task.instance != command.InstanceID || task.AssigneeID != command.ActorID || task.ActivationEpoch != command.TaskEpoch {
			return ErrConflict
		}
		if task.closed != nil {
			closed, err := ledger.GetInTx(ctx, tx, *task.closed)
			if err != nil {
				return err
			}
			if closed.State != "success" || closed.Command.Action != "agree" || closed.Command.TaskID != task.ID ||
				closed.Command.ActorID != task.AssigneeID || closed.Command.InstanceID != command.InstanceID ||
				closed.Command.AppID != command.AppID || closed.Command.TaskEpoch != task.ActivationEpoch || command.TargetNodeID != task.NodeID {
				return ErrConflict
			}
		} else if _, ok := active[task.ID]; !ok {
			return ErrConflict
		}
		var visited bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_tasks
			WHERE app_id=$1 AND instance_id=$2 AND node_id=$3)`, command.AppID, command.InstanceID,
			command.TargetNodeID).Scan(&visited); err != nil {
			return err
		}
		if !visited {
			return ErrConflict
		}
		return nil
	default:
		return ErrInvalid
	}
}

type taskChanges struct {
	close []string
	open  []flowcommands.ExecutionTask
}

func planTasks(ctx context.Context, tx pgx.Tx, command flowcommands.Command, result flowcommands.ExecutionResult,
	graph flowgraph.Graph, active map[string]storedTask) (taskChanges, error) {
	var changes taskChanges
	approval := make(map[string]map[string]bool)
	for _, node := range graph.Nodes {
		if node.Kind == "approval" && node.Approval != nil {
			actors := make(map[string]bool, len(node.Approval.AssigneeIDs))
			for _, actor := range node.Approval.AssigneeIDs {
				actors[actor] = true
			}
			approval[node.ID] = actors
		}
	}
	ids := make([]string, 0, len(result.Tasks))
	remaining := make(map[string]bool, len(result.Tasks))
	for _, task := range result.Tasks {
		if !approval[task.NodeID][task.AssigneeID] {
			return changes, ErrConflict
		}
		ids = append(ids, task.ID)
		remaining[task.ID] = true
	}
	history := make(map[string]storedTask, len(ids))
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT `+taskColumns+` FROM applications.workflow_tasks WHERE id=ANY($1::uuid[])`, ids)
		if err != nil {
			return changes, err
		}
		defer rows.Close()
		for rows.Next() {
			task, err := scanTask(rows)
			if err != nil {
				return changes, err
			}
			history[task.ID] = task
		}
		if err = rows.Err(); err != nil {
			return changes, err
		}
	}
	kept := 0
	for _, task := range result.Tasks {
		old, found := history[task.ID]
		if !found {
			changes.open = append(changes.open, task)
			continue
		}
		if old.app != command.AppID || old.instance != command.InstanceID || old.closed != nil || old.ExecutionTask != task {
			return taskChanges{}, ErrConflict
		}
		if _, ok := active[task.ID]; !ok {
			return taskChanges{}, ErrConflict
		}
		kept++
	}
	// One activation either retains its original identities or consists entirely of new tasks.
	if len(changes.open) > 0 && kept > 0 || command.Action == "return" && kept > 0 {
		return taskChanges{}, ErrConflict
	}
	if len(changes.open) > 0 {
		var latest int64
		err := tx.QueryRow(ctx, `SELECT activation_epoch FROM applications.workflow_tasks
			WHERE app_id=$1 AND instance_id=$2 ORDER BY activation_epoch DESC,id LIMIT 1`,
			command.AppID, command.InstanceID).Scan(&latest)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return taskChanges{}, err
		}
		if latest >= 9007199254740991 || command.Action == "start" && latest != 0 {
			return taskChanges{}, ErrConflict
		}
		for _, task := range changes.open {
			if task.ActivationEpoch != latest+1 {
				return taskChanges{}, ErrConflict
			}
		}
	}
	for id := range active {
		if !remaining[id] {
			changes.close = append(changes.close, id)
		}
	}
	return changes, nil
}

func writeTasks(ctx context.Context, tx pgx.Tx, command flowcommands.Command, changes taskChanges) error {
	if len(changes.close) > 0 {
		tag, err := tx.Exec(ctx, `UPDATE applications.workflow_tasks SET closed_command_id=$3
			WHERE app_id=$1 AND instance_id=$2 AND id=ANY($4::uuid[]) AND closed_command_id IS NULL`,
			command.AppID, command.InstanceID, command.CommandID, changes.close)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != int64(len(changes.close)) {
			return ErrConflict
		}
	}
	for _, task := range changes.open {
		if _, err := tx.Exec(ctx, `INSERT INTO applications.workflow_tasks
			(id,app_id,instance_id,node_id,assignee_id,engine_task_id,activation_epoch,opened_command_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, task.ID, command.AppID, command.InstanceID,
			task.NodeID, task.AssigneeID, task.EngineTaskID, task.ActivationEpoch, command.CommandID); err != nil {
			return err
		}
	}
	return nil
}
