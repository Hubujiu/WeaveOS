package main

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
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
