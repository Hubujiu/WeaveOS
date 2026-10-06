package apprecordservice

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
)

func TestRootRuntimeAdmissionRejectsUnavailableWithoutConsumingAction(t *testing.T) {
	for _, action := range []string{"agree", "reject"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := rootActionRequest(t, f, action)
			f.service.RuntimeReady = func(context.Context) error { return errors.New("private_engine_failure") }
			got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "runtime-unavailable"})
			if !errors.Is(e, ErrUnavailable) || got.CommandID != "" {
				t.Fatalf("unavailable engine accepted new action: %+v %v", got, e)
			}
			if strings.Contains(e.Error(), "private_engine_failure") {
				t.Fatal("engine error leaked")
			}
			rootActionCount(t, f, 0, 0, 0)
			f.service.RuntimeReady = func(context.Context) error { return nil }
			if got := rootAction(t, f, req); got.Status != "pending" {
				t.Fatal("same operation/basis could not retry after recovery")
			}
			rootActionCount(t, f, 1, 1, 1)
		})
	}
}
func TestRootRuntimeAdmissionChecksHealthOutsideBusinessTransaction(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	calls := 0
	f.service.RuntimeReady = func(ctx context.Context) error {
		calls++
		if ctx == nil || ctx.Err() != nil {
			t.Error("invalid request context")
		}
		if f.service.Pool.Stat().AcquiredConns() != 0 {
			t.Error("health RPC would hold a business connection/transaction")
		}
		return nil
	}
	rootAction(t, f, req)
	if calls != 1 {
		t.Fatalf("runtime health check count=%d want1", calls)
	}
}
func TestRootRuntimeAdmissionKeepsAcceptedReplayDuringOutage(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	first := rootAction(t, f, req)
	f.service.RuntimeReady = func(context.Context) error { return errors.New("synthetic outage") }
	if again := rootAction(t, f, req); again != first {
		t.Fatal("outage changed already accepted command identity")
	}
	rootActionCount(t, f, 1, 1, 1)
}
func TestRootRuntimeAdmissionPreservesActualRowVisibilityPriority(t *testing.T) {
	f := rootTaskSetup(t, true)
	req := rootActionRequest(t, f, "agree")
	for _, q := range []string{"DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all')", "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'"} {
		if _, e := f.owner.Exec(f.ctx, q, f.app); e != nil {
			t.Fatal(e)
		}
	}
	f.service.RuntimeReady = func(context.Context) error { return errors.New("synthetic outage") }
	got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "outage-no-row"})
	if !errors.Is(e, applications.ErrMissing) || got.CommandID != "" {
		t.Fatalf("outage bypassed row visibility precedence: %v", e)
	}
	rootActionCount(t, f, 0, 0, 0)
}
