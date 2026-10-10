package main

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"strings"
	"testing"
)

// Independent closed receipt contract: data design V030-070, frozen before SQL.
func TestRootTemplateImportSchemaAcceptsExactMinimalReceipt(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	op := f.id(t)
	data := map[string]any{"operationId": op, "appId": f.app, "policyRevision": 1, "structureVersion": 1, "status": "imported"}
	raw, _ := json.Marshal(data)
	_, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'application.template.import',decode(repeat('12',32),'hex'),$4,201,$5)`, f.actor, op, f.app, raw, "/api/v1/applications/"+f.app)
	if e != nil {
		t.Fatal("closed template import receipt unavailable", e)
	}
}
func TestRootTemplateImportSchemaRejectsNonminimalOrMismatchedReceipt(t *testing.T) {
	for _, mode := range []string{"extra-data", "missing-app", "wrong-operation", "wrong-app", "wrong-status", "wrong-policy", "wrong-structure", "string-policy", "null-structure", "wrong-http", "wrong-location"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			op := f.id(t)
			data := map[string]any{"operationId": op, "appId": f.app, "policyRevision": 1, "structureVersion": 1, "status": "imported"}
			status := 201
			location := "/api/v1/applications/" + f.app
			switch mode {
			case "extra-data":
				data["records"] = []any{}
			case "missing-app":
				delete(data, "appId")
			case "wrong-operation":
				data["operationId"] = f.id(t)
			case "wrong-app":
				data["appId"] = f.id(t)
			case "wrong-status":
				data["status"] = "active"
			case "wrong-policy":
				data["policyRevision"] = 2
			case "wrong-structure":
				data["structureVersion"] = 0
			case "string-policy":
				data["policyRevision"] = "1"
			case "null-structure":
				data["structureVersion"] = nil
			case "wrong-http":
				status = 200
			case "wrong-location":
				location = "/api/v1/application-operations/" + op
			}
			raw, _ := json.Marshal(data)
			_, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'application.template.import',decode(repeat('12',32),'hex'),$4,$5,$6)`, f.actor, op, f.app, raw, status, location)
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "23514" || pg.ConstraintName != "ck_template_import_result" {
				t.Fatal("invalid import receipt accepted or unexpected failure", e)
			}
		})
	}
}

func rootTemplateImportMigration(t *testing.T) (string, string) {
	t.Helper()
	raw, e := os.ReadFile("../../../../db/migrations/00032_application_template_import.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("expected one Up/Down boundary")
	}
	return parts[0], parts[1]
}
func TestRootTemplateImportSchemaNonemptyDownProtectsReceipt(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	_, down := rootTemplateImportMigration(t)
	tx, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	op := f.id(t)
	raw, _ := json.Marshal(map[string]any{"operationId": op, "appId": f.app, "policyRevision": 1, "structureVersion": 1, "status": "imported"})
	if _, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'application.template.import',decode(repeat('12',32),'hex'),$4,201,$5)`, f.actor, op, f.app, raw, "/api/v1/applications/"+f.app); e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(f.ctx, down)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatal("nonempty Down failed to refuse durable receipt", e)
	}
}
func TestRootTemplateImportSchemaEmptyDownUpRestoresExactConstraints(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	up, down := rootTemplateImportMigration(t)
	tx, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	var before, after string
	q := `SELECT string_agg(conname||':'||pg_get_constraintdef(oid),'|' ORDER BY conname) FROM pg_constraint WHERE conrelid='applications.operations'::regclass`
	if e = tx.QueryRow(f.ctx, q).Scan(&before); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(f.ctx, down); e != nil {
		t.Fatal(e)
	}
	var exists bool
	if e = tx.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='applications.operations'::regclass AND conname='ck_template_import_result')`).Scan(&exists); e != nil || exists {
		t.Fatal("empty Down retained new constraint", exists, e)
	}
	if _, e = tx.Exec(f.ctx, up); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(f.ctx, q).Scan(&after); e != nil || before != after {
		t.Fatal("Down/Up changed pre-existing or new constraints", e)
	}
}
