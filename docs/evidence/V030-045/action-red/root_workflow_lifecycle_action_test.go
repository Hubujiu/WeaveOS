package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"
)

func rootLifecycleActionReq(t *testing.T, f rootTaskFixture, action string) WorkflowLifecycleActionRequest {
	t.Helper()
	q := rootLifecycleRequest(f)
	p, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	r := WorkflowLifecycleActionRequest{WorkflowLifecycleRequest: q, OperationID: recordOperationID(t, f.recordFixture), Action: action, BasisToken: p.BasisToken}
	if action == "return" {
		r.TargetNodeID = f.task.NodeID
	}
	return r
}
func rootLifecycleAccept(t *testing.T, f rootTaskFixture, r WorkflowLifecycleActionRequest) WorkflowOperationResult {
	t.Helper()
	v, e := f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, r, applications.Metadata{RequestID: "root-lifecycle"})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestRootLifecycleActionAcceptedIdentityAndNoEarlyTransition(t *testing.T) {
	for _, action := range []string{"withdraw", "return"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			q := rootLifecycleActionReq(t, f, action)
			r := rootLifecycleAccept(t, f, q)
			if r.Status != "pending" || r.CommandID == "" || r.InstanceID != f.instance.ID || r.InstanceState != "" {
				t.Fatalf("premature result %+v", r)
			}
			c, p := rootActionRead(t, f, r)
			if c.Action != action || c.ActorID != f.actor || c.ExpectedSequence != 1 || c.RecordVersion != 1 || c.SchemaVersion != 1 || c.FenceEpoch < 1 || p.Start != nil {
				t.Fatalf("wrong command %+v", c)
			}
			if action == "withdraw" && (c.TaskID != "" || c.TaskEpoch != 0 || c.TargetNodeID != "") {
				t.Fatal("withdraw invented task")
			}
			if action == "return" && (c.TaskID != f.task.ID || c.TaskEpoch != 1 || c.TargetNodeID != f.task.NodeID) {
				t.Fatal("return lost server task binding")
			}
			f.state(t, "active", 1, 1, 1, 1)
			status, e := f.service.WorkflowOperation(f.ctx, f.principal, q.OperationID)
			if e != nil || status != r {
				t.Fatalf("status %+v %v", status, e)
			}
		})
	}
}
func TestRootLifecycleActionReplayAndChangedPayload(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootLifecycleActionReq(t, f, "withdraw")
	a := rootLifecycleAccept(t, f, q)
	b := rootLifecycleAccept(t, f, q)
	if a != b {
		t.Fatal("replay created new command")
	}
	q.Action = "return"
	q.TargetNodeID = f.task.NodeID
	_, e := f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, q, applications.Metadata{RequestID: "changed"})
	if !errors.Is(e, applications.ErrOperationConflict) {
		t.Fatalf("changed payload %v", e)
	}
}
func TestRootLifecycleActionRejectsUnvisitedTargetWithoutWrites(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootLifecycleActionReq(t, f, "return")
	q.TargetNodeID = recordOperationID(t, f.recordFixture)
	r, e := f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, q, applications.Metadata{RequestID: "invalid-target"})
	if !errors.Is(e, ErrWorkflowTaskChanged) || r.CommandID != "" {
		t.Fatalf("unvisited target %+v %v", r, e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 0 {
		t.Fatalf("invalid target wrote operation %d %v", n, e)
	}
}
func TestRootLifecycleActionLatestBasisAndRevokedPermission(t *testing.T) {
	for _, mode := range []string{"version", "revoke"} {
		t.Run(mode, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			q := rootLifecycleActionReq(t, f, "withdraw")
			if mode == "version" {
				if _, e := f.owner.Exec(f.ctx, "UPDATE appdata.t_"+strings.ReplaceAll(f.table, "-", "")+" SET record_version=record_version+1 WHERE id=$1", f.ownRecord); e != nil {
					t.Fatal(e)
				}
			} else {
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
					t.Fatal(e)
				}
			}
			r, e := f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, q, applications.Metadata{RequestID: mode})
			want := ErrWorkflowBasisChanged
			if mode == "revoke" {
				want = applications.ErrDenied
			}
			if !errors.Is(e, want) || r.CommandID != "" {
				t.Fatalf("stale acceptance %+v %v", r, e)
			}
		})
	}
}
func TestRootLifecycleActionWriteFailureRollsBackEverything(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootLifecycleActionReq(t, f, "withdraw")
	fn := "root_lifecycle_" + strings.ReplaceAll(f.app, "-", "")
	sql := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root lifecycle write fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.workflow_evidence_documents FOR EACH ROW WHEN (NEW.app_id='%s') EXECUTE FUNCTION applications.%s()", fn, fn, f.app, fn)
	if _, e := f.owner.Exec(f.ctx, sql); e != nil {
		t.Fatal(e)
	}
	cleanup := func() {
		_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.workflow_evidence_documents; DROP FUNCTION IF EXISTS applications.%s()", fn, fn))
	}
	t.Cleanup(cleanup)
	r, e := f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, q, applications.Metadata{RequestID: "fault"})
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "P0001" || !strings.Contains(pg.Message, "root lifecycle write fault") || r.CommandID != "" {
		t.Fatalf("did not reach actual injection %v", e)
	}
	for _, table := range []string{"operations", "workflow_evidence_documents", "record_command_fences"} {
		var n int
		if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 0 {
			t.Fatalf("partial %s %d %v", table, n, e)
		}
	}
	cleanup()
	if rootLifecycleAccept(t, f, q).Status != "pending" {
		t.Fatal("rollback blocked retry")
	}
}
func TestRootLifecycleOperationKindsAndClosedReceipt(t *testing.T) {
	f := newRecordFixture(t)
	for _, kind := range []string{"workflow.instance.withdraw", "workflow.task.return"} {
		t.Run(kind, func(t *testing.T) {
			op, cmd, instance := recordOperationID(t, f), recordOperationID(t, f), recordOperationID(t, f)
			body, _ := json.Marshal(WorkflowOperationResult{OperationID: op, CommandID: cmd, InstanceID: instance, Status: "pending"})
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			if _, e = tx.Exec(f.ctx, "INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,$4,$5,$6,202,$7)", f.actor, op, f.app, kind, make([]byte, 32), body, "/api/v1/application-workflow-operations/"+op); e != nil {
				t.Fatal(e)
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatal(e)
			}
			bad := recordOperationID(t, f)
			if _, e = f.owner.Exec(f.ctx, "INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,$4,$5,'{}',202,$6)", f.actor, bad, f.app, kind, make([]byte, 32), "/bad"); e == nil {
				t.Fatal("malformed receipt accepted")
			}
		})
	}
}
