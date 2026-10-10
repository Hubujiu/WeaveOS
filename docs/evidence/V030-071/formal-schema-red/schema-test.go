package main

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"testing"
)

func TestRootPrivatePresetsSchemaClosedRowsReceiptsAndMinimumRole(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	var exists bool
	if e := f.owner.QueryRow(f.ctx, "SELECT to_regclass('applications.table_presets') IS NOT NULL").Scan(&exists); e != nil || !exists {
		t.Fatalf("approved private configuration relation absent: exists=%v err=%v", exists, e)
	}
	state := `{"name":"Schema preset","filter":null,"sort":null,"hiddenColumnIds":[],"columnOrder":[],"columnWidths":{}}`
	id := f.id(t)
	if _, e := f.runtime.Exec(f.ctx, `INSERT INTO applications.table_presets(id,owner_user_id,app_id,view_id,name,slot,definition_json,field_kinds) VALUES($1,$2,$3,$4,'Schema preset',1,$5,'{}')`, id, f.actor, f.app, f.view, state); e != nil {
		t.Fatal("minimal runtime insert denied", e)
	}
	mustState := func(t *testing.T, query string, want string, args ...any) {
		t.Helper()
		_, e := f.runtime.Exec(f.ctx, query, args...)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != want {
			t.Fatalf("expected SQLSTATE %s got %v", want, e)
		}
	}
	for _, column := range []string{"owner_user_id", "app_id", "view_id", "id", "slot", "created_at"} {
		t.Run("immutable_"+column, func(t *testing.T) {
			mustState(t, "UPDATE applications.table_presets SET "+column+"="+column+" WHERE id=$1", "42501", id)
		})
	}
	for _, bad := range []string{`{}`, `{"name":"different","filter":null,"sort":null,"hiddenColumnIds":[],"columnOrder":[],"columnWidths":{}}`, `{"name":"Schema preset","filter":null,"sort":null,"hiddenColumnIds":[],"columnOrder":[],"columnWidths":{},"ownerId":"x"}`} {
		t.Run("closed_definition", func(t *testing.T) {
			mustState(t, "UPDATE applications.table_presets SET definition_json=$2 WHERE id=$1", "23514", id, bad)
		})
	}
	for _, version := range []int64{0, 9007199254740992} {
		mustState(t, "UPDATE applications.table_presets SET version=$2 WHERE id=$1", "23514", id, version)
	}
	for _, kind := range []string{"preset.create", "preset.update", "preset.discard"} {
		t.Run(kind, func(t *testing.T) {
			op := f.id(t)
			status, location := 200, ""
			if kind == "preset.create" {
				status = 201
				location = "/api/v1/applications/" + f.app + "/forms/" + f.view + "/table-presets/" + id
			}
			if kind == "preset.discard" {
				status = 204
			}
			receipt, _ := json.Marshal(map[string]any{"operationId": op, "id": id, "version": 1})
			query := `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,$4,decode(repeat('ab',32),'hex'),$5,$6,$7)`
			if _, e := f.owner.Exec(f.ctx, query, f.actor, op, f.app, kind, receipt, status, location); e != nil {
				t.Fatal("closed minimum receipt rejected", e)
			}
			badOp := f.id(t)
			bad, _ := json.Marshal(map[string]any{"operationId": badOp, "id": id, "version": 1, "filter": "secret"})
			_, e := f.owner.Exec(f.ctx, query, f.actor, badOp, f.app, kind, bad, status, location)
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "23514" {
				t.Fatal("receipt leaked unbounded configuration", e)
			}
		})
	}
	var backup, reader, maintenance bool
	if e := f.owner.QueryRow(f.ctx, `SELECT has_table_privilege('auth_backup','applications.table_presets','SELECT'),has_table_privilege('auth_reader','applications.table_presets','SELECT'),has_table_privilege('auth_maintenance','applications.table_presets','SELECT')`).Scan(&backup, &reader, &maintenance); e != nil || !backup || reader || maintenance {
		t.Fatal("incorrect minimal read roles", backup, reader, maintenance, e)
	}
}

// The deployment script is intentionally reapplied by the composition suite.
// New configuration permissions must survive that idempotent reset without
// relying on an extra test-only GRANT after the real script.
func TestRootPrivatePresetsSchemaFormalRoleReapplication(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	roles, err := os.ReadFile("../../../../infra/runtime/roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	for pass := 0; pass < 2; pass++ {
		if _, err = tx.Exec(f.ctx, string(roles)); err != nil {
			t.Fatal(err)
		}
		var read, insert, remove, writeName, writeOwner, backup, reader, maintenance bool
		err = tx.QueryRow(f.ctx, `SELECT
   has_table_privilege('auth_app','applications.table_presets','SELECT'),
   has_table_privilege('auth_app','applications.table_presets','INSERT'),
   has_table_privilege('auth_app','applications.table_presets','DELETE'),
   has_column_privilege('auth_app','applications.table_presets','name','UPDATE'),
   has_column_privilege('auth_app','applications.table_presets','owner_user_id','UPDATE'),
   has_table_privilege('auth_backup','applications.table_presets','SELECT'),
   has_table_privilege('auth_reader','applications.table_presets','SELECT'),
   has_table_privilege('auth_maintenance','applications.table_presets','SELECT')`).Scan(&read, &insert, &remove, &writeName, &writeOwner, &backup, &reader, &maintenance)
		if err != nil || !read || !insert || !remove || !writeName || writeOwner || !backup || reader || maintenance {
			t.Fatalf("formal role reset lost minimum preset capabilities pass=%d read=%v insert=%v delete=%v name=%v owner=%v backup=%v reader=%v maintenance=%v err=%v", pass, read, insert, remove, writeName, writeOwner, backup, reader, maintenance, err)
		}
	}
}
