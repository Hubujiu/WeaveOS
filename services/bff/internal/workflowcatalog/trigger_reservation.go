package workflowcatalog

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type TriggerReservation struct {
	Instance Instance
	Ignored  bool
}

// ReserveTriggeredInTx serializes trigger reservations with the catalog's
// application, table and flow locks. The caller retains those locks until its
// transaction ends; this method neither commits nor dispatches an engine command.
func (catalog Catalog) ReserveTriggeredInTx(ctx context.Context, tx pgx.Tx, in ReserveInput) (TriggerReservation, error) {
	if tx == nil || !validIDs(in.AppID, in.FlowID, in.InstanceID, in.RecordID, in.ActorID) ||
		in.ExpectedRevision < 1 || in.ExpectedRevision > maxSafeInteger ||
		in.ExpectedSchemaVersion < 1 || in.ExpectedSchemaVersion > maxSafeInteger ||
		in.ExpectedRecordVersion < 1 || in.ExpectedRecordVersion > maxSafeInteger {
		return TriggerReservation{}, ErrInvalid
	}
	head, resources, err := lockFlow(ctx, tx, in.AppID, in.FlowID, true)
	if err != nil {
		return TriggerReservation{}, err
	}

	// Never let another live instance conceal reuse of an existing instance ID.
	// ReserveInTx remains authoritative for exact replay and identity conflicts.
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_instances WHERE id=$1)`,
		in.InstanceID).Scan(&exists); err != nil {
		return TriggerReservation{}, mapDBError(err)
	}
	if exists {
		instance, err := catalog.ReserveInTx(ctx, tx, in)
		if err != nil {
			return TriggerReservation{}, err
		}
		return TriggerReservation{Instance: instance}, nil
	}

	// A distinct trigger must satisfy the same current admission checks as a
	// new reservation, even when its eventual result would be ignored.
	if head.State == "closing" {
		return TriggerReservation{}, ErrClosing
	}
	if head.State != "enabled" {
		return TriggerReservation{}, ErrNotReady
	}
	if head.Revision != in.ExpectedRevision || resources.schemaVersion != in.ExpectedSchemaVersion {
		return TriggerReservation{}, ErrConflict
	}
	if head.CurrentVersion < 1 {
		return TriggerReservation{}, ErrNotReady
	}
	var deploymentID string
	err = tx.QueryRow(ctx, `SELECT deployment_id FROM applications.workflow_deployments
		WHERE app_id=$1 AND flow_id=$2 AND version=$3`, in.AppID, in.FlowID, head.CurrentVersion).Scan(&deploymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TriggerReservation{}, ErrNotReady
	}
	if err != nil {
		return TriggerReservation{}, mapDBError(err)
	}
	recordVersion, err := loadRecordVersion(ctx, tx, head.TableID, in.RecordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TriggerReservation{}, ErrMissing
	}
	if err != nil {
		return TriggerReservation{}, mapDBError(err)
	}
	if recordVersion != in.ExpectedRecordVersion {
		return TriggerReservation{}, ErrConflict
	}

	var live Instance
	err = tx.QueryRow(ctx, `SELECT id::text,flow_id::text,app_id::text,table_id::text,view_id::text,
		record_id::text,initiator_id::text,state,definition_version,sequence
		FROM applications.workflow_instances
		WHERE app_id=$1 AND flow_id=$2 AND record_id=$3 AND state IN ('starting','active')
		ORDER BY id LIMIT 1`, in.AppID, in.FlowID, in.RecordID).Scan(&live.ID, &live.FlowID,
		&live.AppID, &live.TableID, &live.ViewID, &live.RecordID, &live.InitiatorID,
		&live.State, &live.DefinitionVersion, &live.Sequence)
	if err == nil {
		return TriggerReservation{Instance: live, Ignored: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return TriggerReservation{}, mapDBError(err)
	}

	instance, err := catalog.ReserveInTx(ctx, tx, in)
	if err != nil {
		return TriggerReservation{}, err
	}
	return TriggerReservation{Instance: instance}, nil
}
