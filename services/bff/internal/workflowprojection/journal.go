package workflowprojection

import (
	"context"
	"errors"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

// replayJournal is a terminal-only path. Its initial unlocked read never grants
// execution: pending/missing commands return to the original lock/graph path.
// Terminal identity is revalidated under app -> table -> command locks.
func replayJournal(ctx context.Context, tx pgx.Tx, c flowcommands.Command, p flowcommands.ExecutionPayload, r flowcommands.Receipt, encoded, body []byte, result flowcommands.ExecutionResult) (Applied, bool, error) {
	var state string
	err := tx.QueryRow(ctx, "SELECT state FROM applications.workflow_commands WHERE command_id=$1", c.CommandID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && state == "pending" {
		return Applied{}, false, nil
	}
	if err != nil {
		return Applied{}, false, err
	}
	var id string
	for _, q := range []string{"SELECT id::text FROM applications.apps WHERE id=$1 FOR UPDATE", "SELECT id::text FROM applications.logical_tables WHERE app_id=$1 AND id=$2 FOR UPDATE"} {
		args := []any{c.AppID}
		if strings.Contains(q, "$2") {
			args = append(args, c.TableID)
		}
		if err = tx.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			return Applied{}, true, projectionDBError(err)
		}
	}
	if err = tx.QueryRow(ctx, "SELECT command_id::text FROM applications.workflow_commands WHERE command_id=$1 FOR UPDATE", c.CommandID).Scan(&id); err != nil {
		return Applied{}, true, projectionDBError(err)
	}
	ledger := flowcommands.Ledger{Namespace: "applications"}
	old, err := ledger.GetInTx(ctx, tx, c.CommandID)
	if err != nil {
		return Applied{}, true, err
	}
	if old.Command != c || old.State == "pending" || old.Receipt == nil {
		return Applied{}, true, ErrConflict
	}
	entry, err := ledger.ApplyInTx(ctx, tx, c, r, c.ExpectedSequence, func(context.Context, pgx.Tx, flowcommands.ApplyPlan) error { return ErrConflict })
	if err != nil {
		return Applied{}, true, err
	}
	if err = verifyEvent(ctx, tx, c, p, r, encoded, body); err != nil {
		return Applied{}, true, err
	}
	if err = verifyJournal(ctx, tx, c, r); err != nil {
		return Applied{}, true, err
	}
	// Existing close finalization is still allowed; a removed catalog must never
	// be recreated as a side effect of replaying already confirmed evidence.
	head, err := (workflowcatalog.Catalog{}).GetInTx(ctx, tx, c.AppID, c.FlowID)
	if err != nil && !errors.Is(err, workflowcatalog.ErrMissing) {
		return Applied{}, true, err
	}
	if err == nil && head.State == "closing" {
		if err = finalizeRequestedClose(ctx, tx, c); err != nil {
			return Applied{}, true, err
		}
	}
	return Applied{Entry: entry, Result: result, Duplicate: true}, true, nil
}

func verifyJournal(ctx context.Context, tx pgx.Tx, c flowcommands.Command, r flowcommands.Receipt) error {
	var table, view, record, flow, version, name, source string
	var definition, epoch int64
	var task, node, target *string
	if err := tx.QueryRow(ctx, `SELECT table_id::text,view_id::text,record_id::text,flow_id::text,version_id::text,definition_version,
 task_id::text,task_epoch,node_id::text,target_node_id::text,flow_name,flow_name_source
 FROM applications.workflow_execution_events WHERE command_id=$1`, c.CommandID).Scan(&table, &view, &record, &flow, &version, &definition, &task, &epoch, &node, &target, &name, &source); err != nil {
		return projectionDBError(err)
	}
	if table != c.TableID || view != c.ViewID || record != c.RecordID || flow != c.FlowID || version != c.VersionID || definition != c.DefinitionVersion || epoch != c.TaskEpoch || strings.TrimSpace(name) == "" || len([]rune(name)) > 100 || source != "captured" && source != "legacy_last_known" {
		return ErrConflict
	}
	if c.TaskID == "" {
		if task != nil || node != nil || epoch != 0 {
			return ErrConflict
		}
	} else {
		if task == nil || *task != c.TaskID || node == nil && r.Outcome != "no_effect" {
			return ErrConflict
		}
	}
	if c.TargetNodeID == "" {
		if target != nil {
			return ErrConflict
		}
	} else if target == nil || *target != c.TargetNodeID {
		return ErrConflict
	}
	return nil
}
