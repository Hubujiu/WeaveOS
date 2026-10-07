package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"testing"
)

func TestRootWorkflowPreviewRevokedPermissionDoesNotRevealClosedTask(t *testing.T) {
	f := rootTaskSetup(t, false)
	c, p := f.accept(t, "agree", f.task.ID, "", 1, 1)
	r, b := f.receipt(t, c, "completed", "")
	f.apply(t, c, p, r, b, true)
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	got, e := f.service.PreviewWorkflowTask(f.ctx, f.principal, f.request())
	if !errors.Is(e, applications.ErrDenied) || got.BasisToken != "" || len(got.Record.Values) != 0 {
		t.Fatalf("revoked actor learned closed task state: %v", e)
	}
}
