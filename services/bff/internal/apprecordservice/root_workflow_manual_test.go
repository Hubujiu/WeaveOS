package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"strings"
	"sync"
	"testing"
)

func rootManualSetup(t *testing.T) (recordFixture, WorkflowManualStartRequest) {
	t.Helper()
	f := rootCaptureSetup(t).recordFixture
	rootTaskKeepDispatchPrivate(t, f)
	h := rootConfiguredTrigger(t, f, "manual", nil)
	return f, WorkflowManualStartRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, FlowID: h.FlowID, OperationID: recordOperationID(t, f), ExpectedWorkflowRevision: h.Revision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1}
}
func rootManualStart(t *testing.T, f recordFixture, r WorkflowManualStartRequest) WorkflowManualStartResult {
	t.Helper()
	got, e := f.service.StartManualWorkflow(f.ctx, f.principal, r, applications.Metadata{RequestID: "manual-start"})
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func rootManualNoReceipt(t *testing.T, f recordFixture, op string) {
	t.Helper()
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_id=$2", f.app, op).Scan(&n); e != nil || n != 0 {
		t.Fatalf("rejected manual request persisted receipt %d %v", n, e)
	}
}
func TestRootManualStartCreatorAcceptsDurableIntentAndRecoversReceipt(t *testing.T) {
	f, r := rootManualSetup(t)
	got := rootManualStart(t, f, r)
	if got.OperationID != r.OperationID || got.FlowID != r.FlowID || got.InstanceID == "" || got.Status != "accepted" || got.Ignored {
		t.Fatalf("not an accepted minimal intent: %+v", got)
	}
	var state, actor, record string
	var version int64
	if e := f.owner.QueryRow(f.ctx, "SELECT state,initiator_id::text,record_id::text,definition_version FROM applications.workflow_instances WHERE id=$1", got.InstanceID).Scan(&state, &actor, &record, &version); e != nil || state != "starting" || actor != f.actor || record != r.RecordID || version != 1 {
		t.Fatalf("wrong durable identity %s/%s/%s/v%d %v", state, actor, record, version, e)
	}
	if replay := rootManualStart(t, f, r); replay != got {
		t.Fatal("replay replaced original intent")
	}
	rootTriggeredCount(t, f, r.RecordID, 1)
	// Revocation blocks new action, but not recovery of a confirmed minimum result.
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	if replay := rootManualStart(t, f, r); replay != got {
		t.Fatal("revocation lost confirmed minimum receipt")
	}
	r.OperationID = recordOperationID(t, f)
	if _, e := f.service.StartManualWorkflow(f.ctx, f.principal, r, applications.Metadata{RequestID: "manual-revoked"}); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("revoked new request accepted %v", e)
	}
	rootManualNoReceipt(t, f, r.OperationID)
}
func TestRootManualStartOwnerCannotBypassRecordCreator(t *testing.T) {
	f, r := rootManualSetup(t)
	p := f.principal
	p.UserID = f.other
	p.SessionRef = recordOperationID(t, f)
	if _, e := f.service.StartManualWorkflow(f.ctx, p, r, applications.Metadata{RequestID: "manual-owner"}); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("app owner bypassed creator requirement %v", e)
	}
	rootTriggeredCount(t, f, r.RecordID, 0)
	rootManualNoReceipt(t, f, r.OperationID)
}
func TestRootManualStartForeignRecordCannotBypassCreator(t *testing.T) {
	f, r := rootManualSetup(t)
	r.RecordID = f.otherRecord
	if _, e := f.service.StartManualWorkflow(f.ctx, f.principal, r, applications.Metadata{RequestID: "manual-foreign"}); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("reader bypassed creator requirement %v", e)
	}
	rootTriggeredCount(t, f, r.RecordID, 0)
	rootManualNoReceipt(t, f, r.OperationID)
}
func TestRootManualStartIgnoresInFlightButRejectsTerminalHistory(t *testing.T) {
	for _, state := range []string{"starting", "active", "completed", "rejected", "withdrawn", "no_effect"} {
		t.Run(state, func(t *testing.T) {
			f, r := rootManualSetup(t)
			original := rootManualStart(t, f, r)
			// Owner only models confirmed states for admission policy; no engine proof claimed.
			if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state=$2 WHERE id=$1", original.InstanceID, state); e != nil {
				t.Fatal(e)
			}
			r.OperationID = recordOperationID(t, f)
			got, e := f.service.StartManualWorkflow(f.ctx, f.principal, r, applications.Metadata{RequestID: "manual-history"})
			if state == "starting" || state == "active" {
				if e != nil || !got.Ignored || got.Status != "ignored" || got.InstanceID != original.InstanceID {
					t.Fatalf("duplicate not ignored %+v %v", got, e)
				}
			} else {
				if !errors.Is(e, workflowcatalog.ErrConflict) || got.InstanceID != "" {
					t.Fatalf("ordinary manual bypassed rework/review role gate %+v %v", got, e)
				}
				rootManualNoReceipt(t, f, r.OperationID)
			}
			rootTriggeredCount(t, f, r.RecordID, 1)
		})
	}
}
func TestRootManualStartDifferentFlowsIndependent(t *testing.T) {
	f, r := rootManualSetup(t)
	a := rootManualStart(t, f, r)
	h := rootConfiguredTrigger(t, f, "manual", nil)
	r.FlowID = h.FlowID
	r.ExpectedWorkflowRevision = h.Revision
	r.OperationID = recordOperationID(t, f)
	b := rootManualStart(t, f, r)
	if b.Ignored || b.InstanceID == a.InstanceID {
		t.Fatal("A history incorrectly blocked B")
	}
	rootTriggeredCount(t, f, r.RecordID, 2)
}
func TestRootManualStartConcurrentRequestsReserveOnce(t *testing.T) {
	f, r := rootManualSetup(t)
	requests := []WorkflowManualStartRequest{r, r, r, r}
	for i := 1; i < len(requests); i++ {
		requests[i].OperationID = recordOperationID(t, f)
	}
	var wg sync.WaitGroup
	results := make([]WorkflowManualStartResult, 4)
	errs := make([]error, 4)
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = f.service.StartManualWorkflow(f.ctx, f.principal, requests[i], applications.Metadata{RequestID: "concurrent-manual"})
		}(i)
	}
	wg.Wait()
	n := 0
	id := ""
	for i, g := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !g.Ignored {
			n++
		}
		if id == "" {
			id = g.InstanceID
		}
		if id != g.InstanceID {
			t.Fatal("concurrent requests created distinct instances")
		}
	}
	if n != 1 {
		t.Fatalf("accepted %d want 1", n)
	}
	rootTriggeredCount(t, f, r.RecordID, 1)
}
func TestRootManualStartCurrentRevisionAndRecordCASAreRequired(t *testing.T) {
	for _, part := range []string{"flow", "schema", "record"} {
		t.Run(part, func(t *testing.T) {
			f, r := rootManualSetup(t)
			switch part {
			case "flow":
				r.ExpectedWorkflowRevision++
			case "schema":
				r.ExpectedSchemaVersion++
			case "record":
				r.ExpectedRecordVersion++
			}
			_, e := f.service.StartManualWorkflow(f.ctx, f.principal, r, applications.Metadata{RequestID: "manual-cas"})
			if !errors.Is(e, workflowcatalog.ErrConflict) {
				t.Fatalf("stale %s did not conflict %v", part, e)
			}
			rootTriggeredCount(t, f, r.RecordID, 0)
			rootManualNoReceipt(t, f, r.OperationID)
		})
	}
}
func TestRootManualStartRequiresPublishedManualMatch(t *testing.T) {
	for _, kind := range []string{"no_manual", "no_match", "closing", "candidate_only"} {
		t.Run(kind, func(t *testing.T) {
			f, r := rootManualSetup(t)
			if kind == "no_manual" || kind == "no_match" || kind == "candidate_only" {
				event := "record.updated"
				var condition []byte
				if kind == "no_match" {
					event = "manual"
				}
				// Build the condition from the independently defined existing DSL fixture shape.
				if kind == "no_match" {
					condition = []byte(strings.Replace(string(rootTriggerCondition(f.public)), "alpha", "never-match", 1))
				}
				h := rootConfiguredTrigger(t, f, event, condition)
				r.FlowID = h.FlowID
				r.ExpectedWorkflowRevision = h.Revision
				if kind == "candidate_only" {
					cfg := []workflowcatalog.Trigger{{Event: "manual"}}
					in := rootCatalogInput(f, h.FlowID, h.Revision, rootCatalogGraph(t, f, false))
					in.Triggers = &cfg
					rootCatalogTx(t, f, func(tx pgx.Tx) error {
						next, e := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
						r.ExpectedWorkflowRevision = next.Revision
						return e
					})
				}
			} else {
				rootCatalogTx(t, f, func(tx pgx.Tx) error {
					h, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, r.FlowID, r.ExpectedWorkflowRevision)
					r.ExpectedWorkflowRevision = h.Revision
					return e
				})
			}
			_, e := f.service.StartManualWorkflow(f.ctx, f.principal, r, applications.Metadata{RequestID: "manual-config"})
			if !errors.Is(e, workflowcatalog.ErrNotReady) && !errors.Is(e, workflowcatalog.ErrClosing) {
				t.Fatalf("%s admission not rejected %v", kind, e)
			}
			rootTriggeredCount(t, f, r.RecordID, 0)
			rootManualNoReceipt(t, f, r.OperationID)
		})
	}
}
