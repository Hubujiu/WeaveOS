package applications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/jackc/pgx/v5"
)

// AcceptedWorkflowContext contains only persisted instance identity and locked
// current resource facts. It grants no user access and is not an HTTP input.
type AcceptedWorkflowContext struct {
	RecordContext
	InstanceID, FlowID, RecordID, State, VersionID, CreatedBy string
	DefinitionVersion, Sequence, RecordVersion                int64
	AllowWithdraw                                             bool
	Graph                                                     json.RawMessage
}

// LockAcceptedWorkflowInTx is solely for recovery of an already durable intent.
// Caller must first hold personnel.lock_query_revisions and owns the transaction.
func LockAcceptedWorkflowInTx(ctx context.Context, tx pgx.Tx, instanceID string) (AcceptedWorkflowContext, error) {
	var out AcceptedWorkflowContext
	if tx == nil || ctx == nil {
		return out, ErrInvalid
	}
	if id, ok := canonicalID(instanceID); !ok || id != instanceID {
		return out, ErrInvalid
	}
	var appID, tableID, viewID, flowID, recordID, initiator string
	err := tx.QueryRow(ctx, `SELECT app_id::text,table_id::text,view_id::text,flow_id::text,record_id::text,initiator_id::text FROM applications.workflow_instances WHERE id=$1`, instanceID).Scan(&appID, &tableID, &viewID, &flowID, &recordID, &initiator)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrMissing
	}
	if err != nil {
		return out, err
	}
	app, err := loadApp(ctx, tx, appID, true)
	if err != nil {
		return out, err
	}
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM applications.logical_tables WHERE app_id=$1 AND id=$2 FOR UPDATE`, appID, tableID).Scan(&locked); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2 AND table_id=$3 AND view_id=$4 FOR UPDATE`, appID, flowID, tableID, viewID).Scan(&locked); err != nil {
		return out, err
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(tableID, "-", "")}.Sanitize()
	if err = tx.QueryRow(ctx, "SELECT created_by::text,record_version FROM "+relation+" WHERE id=$1", recordID).Scan(&out.CreatedBy, &out.RecordVersion); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT state,definition_version,sequence FROM applications.workflow_instances WHERE id=$1 AND app_id=$2 AND flow_id=$3 AND table_id=$4 AND view_id=$5 AND record_id=$6 AND initiator_id=$7`, instanceID, appID, flowID, tableID, viewID, recordID, initiator).Scan(&out.State, &out.DefinitionVersion, &out.Sequence); err != nil {
		return out, err
	}
	out.RecordContext, err = loadRecordContext(ctx, tx, apppolicy.TrustedActor{ID: initiator}, app, viewID)
	if err != nil {
		return out, err
	}
	if out.TableID != tableID || !out.SchemaReady {
		return out, ErrPolicyConflict
	}
	if err = tx.QueryRow(ctx, `SELECT v.version_id::text,v.graph_json,v.allow_withdraw FROM applications.workflow_versions v JOIN applications.workflow_deployments d ON d.app_id=v.app_id AND d.flow_id=v.flow_id AND d.version=v.version WHERE v.app_id=$1 AND v.flow_id=$2 AND v.version=$3`, appID, flowID, out.DefinitionVersion).Scan(&out.VersionID, &out.Graph, &out.AllowWithdraw); err != nil {
		return out, err
	}
	out.InstanceID = instanceID
	out.FlowID = flowID
	out.RecordID = recordID
	return out, nil
}

// WorkflowApproverContextInTx resolves current active actor facts in the durable
// instance's scope. The caller separately verifies membership in its fixed graph
// and row-specific read permission. This function is not a login alternative.
func WorkflowApproverContextInTx(ctx context.Context, tx pgx.Tx, instanceID, actorID string) (RecordContext, error) {
	var root bool
	if err := tx.QueryRow(ctx, "SELECT is_bootstrap_admin FROM auth.users WHERE id=$1 AND status='active'", actorID).Scan(&root); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecordContext{}, ErrDenied
		}
		return RecordContext{}, err
	}
	var appID, viewID string
	if err := tx.QueryRow(ctx, "SELECT app_id::text,view_id::text FROM applications.workflow_instances WHERE id=$1", instanceID).Scan(&appID, &viewID); err != nil {
		return RecordContext{}, err
	}
	app, err := loadApp(ctx, tx, appID, false)
	if err != nil {
		return RecordContext{}, err
	}
	return loadRecordContext(ctx, tx, apppolicy.TrustedActor{ID: actorID, BootstrapAdmin: root}, app, viewID)
}
