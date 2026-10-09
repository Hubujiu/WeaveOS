package workflowcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/jackc/pgx/v5"
)

// RecordTriggerInput contains trusted facts from an already-authorized write.
// ViewID identifies the write view; rules are selected across its entire table.
type RecordTriggerInput struct {
	AppID, TableID, ViewID, RecordID, ActorID, Event string
	ExpectedSchemaVersion, ExpectedRecordVersion     int64
}
type RecordTriggerCounts struct{ Reserved, Ignored int }

// ReserveRecordTriggersInTx atomically retains matching starts with the caller's
// record write. It never commits, calls an engine, or grants resource access.
func (catalog Catalog) ReserveRecordTriggersInTx(ctx context.Context, tx pgx.Tx, in RecordTriggerInput) (RecordTriggerCounts, error) {
	counts := RecordTriggerCounts{}
	if tx == nil || !validIDs(in.AppID, in.TableID, in.ViewID, in.RecordID, in.ActorID) ||
		in.Event != "record.created" && in.Event != "record.updated" ||
		in.ExpectedSchemaVersion < 1 || in.ExpectedSchemaVersion > maxSafeInteger || in.ExpectedRecordVersion < 1 || in.ExpectedRecordVersion > maxSafeInteger {
		return counts, ErrInvalid
	}
	resources, err := lockResources(ctx, tx, in.AppID, in.TableID, in.ViewID, true)
	if err != nil {
		return counts, err
	}
	if resources.schemaVersion != in.ExpectedSchemaVersion {
		return counts, ErrConflict
	}
	actual, err := loadRecordVersion(ctx, tx, in.TableID, in.RecordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return counts, ErrMissing
	}
	if err != nil {
		return counts, mapDBError(err)
	}
	if actual != in.ExpectedRecordVersion {
		return counts, ErrConflict
	}
	eventJSON, _ := json.Marshal([]map[string]string{{"event": in.Event}})
	cursor := "00000000-0000-0000-0000-000000000000"
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(in.TableID, "-", "")}.Sanitize()
	// App/table locks serialize configuration changes. Keyset iteration keeps
	// result memory constant and acquires flow locks in the same stable order.
	for {
		var flowID string
		err = tx.QueryRow(ctx, `SELECT d.id::text FROM applications.workflow_definitions d
   JOIN applications.workflow_versions v ON v.app_id=d.app_id AND v.flow_id=d.id AND v.version=d.current_version
   WHERE d.app_id=$1 AND d.table_id=$2 AND d.state='enabled' AND d.id>$3::uuid AND v.triggers_json @> $4::jsonb
   ORDER BY d.id LIMIT 1`, in.AppID, in.TableID, cursor, eventJSON).Scan(&flowID)
		if errors.Is(err, pgx.ErrNoRows) {
			return counts, nil
		}
		if err != nil {
			return counts, mapDBError(err)
		}
		cursor = flowID
		head, err := headForUpdate(ctx, tx, in.AppID, flowID)
		if err != nil {
			return counts, mapDBError(err)
		}
		triggers, err := catalog.TriggersForVersionInTx(ctx, tx, in.AppID, flowID, head.CurrentVersion)
		if err != nil {
			return counts, err
		}
		triggers, err = NormalizeTriggers(triggers, resources.fields)
		if err != nil {
			return counts, ErrNotReady
		}
		for _, trigger := range triggers {
			if trigger.Event != in.Event {
				continue
			}
			plan, err := appquery.Compile(trigger.Condition, nil, resources.fields, 2)
			if err != nil {
				return counts, ErrNotReady
			}
			args := append([]any{in.RecordID}, plan.Arguments...)
			var matches bool
			if err = tx.QueryRow(ctx, "SELECT COALESCE(("+plan.Predicate+"),false) FROM "+relation+" r WHERE r.id=$1::uuid", args...).Scan(&matches); err != nil {
				return counts, mapDBError(err)
			}
			if !matches {
				break
			}
			id, err := randomID(ctx, tx)
			if err != nil {
				return counts, err
			}
			reservation, err := catalog.ReserveTriggeredInTx(ctx, tx, ReserveInput{AppID: in.AppID, FlowID: head.FlowID, InstanceID: id, RecordID: in.RecordID, ActorID: in.ActorID, ExpectedRevision: head.Revision, ExpectedSchemaVersion: in.ExpectedSchemaVersion, ExpectedRecordVersion: in.ExpectedRecordVersion})
			if err != nil {
				return counts, err
			}
			if reservation.Ignored {
				counts.Ignored++
			} else {
				counts.Reserved++
			}
			break
		}
	}
}
