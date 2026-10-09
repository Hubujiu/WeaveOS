package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

var ErrWorkflowRecordReadOnly = errors.New("record is read-only while workflow is in flight")

func retainRecordTriggers(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, stored apprecords.MutationResult, event string) error {
	_, err := (workflowcatalog.Catalog{}).ReserveRecordTriggersInTx(ctx, tx, workflowcatalog.RecordTriggerInput{
		AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, RecordID: stored.ID, ActorID: facts.Actor.ID, Event: event,
		ExpectedSchemaVersion: stored.SchemaVersion, ExpectedRecordVersion: stored.RecordVersion,
	})
	return err
}

// Called by the DML adapter only after Writer has locked and authorized the row
// and checked its fence/CAS. PostgreSQL's actual column types define equality;
// no client JSON spelling or record-version increment stands in for a change.
func recordValuesActuallyChange(ctx context.Context, tx pgx.Tx, table apprecords.Table, in apprecords.Edit, ids []string) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}
	if !appfields.ValidID(table.TableID) || !appfields.ValidID(in.ID) {
		return false, apprecords.ErrInvalid
	}
	physical := make(map[string]any, len(ids))
	comparisons := make([]string, 0, len(ids))
	for _, id := range ids {
		if !appfields.ValidID(id) {
			return false, apprecords.ErrInvalid
		}
		col := "f_" + strings.ReplaceAll(id, "-", "")
		physical[col] = in.Changes[id]
		quoted := pgx.Identifier{col}.Sanitize()
		comparisons = append(comparisons, "r."+quoted+" IS DISTINCT FROM proposed."+quoted)
	}
	raw, err := json.Marshal(physical)
	if err != nil {
		return false, apprecords.ErrInvalid
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(table.TableID, "-", "")}.Sanitize()
	var changed bool
	err = tx.QueryRow(ctx, "SELECT ("+strings.Join(comparisons, " OR ")+") FROM "+relation+" r CROSS JOIN LATERAL jsonb_populate_record(NULL::"+relation+",$2::jsonb) proposed WHERE r.id=$1::uuid", in.ID, raw).Scan(&changed)
	return changed, err
}
