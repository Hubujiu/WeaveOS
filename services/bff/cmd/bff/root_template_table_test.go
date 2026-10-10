package main

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
	"time"
)

func rootTemplateEmptyTable(t *testing.T, f *rootTaskHTTPFixture) string {
	t.Helper()
	id := f.id(t)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.logical_tables(id,app_id,name,position) VALUES($1,$2,'Imported',0)", id, f.app); e != nil {
		t.Fatal(e)
	}
	return id
}
func rootTemplateField(t *testing.T, f *rootTaskHTTPFixture, kind, def, config string) appfields.Field {
	return appfields.Field{ID: f.id(t), Name: kind, Kind: kind, Default: json.RawMessage(def), Config: json.RawMessage(config)}
}
func rootTemplateTableLimits() appschema.Limits {
	return appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}
}
func TestRootTemplateTableInitializesRealRestrictedTypedColumns(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	id := rootTemplateEmptyTable(t, f)
	option := f.id(t)
	fields := []appfields.Field{
		rootTemplateField(t, f, "text", `"literal'); DROP SCHEMA auth CASCADE;--"`, `{"maxLength":null}`),
		rootTemplateField(t, f, "number", `"12.30"`, `{"precision":12,"scale":2}`),
		rootTemplateField(t, f, "boolean", `true`, `{}`),
		rootTemplateField(t, f, "date", `"2026-10-10"`, `{}`),
		rootTemplateField(t, f, "datetime", `"2026-10-10T01:02:03Z"`, `{"precision":"second"}`),
		rootTemplateField(t, f, "member", `"`+f.other+`"`, `{}`),
		rootTemplateField(t, f, "multi_select", `["`+option+`"]`, `{"options":[{"id":"`+option+`","label":"A"}]}`),
	}
	fields[0].Required = true
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if e = appstructure.InitializeTemplateTableInTx(f.ctx, tx, rootTemplatePrincipal(f), f.app, id, fields, rootTemplateTableLimits()); e != nil {
		t.Fatal("typed template table initialization unavailable", e)
	}
	var exists, ready bool
	var version, count int
	if e = tx.QueryRow(f.ctx, "SELECT to_regclass($1) IS NOT NULL,schema_ready,schema_version,(SELECT count(*) FROM applications.fields WHERE table_id=$2) FROM applications.logical_tables WHERE id=$2", "appdata.t_"+strings.ReplaceAll(id, "-", ""), id).Scan(&exists, &ready, &version, &count); e != nil || !exists || !ready || version != 1 || count != 7 {
		t.Fatal("typed metadata missing", exists, ready, version, count, e)
	}
	expected := []string{"text", "numeric(12,2)", "boolean", "date", "timestamp with time zone", "uuid", "uuid[]"}
	for i, field := range fields {
		var typ, defaultValue string
		var required bool
		e = tx.QueryRow(f.ctx, `SELECT format_type(a.atttypid,a.atttypmod),a.attnotnull,COALESCE(pg_get_expr(d.adbin,d.adrelid),'') FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=to_regclass($1) AND a.attname=$2`, "appdata.t_"+strings.ReplaceAll(id, "-", ""), "f_"+strings.ReplaceAll(field.ID, "-", "")).Scan(&typ, &required, &defaultValue)
		if e != nil || typ != expected[i] || defaultValue == "" || i == 0 && !required {
			t.Fatal("typed storage or defaults missing", i, typ, defaultValue, required, e)
		}
	}
	var owner, canCreate, canInsert bool
	if e = tx.QueryRow(f.ctx, `SELECT pg_get_userbyid(c.relowner)=current_user,has_schema_privilege(current_user,'appdata','CREATE'),has_table_privilege(current_user,c.oid,'INSERT') FROM pg_class c WHERE c.oid=to_regclass($1)`, "appdata.t_"+strings.ReplaceAll(id, "-", "")).Scan(&owner, &canCreate, &canInsert); e != nil || owner || canCreate || canInsert {
		t.Fatal("runtime gained broad DDL/DML", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	probe, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer probe.Rollback(f.ctx)
	tableSQL := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
	columns := []string{}
	for _, field := range fields {
		column := pgx.Identifier{"f_" + strings.ReplaceAll(field.ID, "-", "")}.Sanitize()
		if field.Kind == "datetime" {
			column = "(" + column + " AT TIME ZONE 'UTC')"
		}
		columns = append(columns, column+"::text")
	}
	var text, number, boolean, date, datetime, member, options string
	e = probe.QueryRow(f.ctx, "INSERT INTO "+tableSQL+"(id,created_by) VALUES($1,$2) RETURNING "+strings.Join(columns, ","), f.id(t), f.actor).Scan(&text, &number, &boolean, &date, &datetime, &member, &options)
	if e != nil || text != "literal'); DROP SCHEMA auth CASCADE;--" || number != "12.30" || boolean != "true" || date != "2026-10-10" || member != f.other || options != "{"+option+"}" || !strings.HasPrefix(datetime, "2026-10-10 01:02:03") {
		t.Fatal("actual typed defaults changed", text, number, boolean, date, datetime, member, options, e)
	}
	if e = probe.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}

	var rows int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM "+pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(id, "-", "")}.Sanitize()).Scan(&rows); e != nil || rows != 0 {
		t.Fatal("import table contains rows", rows, e)
	}
}
func TestRootTemplateTableOuterRollbackRemovesDDLAndMetadata(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	id := rootTemplateEmptyTable(t, f)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if e = appstructure.InitializeTemplateTableInTx(f.ctx, tx, rootTemplatePrincipal(f), f.app, id, []appfields.Field{}, rootTemplateTableLimits()); e != nil {
		t.Fatal("empty table initialization unavailable", e)
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	var exists, ready bool
	var version int
	if e = f.owner.QueryRow(f.ctx, "SELECT to_regclass($1) IS NOT NULL,schema_ready,schema_version FROM applications.logical_tables WHERE id=$2", "appdata.t_"+strings.ReplaceAll(id, "-", ""), id).Scan(&exists, &ready, &version); e != nil || exists || ready || version != 0 {
		t.Fatal("caller rollback left partial table", exists, ready, version, e)
	}
}
func TestRootTemplateTableRejectsWrongAuthorityScopeOrExistingTable(t *testing.T) {
	for _, mode := range []string{"non-owner", "stale-account", "foreign-app", "already-ready"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			id := rootTemplateEmptyTable(t, f)
			p := rootTemplatePrincipal(f)
			app := f.app
			switch mode {
			case "non-owner":
				p.UserID = f.other
				p.Record.UserID = f.other
			case "stale-account":
				p.Record.AuthVersion = "0"
			case "foreign-app":
				app = f.id(t)
			case "already-ready":
				id = f.table
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			if e = appstructure.InitializeTemplateTableInTx(f.ctx, tx, p, app, id, []appfields.Field{}, rootTemplateTableLimits()); e == nil {
				t.Fatal("unsafe initialization accepted")
			}
			if e = tx.Rollback(f.ctx); e != nil {
				t.Fatal(e)
			}
			var value string
			column := pgx.Identifier{"f_" + strings.ReplaceAll(f.field, "-", "")}.Sanitize()
			table := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
			if e = f.owner.QueryRow(f.ctx, "SELECT "+column+" FROM "+table+" WHERE id=$1", f.record).Scan(&value); e != nil || value != "original" {
				t.Fatal("existing data changed", value, e)
			}
		})
	}
}

