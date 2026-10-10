package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestRootDeletionAuditExactShape(t *testing.T) {
	for _, kind := range []string{"valid", "old-generic-fallback", "extra-field", "missing-flow", "zero-flow", "invalid-app", "wrong-status", "wrong-action", "numeric-operation"} {
		t.Run(kind, func(t *testing.T) {
			p := deletionFixture(t)
			op := uuid(t, p.s.f.owner)
			body := map[string]any{"appId": p.s.f.app, "flowId": p.flow, "operationId": op, "status": "deleted", "action": "workflow.delete.completed"}
			switch kind {
			case "old-generic-fallback":
				body = map[string]any{"appId": p.s.f.app, "operationId": op, "structureVersion": 1, "schemaVersion": 1, "viewVersion": 1, "changeCount": 1}
			case "extra-field":
				body["bpmn"] = "must-not-leak"
			case "missing-flow":
				delete(body, "flowId")
			case "zero-flow":
				body["flowId"] = "00000000-0000-0000-0000-000000000000"
			case "invalid-app":
				body["appId"] = "invalid"
			case "wrong-status":
				body["status"] = "pending"
			case "wrong-action":
				body["action"] = "workflow.close"
			case "numeric-operation":
				body["operationId"] = 1
			}
			raw, _ := json.Marshal(body)
			_, e := p.s.f.runtime.Exec(context.Background(), `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary) VALUES('application_structure_changed','success',$1,'WORKFLOW_DELETION_COMPLETED',$2,'form',$3,$4)`, p.s.f.actor, op, p.s.view, raw)
			if kind == "valid" {
				if e != nil {
					t.Fatal("valid deletion completion audit is rejected", e)
				}
				return
			}
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "23514" {
				t.Fatalf("invalid deletion audit accepted (%s): %v", kind, e)
			}
		})
	}
}
