package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

type RecordGate struct{ AppID, TableID, ViewID string }

func (g RecordGate) LockTable(c context.Context, tx pgx.Tx, id string, expected int64) error {
	if tx == nil {
		return session.ErrUnavailable
	}
	if id != g.TableID || !appfields.ValidID(g.AppID) || !appfields.ValidID(g.ViewID) || !appfields.ValidID(id) {
		return applications.ErrResourceInvalid
	}
	var ready bool
	var version int64
	e := tx.QueryRow(c, `SELECT t.schema_ready,t.schema_version FROM applications.logical_tables t
 JOIN applications.form_views v ON v.app_id=t.app_id AND v.table_id=t.id AND v.id=$3
 JOIN applications.menu_resources r ON r.app_id=v.app_id AND r.resource_kind='form' AND r.resource_id=v.id
 WHERE t.app_id=$1 AND t.id=$2 FOR UPDATE OF t`, g.AppID, id, g.ViewID).Scan(&ready, &version)
	if errors.Is(e, pgx.ErrNoRows) {
		return applications.ErrMissing
	}
	if e != nil {
		return session.ErrUnavailable
	}
	if !ready {
		return &Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
	}
	if version != expected {
		return &Error{Code: "APPLICATION_SCHEMA_CONFLICT", Data: map[string]int64{"currentSchemaVersion": version}}
	}
	return nil
}

type RecordFence struct{ AppID, TableID string }

func (f RecordFence) Check(c context.Context, tx pgx.Tx, table, id string, version int64) error {
	if tx == nil {
		return session.ErrUnavailable
	}
	if table != f.TableID || !appfields.ValidID(f.AppID) || !appfields.ValidID(table) || !appfields.ValidID(id) || version < 1 || version > 9007199254740991 {
		return applications.ErrResourceInvalid
	}
	var actual string
	e := tx.QueryRow(c, "SELECT id::text FROM applications.logical_tables WHERE app_id=$1 AND id=$2 FOR UPDATE", f.AppID, table).Scan(&actual)
	if errors.Is(e, pgx.ErrNoRows) {
		return applications.ErrMissing
	}
	if e != nil {
		return session.ErrUnavailable
	}
	var pending bool
	e = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM applications.record_command_fences WHERE app_id=$1 AND table_id=$2 AND record_id=$3 AND state='pending')", f.AppID, table, id).Scan(&pending)
	if e != nil {
		return session.ErrUnavailable
	}
	if pending {
		return &Error{Code: "APPLICATION_RECORD_FENCED"}
	}
	return nil
}

type RecordMutationResult struct {
	OperationID   string    `json:"operationId"`
	ID            string    `json:"id"`
	RecordVersion int64     `json:"recordVersion"`
	SchemaVersion int64     `json:"schemaVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type RecordAudit struct {
	Context             applications.RecordContext
	OperationID         string
	BeforeRecordVersion int64
	Metadata            applications.Metadata
}

func (a RecordAudit) Append(c context.Context, tx pgx.Tx, result RecordMutationResult, kind string, ids []string) error {
	if tx == nil {
		return session.ErrUnavailable
	}
	if result.OperationID != a.OperationID || !appfields.ValidID(result.ID) || !appfields.ValidID(a.Context.Actor.ID) || a.Metadata.RequestID == "" || kind != "create" && kind != "edit" || result.RecordVersion < 1 || a.BeforeRecordVersion < 0 || result.RecordVersion < a.BeforeRecordVersion {
		return applications.ErrResourceInvalid
	}
	active := map[string]bool{}
	var fields []appfields.Field
	if json.Unmarshal(a.Context.Fields, &fields) != nil {
		return session.ErrUnavailable
	}
	for _, field := range fields {
		active[field.ID] = true
	}
	sorted := append([]string{}, ids...)
	sort.Strings(sorted)
	for i, id := range sorted {
		if !active[id] || i > 0 && sorted[i-1] == id {
			return applications.ErrResourceInvalid
		}
	}
	_, e := tx.Exec(c, `INSERT INTO applications.record_write_audit(actor_user_id,app_id,table_id,view_id,record_id,operation_id,before_record_version,after_record_version,changed_field_ids,origin,request_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'ordinary',$10)`, a.Context.Actor.ID, a.Context.App.ID, a.Context.TableID, a.Context.ViewID, result.ID, a.OperationID, a.BeforeRecordVersion, result.RecordVersion, sorted, a.Metadata.RequestID)
	if e != nil {
		return session.ErrUnavailable
	}
	return nil
}