func TestRootTemplateTableMidDDLAndMetadataFailureRollback(t *testing.T) {
	for _, phase := range []string{"ddl", "metadata"} {
		t.Run(phase, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			id := rootTemplateEmptyTable(t, f)
			fields := []appfields.Field{rootTemplateField(t, f, "text", `null`, `{"maxLength":null}`), rootTemplateField(t, f, "text", `null`, `{"maxLength":null}`)}
			if phase == "ddl" {
				fields[1].ID = f.field
			} else {
				suffix := strings.ReplaceAll(id, "-", "")
				fn := pgx.Identifier{"public", "template_fail_" + suffix}.Sanitize()
				trigger := pgx.Identifier{"template_fail_" + suffix}.Sanitize()
				sql := "CREATE FUNCTION " + fn + "() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id='" + id + "'::uuid THEN RAISE EXCEPTION 'isolated metadata failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER " + trigger + " BEFORE UPDATE ON applications.logical_tables FOR EACH ROW EXECUTE FUNCTION " + fn + "()"
				if _, e := f.owner.Exec(f.ctx, sql); e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() {
					if _, e := f.owner.Exec(f.ctx, "DROP TRIGGER "+trigger+" ON applications.logical_tables; DROP FUNCTION "+fn+"()"); e != nil {
						t.Error(e)
					}
				})
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			if e = appstructure.InitializeTemplateTableInTx(f.ctx, tx, rootTemplatePrincipal(f), f.app, id, fields, rootTemplateTableLimits()); e == nil {
				t.Fatal("injected mid-transaction failure ignored")
			}
			if e = tx.Rollback(f.ctx); e != nil {
				t.Fatal(e)
			}
			var exists, ready bool
			var version, count int
			if e = f.owner.QueryRow(f.ctx, "SELECT to_regclass($1) IS NOT NULL,schema_ready,schema_version,(SELECT count(*) FROM applications.fields WHERE table_id=$2) FROM applications.logical_tables WHERE id=$2", "appdata.t_"+strings.ReplaceAll(id, "-", ""), id).Scan(&exists, &ready, &version, &count); e != nil || exists || ready || version != 0 || count != 0 {
				t.Fatal("failure left partial DDL or metadata", exists, ready, version, count, e)
			}
		})
	}
}
