package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"strings"
	"testing"
)

func TestRootRoundReceiptClosedShapeAndHistoryRefusal(t *testing.T) {
	for _, kind := range []string{"resubmit", "review"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecordFixture(t)
			op := recordOperationID(t, f)
			body := map[string]any{"operationId": op, "flowId": recordOperationID(t, f), "previousInstanceId": recordOperationID(t, f), "instanceId": recordOperationID(t, f), "roundKind": kind, "roundNumber": 2, "status": "accepted"}
			raw, _ := json.Marshal(body)
			_, e := f.owner.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,$4,$5,$6::jsonb,202,$7)`, f.actor, op, f.app, "workflow.round."+kind, make([]byte, 32), raw, "/api/v1/application-operations/"+op)
			if e != nil {
				t.Fatalf("valid round acceptance not supported %v", e)
			}
			for _, bad := range []string{`result_json=result_json-'previousInstanceId'`, `result_json=result_json||'{"secret":"no"}'::jsonb`, `result_json=jsonb_set(result_json,'{roundNumber}','0')`, `result_json=jsonb_set(result_json,'{roundNumber}','1.5')`, `result_json=jsonb_set(result_json,'{roundNumber}','9007199254740992')`, `result_json=jsonb_set(result_json,'{roundKind}','"invalid"')`, `result_json=jsonb_set(result_json,'{status}','"completed"')`, `result_json=jsonb_set(result_json,'{instanceId}','"00000000-0000-0000-0000-000000000000"')`, `http_status=200`, `location='/bad'`} {
				_, e = f.owner.Exec(f.ctx, "UPDATE applications.operations SET "+bad+" WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op)
				var pg *pgconn.PgError
				if !errors.As(e, &pg) || pg.Code != "23514" {
					t.Fatalf("allowed malformed receipt %s %v", bad, e)
				}
			}
			sql, e := os.ReadFile("../../../../db/migrations/00031_workflow_round_operations.sql")
			if e != nil {
				t.Fatal(e)
			}
			parts := strings.Split(string(sql), "-- +goose Down")
			if len(parts) != 2 {
				t.Fatal("Down missing")
			}
			tx, e := f.owner.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			_, e = tx.Exec(f.ctx, parts[1])
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "55000" {
				t.Fatalf("Down lost round history %v", e)
			}
		})
	}
}
