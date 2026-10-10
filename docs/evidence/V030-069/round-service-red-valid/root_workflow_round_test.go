package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"sync"
	"testing"
)

func rootRoundSetup(t *testing.T, state string) (recordFixture, WorkflowRoundRequest, WorkflowManualStartRequest) {
	t.Helper()
	f, r := rootManualSetup(t)
	v := rootManualStart(t, f, r)
	rootRoundTerminal(t, f, v.InstanceID, state)
	return f, WorkflowRoundRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, InstanceID: v.InstanceID}, r
}
func rootRoundStartRequest(t *testing.T, f recordFixture, q WorkflowRoundRequest, r WorkflowManualStartRequest, kind string) WorkflowRoundStartRequest {
	return WorkflowRoundStartRequest{WorkflowRoundRequest: q, OperationID: recordOperationID(t, f), Kind: kind, ExpectedWorkflowRevision: r.ExpectedWorkflowRevision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1}
}
func TestRootRoundServicePreviewLatestAndHistoricalRoles(t *testing.T) {
	f, q, r := rootRoundSetup(t, "rejected")
	v, e := f.service.PreviewWorkflowRound(f.ctx, f.principal, q)
	if e != nil || !v.CanRework || !v.CanResubmit || !v.CanReview || v.RoundNumber != 1 || v.LatestInstanceID != q.InstanceID || v.FlowID != r.FlowID || v.SchemaVersion != 1 || v.RecordVersion != 1 {
		t.Fatalf("latest rejected original/configured approver %+v %v", v, e)
	}
	if len(v.EditableFieldIDs) == 0 {
		t.Fatal("current editable fields missing")
	}
	// A new accepted intent makes the old terminal round read-only immediately.
	in := rootCatalogReserveInput(t, f, workflowcatalog.Head{FlowID: r.FlowID, Revision: r.ExpectedWorkflowRevision})
	next := rootCatalogReserve(t, f, in)
	v, e = f.service.PreviewWorkflowRound(f.ctx, f.principal, q)
	if e != nil || v.CanRework || v.CanResubmit || v.CanReview || len(v.EditableFieldIDs) != 0 || v.LatestInstanceID != next.ID {
		t.Fatalf("old round retained capabilities %+v %v", v, e)
	}
}
func TestRootRoundServicePreviewCurrentPermissionAndScope(t *testing.T) {
	f, q, _ := rootRoundSetup(t, "completed")
	v, e := f.service.PreviewWorkflowRound(f.ctx, f.principal, q)
	if e != nil || !v.CanReview || v.CanResubmit || v.CanRework {
		t.Fatalf("configured approver never assigned a task cannot review %+v %v", v, e)
	}
	foreign := q
	foreign.RecordID = f.otherRecord
	if _, e = f.service.PreviewWorkflowRound(f.ctx, f.principal, foreign); !errors.Is(e, applications.ErrMissing) {
		t.Fatalf("foreign instance disclosure %v", e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	if _, e = f.service.PreviewWorkflowRound(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("revoked preview allowed %v", e)
	}
}
func TestRootRoundServiceResubmitAndReviewPreserveOriginal(t *testing.T) {
	for _, kind := range []string{"resubmit", "review"} {
		t.Run(kind, func(t *testing.T) {
			state := "rejected"
			if kind == "review" {
				state = "completed"
			}
			f, q, r := rootRoundSetup(t, state)
			req := rootRoundStartRequest(t, f, q, r, kind)
			meta := applications.Metadata{RequestID: "round-accept"}
			got, e := f.service.StartWorkflowRound(f.ctx, f.principal, req, meta)
			if e != nil || got.InstanceID == "" || got.InstanceID == q.InstanceID || got.PreviousInstanceID != q.InstanceID || got.RoundKind != kind || got.RoundNumber != 2 || got.Status != "accepted" || got.OperationID != req.OperationID || got.FlowID != r.FlowID {
				t.Fatalf("new linked intent %+v %v", got, e)
			}
			replay, e := f.service.StartWorkflowRound(f.ctx, f.principal, req, meta)
			if e != nil || replay != got {
				t.Fatalf("original receipt changed %+v %v", replay, e)
			}
			var original string
			if e = f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE id=$1", q.InstanceID).Scan(&original); e != nil || original != state {
				t.Fatalf("overwrote historical result %s %v", original, e)
			}
			rootTriggeredCount(t, f, q.RecordID, 2)
			req.OperationID = recordOperationID(t, f)
			if _, e = f.service.StartWorkflowRound(f.ctx, f.principal, req, meta); !errors.Is(e, workflowcatalog.ErrConflict) {
				t.Fatalf("old round reaccepted %v", e)
			}
			rootManualNoReceipt(t, f, req.OperationID)
		})
	}
}
func TestRootRoundServiceStartCASAndRevocation(t *testing.T) {
	for _, part := range []string{"schema", "record", "flow", "revoked", "role"} {
		t.Run(part, func(t *testing.T) {
			f, q, r := rootRoundSetup(t, "rejected")
			req := rootRoundStartRequest(t, f, q, r, "resubmit")
			p := f.principal
			switch part {
			case "schema":
				req.ExpectedSchemaVersion++
			case "record":
				req.ExpectedRecordVersion++
			case "flow":
				req.ExpectedWorkflowRevision++
			case "revoked":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
					t.Fatal(e)
				}
			case "role":
				p.UserID = f.other
				p.SessionRef = recordOperationID(t, f)
			}
			_, e := f.service.StartWorkflowRound(f.ctx, p, req, applications.Metadata{RequestID: "round-reject"})
			want := workflowcatalog.ErrConflict
			if part == "role" || part == "revoked" {
				want = applications.ErrDenied
			}
			if !errors.Is(e, want) {
				t.Fatalf("%s wrong denial %v", part, e)
			}
			rootTriggeredCount(t, f, q.RecordID, 1)
			rootManualNoReceipt(t, f, req.OperationID)
		})
	}
}
func TestRootRoundServiceConcurrentSuccessorExactlyOne(t *testing.T) {
	f, q, r := rootRoundSetup(t, "rejected")
	requests := []WorkflowRoundStartRequest{rootRoundStartRequest(t, f, q, r, "resubmit"), rootRoundStartRequest(t, f, q, r, "review")}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = f.service.StartWorkflowRound(f.ctx, f.principal, requests[i], applications.Metadata{RequestID: "round-race"})
		}(i)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, workflowcatalog.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success/conflict=%d/%d", success, conflict)
	}
	rootTriggeredCount(t, f, q.RecordID, 2)
}
func TestRootRoundServiceReworkIsIndependentSaveWithOtherFlowActive(t *testing.T) {
	f, q, _ := rootRoundSetup(t, "rejected")
	otherHead := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	b := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, otherHead))
	rootRoundTerminal(t, f, b.ID, "active")
	req := WorkflowRoundReworkRequest{WorkflowRoundRequest: q, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "reworked"}}
	meta := applications.Metadata{RequestID: "round-save"}
	got, e := f.service.ReworkWorkflowRound(f.ctx, f.principal, req, meta)
	if e != nil || got.RecordVersion != 2 || got.ID != q.RecordID {
		t.Fatalf("authorized A rework while B active %+v %v", got, e)
	}
	replay, e := f.service.ReworkWorkflowRound(f.ctx, f.principal, req, meta)
	if e != nil || replay != got {
		t.Fatalf("rework replay %+v %v", replay, e)
	}
	row, e := f.service.GetRecord(f.ctx, f.principal, f.app, f.view, q.RecordID)
	if e != nil || row.Values[f.public] != "reworked" {
		t.Fatalf("not persisted %v", e)
	}
	var state string
	if e = f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE id=$1", b.ID).Scan(&state); e != nil || state != "active" {
		t.Fatalf("B changed %s %v", state, e)
	}
	rootTriggeredCount(t, f, q.RecordID, 2)
	req.OperationID = recordOperationID(t, f)
	if _, e = f.service.ReworkWorkflowRound(f.ctx, f.principal, req, meta); e == nil {
		t.Fatal("stale rework overwrote newer values")
	}
	rootManualNoReceipt(t, f, req.OperationID)
}
