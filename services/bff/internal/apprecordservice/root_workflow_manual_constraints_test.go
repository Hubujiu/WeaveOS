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

func rootManualReceiptRow(t *testing.T, f recordFixture) string {
	t.Helper()
	op := recordOperationID(t, f)
	raw, _ := json.Marshal(map[string]any{"operationId": op, "flowId": recordOperationID(t, f), "instanceId": recordOperationID(t, f), "status": "accepted", "ignored": false})
	_, e := f.owner.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'workflow.manual.start',$4,$5::jsonb,202,$6)`, f.actor, op, f.app, make([]byte, 32), raw, "/api/v1/application-operations/"+op)
	if e != nil {
		t.Fatalf("valid durable manual acceptance receipt not supported: %v", e)
	}
	return op
}
func TestRootManualReceiptClosedShapeAndAcceptedStatus(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	op := rootManualReceiptRow(t, f)
	for _, mutation := range []string{
		`result_json=jsonb_set(result_json,'{status}','"success"'::jsonb)`,
		`result_json=jsonb_set(result_json,'{status}','"ignored"'::jsonb)`,
		`result_json=jsonb_set(result_json,'{ignored}','true'::jsonb)`,
		`result_json=jsonb_set(result_json,'{ignored}','"false"'::jsonb)`,
		`result_json=result_json-'flowId'`,
		`result_json=result_json||'{"values":{"hidden":"no"}}'::jsonb`,
		`result_json=jsonb_set(result_json,'{instanceId}','"00000000-0000-0000-0000-000000000000"'::jsonb)`,
		`result_json=jsonb_set(result_json,'{flowId}','"INVALID"'::jsonb)`,
		`result_json=jsonb_set(result_json,'{operationId}','null'::jsonb)`,
		`http_status=200`, `location='/wrong'`,
	} {
		_, e := f.owner.Exec(f.ctx, "UPDATE applications.operations SET "+mutation+" WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op)
		var pgerr *pgconn.PgError
		if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("manual receipt allowed %s: %v", mutation, e)
		}
	}
	if _, e := f.owner.Exec(f.ctx, `UPDATE applications.operations SET result_json=result_json||'{"status":"ignored","ignored":true}'::jsonb WHERE actor_user_id=$1 AND operation_id=$2`, f.actor, op); e != nil {
		t.Fatalf("valid duplicate-ignore result rejected %v", e)
	}
}
func TestRootManualReceiptDownRefusesHistory(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	op := rootManualReceiptRow(t, f)
	raw, e := os.ReadFile("../../../../db/migrations/00025_workflow_manual_start_operations.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("exact Down missing")
	}
	tx, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	_, e = tx.Exec(f.ctx, parts[1])
	var pgerr *pgconn.PgError
	if !errors.As(e, &pgerr) || pgerr.Code != "55000" {
		t.Fatalf("Down did not refuse stored manual history %v", e)
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&n); e != nil || n != 1 {
		t.Fatalf("Down changed retained history %d %v", n, e)
	}
}
