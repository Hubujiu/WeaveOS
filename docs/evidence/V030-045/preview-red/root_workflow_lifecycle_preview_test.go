package apprecordservice

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
	"time"
)

func rootLifecycleRequest(f rootTaskFixture) WorkflowLifecycleRequest {
	return WorkflowLifecycleRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, InstanceID: f.instance.ID}
}
func TestRootLifecyclePreviewCurrentRecordAndTargets(t *testing.T) {
	f := rootTaskSetup(t, false)
	r, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, rootLifecycleRequest(f))
	if e != nil {
		t.Fatal(e)
	}
	if r.BasisToken == "" || !r.CanWithdraw || r.Instance.ID != f.instance.ID || r.Instance.Sequence != 1 || r.Record.ID != f.ownRecord || r.Record.RecordVersion != 1 || len(r.ReturnTargets) != 1 || r.ReturnTargets[0].NodeID != f.task.NodeID || r.ReturnTargets[0].TaskID != f.task.ID || r.ReturnTargets[0].ActivationEpoch != 1 {
		t.Fatalf("wrong preview %+v", r)
	}
	m, e := f.service.WorkflowLifecycleBases.Load(f.ctx, f.principal.SessionRef, r.BasisToken)
	if e != nil || m.Fingerprint == "" {
		t.Fatalf("missing actual basis %v", e)
	}
	if _, e = f.service.WorkflowLifecycleBases.Load(f.ctx, recordOperationID(t, f.recordFixture), r.BasisToken); e == nil {
		t.Fatal("cross-session basis readable")
	}
}
func TestRootLifecyclePreviewRevokedRecordAccessDenied(t *testing.T) {
	f := rootTaskSetup(t, false)
	if _, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, rootLifecycleRequest(f)); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	r, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, rootLifecycleRequest(f))
	if !errors.Is(e, applications.ErrDenied) || r.BasisToken != "" {
		t.Fatalf("revoked preview %+v %v", r, e)
	}
}
func TestRootLifecyclePreviewRejectsMismatchedRecord(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootLifecycleRequest(f)
	q.RecordID = f.otherRecord
	r, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, q)
	if !errors.Is(e, applications.ErrMissing) || r.BasisToken != "" {
		t.Fatalf("crossrecord %+v %v", r, e)
	}
}
func TestRootLifecyclePreviewTerminalCannotReturnOrWithdraw(t *testing.T) {
	f := rootTaskSetup(t, false)
	if _, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, rootLifecycleRequest(f)); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state='approved',sequence=sequence+1 WHERE id=$1", f.instance.ID); e != nil {
		t.Fatal(e)
	}
	r, e := f.service.PreviewWorkflowLifecycle(f.ctx, f.principal, rootLifecycleRequest(f))
	if !errors.Is(e, ErrWorkflowTaskChanged) || r.BasisToken != "" {
		t.Fatalf("terminal preview %+v %v", r, e)
	}
}
func TestRootLifecyclePreviewSingleConnectionDoesNotNestPool(t *testing.T) {
	f := rootTaskSetup(t, false)
	config := f.runtime.Config().Copy()
	config.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	svc := *f.service
	svc.Pool = pool
	ctx, cancel := context.WithTimeout(f.ctx, 3*time.Second)
	defer cancel()
	if _, e = svc.PreviewWorkflowLifecycle(ctx, f.principal, rootLifecycleRequest(f)); e != nil {
		t.Fatal(e)
	}
}
