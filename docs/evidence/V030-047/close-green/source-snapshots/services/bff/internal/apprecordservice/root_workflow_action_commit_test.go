package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
	"sync/atomic"
	"testing"
)

func TestRootWorkflowActionLostActualCommitReplyRecoversOnePendingCommand(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	var dropped atomic.Bool
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, &dropped, nil)
	result, e := faulty.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "workflow-lost-commit"})
	if !dropped.Load() || !errors.Is(e, applications.ErrUnconfirmed) || result.CommandID != "" {
		t.Fatalf("actual lost commit was not unconfirmed: dropped%v result%+v err%v", dropped.Load(), result, e)
	}
	rootActionCount(t, f, 1, 1, 1)
	recovered, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
	if e != nil || recovered.Status != "pending" || recovered.CommandID == "" || recovered.Sequence != 0 {
		t.Fatalf("accepted action not recoverable: %+v %v", recovered, e)
	}
	replay := rootAction(t, f, req)
	if replay != recovered {
		t.Fatal("lost commit retry changed original command")
	}
	rootActionCount(t, f, 1, 1, 1)
}
func TestRootWorkflowActionActualCommitRollbackLeavesNoPartialAcceptance(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	abort := &abortRecordBeforeCommit{}
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, nil, abort)
	result, e := faulty.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "workflow-definite-rollback"})
	if !abort.injected.Load() || !errors.Is(e, pgx.ErrTxCommitRollback) || errors.Is(e, applications.ErrUnconfirmed) || result.CommandID != "" {
		t.Fatalf("definite commit rollback misclassified: result%+v err%v", result, e)
	}
	rootActionCount(t, f, 0, 0, 0)
	if _, e = f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID); !errors.Is(e, applications.ErrMissing) {
		t.Fatalf("rolled-back action reported accepted: %v", e)
	}
	result = rootAction(t, f, req)
	if result.Status != "pending" {
		t.Fatal("same original operation cannot retry after proven rollback")
	}
	rootActionCount(t, f, 1, 1, 1)
}
