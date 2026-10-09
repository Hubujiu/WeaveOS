package apprecordservice

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRootWorkflowInboxPersonalIndexReady(t *testing.T) {
	f := newRecordFixture(t)
	var def string
	var valid, ready bool
	e := f.owner.QueryRow(f.ctx, `SELECT pg_get_indexdef(i.indexrelid),i.indisvalid,i.indisready FROM pg_index i JOIN pg_class ix ON ix.oid=i.indexrelid WHERE ix.relnamespace='applications'::regnamespace AND ix.relname='ix_workflow_tasks_personal_order'`).Scan(&def, &valid, &ready)
	if e != nil {
		t.Fatalf("personal cross-application candidate access path missing: %v", e)
	}
	if !valid || !ready || !strings.Contains(def, "USING btree (assignee_id, created_at DESC, id DESC)") || !strings.Contains(def, "WHERE (closed_command_id IS NULL)") {
		t.Fatal("wrong index scope/order/predicate", def)
	}
}

func TestRootWorkflowInboxIndexRollbackRetainsTasks(t *testing.T) {
	f := rootTaskSetup(t, false)
	raw, e := os.ReadFile("../../../../db/migrations/00026_workflow_personal_inbox_index.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("expected explicit Down")
	}
	tx, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(f.ctx, parts[1]); e != nil {
		t.Fatal(e)
	}
	var taskCount int
	if e = tx.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_tasks WHERE app_id=$1 AND id=$2 AND closed_command_id IS NULL", f.app, f.task.ID).Scan(&taskCount); e != nil || taskCount != 1 {
		t.Fatal("Down changed business task", e)
	}
	var gone bool
	if e = tx.QueryRow(f.ctx, "SELECT to_regclass('applications.ix_workflow_tasks_personal_order') IS NULL").Scan(&gone); e != nil || !gone {
		t.Fatal("Down did not remove only its index", e)
	}
	if _, e = tx.Exec(f.ctx, strings.TrimPrefix(parts[0], "-- +goose Up")); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_tasks WHERE app_id=$1 AND id=$2 AND closed_command_id IS NULL", f.app, f.task.ID).Scan(&taskCount); e != nil || taskCount != 1 {
		t.Fatal("Up changed business task", e)
	}
}
