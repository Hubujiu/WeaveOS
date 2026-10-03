package appstructure

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
)

func (d RecordHistoryDML) Insert(c context.Context, tx pgx.Tx, table RecordTable, in RecordCreate, ids []string) (StoredRecordHeader, error) {
	if d.History == nil {
		return StoredRecordHeader{}, ErrUnavailable
	}
	if e := (RecordGate{AppID: table.AppID, TableID: table.TableID, ViewID: table.ViewID}).LockTable(c, tx, table.TableID, in.ExpectedSchemaVersion); e != nil {
		return StoredRecordHeader{}, e
	}
	fields, allIDs, e := allHistoryFields(c, tx, table.AppID, table.TableID)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	header, e := (nativeRecordDML{}).Insert(c, tx, table, in, ids)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	before := map[string]json.RawMessage{}
	for _, id := range allIDs {
		before[id] = json.RawMessage("null")
	}
	stored, e := historyValues(c, tx, table, in.ID, fields, allIDs)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	actual := map[string]any{}
	for id, v := range stored {
		actual[id] = v
	}
	if e = d.append(c, tx, table, header, in.OperationID, in.ActorID, 0, fields, before, actual, allIDs); e != nil {
		return StoredRecordHeader{}, e
	}
	return header, nil
}

func allHistoryFields(c context.Context, tx pgx.Tx, app, table string) (map[string]appfields.Field, []string, error) {
	rows, e := tx.Query(c, "SELECT definition FROM applications.fields WHERE app_id=$1 AND table_id=$2 AND NOT removed ORDER BY id", app, table)
	if e != nil {
		return nil, nil, ErrUnavailable
	}
	defer rows.Close()
	fields := map[string]appfields.Field{}
	ids := []string{}
	for rows.Next() {
		var raw json.RawMessage
		var f appfields.Field
		if rows.Scan(&raw) != nil || json.Unmarshal(raw, &f) != nil || !appfields.ValidID(f.ID) {
			return nil, nil, ErrUnavailable
		}
		fields[f.ID] = f
		ids = append(ids, f.ID)
	}
	if rows.Err() != nil {
		return nil, nil, ErrUnavailable
	}
	return fields, ids, nil
}
func (d RecordHistoryDML) UpdateCAS(c context.Context, tx pgx.Tx, table RecordTable, in RecordEdit, ids []string) (StoredRecordHeader, error) {
	if d.History == nil {
		return StoredRecordHeader{}, ErrUnavailable
	}
	old, e := (nativeRecordDML{}).LockHeader(c, tx, table, in.ID)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	if old.RecordVersion != in.ExpectedRecordVersion {
		return StoredRecordHeader{}, &Error{Code: "APPLICATION_RECORD_CONFLICT"}
	}
	fields, e := historyFields(c, tx, table.AppID, table.TableID, ids)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	before, e := historyValues(c, tx, table, in.ID, fields, ids)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	header, e := (nativeRecordDML{}).UpdateCAS(c, tx, table, in, ids)
	if e != nil {
		return StoredRecordHeader{}, e
	}
	if len(ids) > 0 {
		if e = d.append(c, tx, table, header, in.OperationID, in.ActorID, old.RecordVersion, fields, before, in.Changes, ids); e != nil {
			return StoredRecordHeader{}, e
		}
	}
	return header, nil
}
func historyFields(c context.Context, tx pgx.Tx, app, table string, ids []string) (map[string]appfields.Field, error) {
	if tx == nil {
		return nil, ErrUnavailable
	}
	unique := map[string]bool{}
	for _, id := range ids {
		if !appfields.ValidID(id) || unique[id] {
			return nil, applications.ErrResourceInvalid
		}
		unique[id] = true
	}
	rows, e := tx.Query(c, "SELECT definition FROM applications.fields WHERE app_id=$1 AND table_id=$2 AND id=ANY($3::uuid[]) AND NOT removed ORDER BY id", app, table, ids)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	fields := map[string]appfields.Field{}
	for rows.Next() {
		var raw json.RawMessage
		var f appfields.Field
		if rows.Scan(&raw) != nil || json.Unmarshal(raw, &f) != nil {
			return nil, ErrUnavailable
		}
		fields[f.ID] = f
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	if len(fields) != len(ids) {
		return nil, applications.ErrResourceInvalid
	}
	return fields, nil
}
func historyValues(c context.Context, tx pgx.Tx, table RecordTable, id string, fields map[string]appfields.Field, ids []string) (map[string]json.RawMessage, error) {
	if len(ids) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	expressions := []string{}
	args := []any{id}
	sorted := append([]string{}, ids...)
	sort.Strings(sorted)
	for _, id := range sorted {
		f := fields[id]
		col := columnPhysical(id)
		if f.Kind == "money" || f.Kind == "number" {
			col += "::text"
		}
		args = append(args, id)
		expressions = append(expressions, fmt.Sprintf("$%d::text,to_jsonb(%s)", len(args), col))
	}
	var raw json.RawMessage
	if e := tx.QueryRow(c, "SELECT jsonb_build_object("+strings.Join(expressions, ",")+") FROM "+tablePhysical(table.TableID)+" WHERE id=$1", args...).Scan(&raw); e != nil {
		return nil, recordError(e)
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return nil, ErrUnavailable
	}
	for id, v := range values {
		canonical, e := appfields.NormalizeValue(fields[id], v)
		if e != nil {
			return nil, ErrUnavailable
		}
		values[id] = canonical
	}
	return values, nil
}
func (d RecordHistoryDML) append(c context.Context, tx pgx.Tx, table RecordTable, header StoredRecordHeader, op, actor string, version int64, fields map[string]appfields.Field, before map[string]json.RawMessage, after map[string]any, ids []string) error {
	origin := d.Origin
	if origin == "" {
		origin = "ordinary"
	}
	m := HistoryMutation{AppID: table.AppID, TableID: table.TableID, ViewID: table.ViewID, RecordID: header.ID, ActorID: actor, OperationID: op, RecordVersionBefore: version, RecordVersionAfter: header.RecordVersion, Origin: origin, OpaqueTaskRef: d.OpaqueTaskRef, OccurredAt: header.UpdatedAt, Changes: []HistoryChange{}}
	for _, id := range ids {
		next, e := json.Marshal(after[id])
		if e != nil {
			return applications.ErrResourceInvalid
		}
		if !jsonEqual(before[id], json.RawMessage(next)) {
			m.Changes = append(m.Changes, HistoryChange{FieldID: id, FieldKind: fields[id].Kind, Before: before[id], After: next})
		}
	}
	if len(m.Changes) == 0 {
		return nil
	}
	return d.History.Append(c, tx, m)
}
