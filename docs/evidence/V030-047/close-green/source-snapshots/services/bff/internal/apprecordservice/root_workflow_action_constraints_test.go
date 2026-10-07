package apprecordservice

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"strings"
	"testing"
)

func TestRootWorkflowActionAcceptedReceiptIsPendingMinimalAnd202Only(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	rootAction(t, f, req)
	for _, mutation := range []string{
		"result_json=jsonb_set(result_json,'{status}','\"success\"'::jsonb)",
		"result_json=result_json||'{\"values\":{\"secret\":\"forbidden\"}}'::jsonb",
		"result_json=jsonb_set(result_json,'{commandId}','\"00000000-0000-0000-0000-000000000000\"'::jsonb)",
		"http_status=200",
		"location='/wrong-operation'",
	} {
		_, e := f.owner.Exec(f.ctx, "UPDATE applications.operations SET "+mutation+" WHERE actor_user_id=$1 AND operation_id=$2", f.actor, req.OperationID)
		var pgerr *pgconn.PgError
		if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("accepted receipt CHECK allowed %s: %v", mutation, e)
		}
	}
	op := recordOperationID(t, f.recordFixture)
	if _, e := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "ordinary"}}, applications.Metadata{RequestID: "ordinary-not-async"}); e != nil {
		t.Fatal(e)
	}
	_, e := f.owner.Exec(f.ctx, "UPDATE applications.operations SET http_status=202 WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op)
	var pgerr *pgconn.PgError
	if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
		t.Fatalf("ordinary operation gained unapproved async shape: %v", e)
	}
}
func TestRootWorkflowActionMigrationDownNeverDeletesAcceptedHistory(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	result := rootAction(t, f, req)
	raw, e := os.ReadFile("../../../../db/migrations/00021_workflow_task_operations.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("exact migration Down missing")
	}
	tx, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	_, e = tx.Exec(f.ctx, parts[1])
	var pgerr *pgconn.PgError
	if !errors.As(e, &pgerr) || pgerr.Code != "55000" {
		t.Fatalf("Down did not refuse existing workflow operation: %v", e)
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	after, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
	if e != nil || after != result {
		t.Fatal("failed rollback damaged original accepted history", e)
	}
	rootActionCount(t, f, 1, 1, 1)
}
