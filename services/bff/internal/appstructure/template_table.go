package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// InitializeTemplateTableInTx only initializes a new, empty logical table.
// The caller owns the transaction and has already locked current identity/source
// dependencies before its app/table gates. This port never commits or writes
// business rows, views, audit, operations, or engine commands.
func InitializeTemplateTableInTx(ctx context.Context, tx pgx.Tx, p session.Principal, appID, tableID string, fields []appfields.Field, limits appschema.Limits) error {
	if tx == nil || fields == nil || !appfields.ValidID(appID) || !appfields.ValidID(tableID) || appID == "00000000-0000-0000-0000-000000000000" || tableID == "00000000-0000-0000-0000-000000000000" {
		return applications.ErrInvalid
	}
	normalized, e := appfields.NormalizeFields(fields)
	if e != nil {
		return e
	}
	physical, e := physicalFields(normalized)
	if e != nil {
		return e
	}
	ddl := &adapters{p: p, app: appID, before: Definition{AppID: appID, Table: Table{ID: tableID}}}
	metadata := &templateTableMetadata{app: appID, table: tableID, fields: normalized, guard: ddl}
	executor := appschema.Executor{Namespace: "appdata", Guard: metadata, Metadata: metadata, DDL: ddl, Limits: limits}
	_, e = executor.ApplyInTx(ctx, tx, appschema.Request{TableID: tableID, ExpectedRevision: "0", Fields: physical})
	return e
}

type templateTableMetadata struct {
	app, table string
	fields     []appfields.Field
	guard      *adapters
}

func (m *templateTableMetadata) Check(ctx context.Context, tx pgx.Tx, id string) error {
	if id != m.table {
		return applications.ErrResourceInvalid
	}
	if e := m.guard.Check(ctx, tx, id); e != nil {
		return e
	}
	var registered bool
	e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='app.'||$1::text||'.access' AND app_id=$1::text AND category='application' AND enabled)", m.app).Scan(&registered)
	if e != nil {
		return e
	}
	if !registered {
		return applications.ErrDenied
	}
	return nil
}
func (m *templateTableMetadata) Lock(ctx context.Context, tx pgx.Tx, id string) (appschema.Snapshot, error) {
	if id != m.table {
		return appschema.Snapshot{}, applications.ErrResourceInvalid
	}
	table, fields, e := loadTable(ctx, tx, m.app, id, true)
	if e != nil {
		return appschema.Snapshot{}, e
	}
	if table.SchemaReady || table.SchemaVersion != 0 || len(fields) != 0 {
		return appschema.Snapshot{}, applications.ErrResourceInvalid
	}
	var existing bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM applications.fields WHERE app_id=$1 AND table_id=$2)", m.app, id).Scan(&existing); e != nil {
		return appschema.Snapshot{}, e
	}
	if existing {
		return appschema.Snapshot{}, applications.ErrResourceInvalid
	}
	return appschema.Snapshot{Exists: false, Revision: "0", Fields: []appschema.Field{}}, nil
}
func (m *templateTableMetadata) Store(ctx context.Context, tx pgx.Tx, id string, before appschema.Snapshot, _ []appschema.Field) (string, error) {
	if id != m.table || before.Exists || before.Revision != "0" {
		return "", applications.ErrResourceInvalid
	}
	for _, f := range m.fields {
		raw, e := json.Marshal(f)
		if e != nil {
			return "", e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO applications.fields(id,app_id,table_id,definition) VALUES($1,$2,$3,$4)", f.ID, m.app, id, raw); e != nil {
			return "", e
		}
	}
	raw, e := json.Marshal(m.fields)
	if e != nil {
		return "", e
	}
	tag, e := tx.Exec(ctx, "UPDATE applications.logical_tables SET fields_json=$3,schema_version=1,schema_ready=true WHERE app_id=$1 AND id=$2 AND schema_version=0 AND NOT schema_ready", m.app, id, raw)
	if e != nil {
		return "", e
	}
	if tag.RowsAffected() != 1 {
		return "", applications.ErrResourceInvalid
	}
	return "1", nil
}
