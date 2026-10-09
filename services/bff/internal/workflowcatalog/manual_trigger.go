package workflowcatalog

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/jackc/pgx/v5"
	"strings"
)

type ManualReserveInput struct {
	ReserveInput
	TableID string
}

// ReserveManualInTx is called only after the service verifies the current
// session, actual row read access and immutable record creator. It never commits
// or dispatches RPC. All configuration/history checks precede duplicate ignore.
func (catalog Catalog) ReserveManualInTx(ctx context.Context, tx pgx.Tx, in ManualReserveInput) (TriggerReservation, error) {
	if tx == nil || !validIDs(in.AppID, in.FlowID, in.TableID, in.InstanceID, in.RecordID, in.ActorID) || in.ExpectedRevision < 1 || in.ExpectedRevision > maxSafeInteger || in.ExpectedSchemaVersion < 1 || in.ExpectedSchemaVersion > maxSafeInteger || in.ExpectedRecordVersion < 1 || in.ExpectedRecordVersion > maxSafeInteger {
		return TriggerReservation{}, ErrInvalid
	}
	// The caller holds its application/table gate. Reject another table before
	// attempting its locks; an arbitrary flow ID cannot reverse table lock order.
	current, err := headRead(ctx, tx, in.AppID, in.FlowID)
	if err != nil {
		return TriggerReservation{}, mapDBError(err)
	}
	if current.TableID != in.TableID {
		return TriggerReservation{}, ErrMissing
	}
	head, resources, err := lockFlow(ctx, tx, in.AppID, in.FlowID, true)
	if err != nil {
		return TriggerReservation{}, err
	}
	if head.State == "closing" {
		return TriggerReservation{}, ErrClosing
	}
	if head.State != "enabled" || head.CurrentVersion < 1 {
		return TriggerReservation{}, ErrNotReady
	}
	if head.Revision != in.ExpectedRevision || resources.schemaVersion != in.ExpectedSchemaVersion {
		return TriggerReservation{}, ErrConflict
	}
	actual, err := loadRecordVersion(ctx, tx, in.TableID, in.RecordID)
	if err != nil {
		return TriggerReservation{}, mapDBError(err)
	}
	if actual != in.ExpectedRecordVersion {
		return TriggerReservation{}, ErrConflict
	}
	triggers, err := catalog.TriggersForVersionInTx(ctx, tx, in.AppID, in.FlowID, head.CurrentVersion)
	if err != nil {
		return TriggerReservation{}, err
	}
	triggers, err = NormalizeTriggers(triggers, resources.fields)
	if err != nil {
		return TriggerReservation{}, ErrNotReady
	}
	matched := false
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(in.TableID, "-", "")}.Sanitize()
	for _, trigger := range triggers {
		if trigger.Event != "manual" {
			continue
		}
		plan, e := appquery.Compile(trigger.Condition, nil, resources.fields, 2)
		if e != nil {
			return TriggerReservation{}, ErrNotReady
		}
		args := append([]any{in.RecordID}, plan.Arguments...)
		if e = tx.QueryRow(ctx, "SELECT COALESCE(("+plan.Predicate+"),false) FROM "+relation+" r WHERE r.id=$1::uuid", args...).Scan(&matched); e != nil {
			return TriggerReservation{}, mapDBError(e)
		}
		break
	}
	if !matched {
		return TriggerReservation{}, ErrNotReady
	}
	var history, live bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3),EXISTS(SELECT 1 FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3 AND state IN ('starting','active'))`, in.AppID, in.FlowID, in.RecordID).Scan(&history, &live)
	if err != nil {
		return TriggerReservation{}, mapDBError(err)
	}
	if history && !live {
		return TriggerReservation{}, ErrConflict
	}
	return catalog.ReserveTriggeredInTx(ctx, tx, in.ReserveInput)
}
