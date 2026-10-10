package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
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

// Supplemental adversarial qualification of the already RED-first service.
func TestRootRoundServiceCommitFaultRollsBackAllEffects(t *testing.T) {
	for _, kind := range []string{"resubmit", "rework"} {
		t.Run(kind, func(t *testing.T) {
			f, q, r := rootRoundSetup(t, "rejected")
			name := "round_fault_" + strings.ReplaceAll(f.app, "-", "")
			function := pgx.Identifier{"applications", name}.Sanitize()
			trigger := pgx.Identifier{name}.Sanitize()
			if _, e := f.owner.Exec(f.ctx, "CREATE FUNCTION "+function+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.app_id='"+f.app+"'::uuid THEN RAISE EXCEPTION 'isolated round commit fault' USING ERRCODE='P0001'; END IF; RETURN NEW; END $$"); e != nil {
				t.Fatal(e)
			}
			defer f.owner.Exec(f.ctx, "DROP FUNCTION "+function+"() CASCADE")
			if _, e := f.owner.Exec(f.ctx, "CREATE CONSTRAINT TRIGGER "+trigger+" AFTER INSERT ON applications.operations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION "+function+"()"); e != nil {
				t.Fatal(e)
			}
			op := recordOperationID(t, f)
			var e error
			if kind == "resubmit" {
				in := rootRoundStartRequest(t, f, q, r, kind)
				in.OperationID = op
				_, e = f.service.StartWorkflowRound(f.ctx, f.principal, in, applications.Metadata{RequestID: "round-commit-fault"})
			} else {
				_, e = f.service.ReworkWorkflowRound(f.ctx, f.principal, WorkflowRoundReworkRequest{WorkflowRoundRequest: q, OperationID: op, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "must rollback"}}, applications.Metadata{RequestID: "round-commit-fault"})
			}
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "P0001" || !strings.Contains(pg.Message, "isolated round commit fault") {
				t.Fatalf("fault not reached %v", e)
			}
			rootManualNoReceipt(t, f, op)
			rootTriggeredCount(t, f, q.RecordID, 1)
			var rounds int
			if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_rounds WHERE app_id=$1", f.app).Scan(&rounds); e != nil || rounds != 1 {
				t.Fatal("failed commit left round", rounds, e)
			}
			preview, e := f.service.PreviewWorkflowRound(f.ctx, f.principal, q)
			if e != nil || preview.RecordVersion != 1 {
				t.Fatal("failed commit changed record", e)
			}
		})
	}
}
func TestRootRoundServiceReworkBoundariesAndUnknownFence(t *testing.T) {
	for _, mode := range []string{"completed", "old", "field", "revoked-edit", "fence"} {
		t.Run(mode, func(t *testing.T) {
			f, q, r := rootRoundSetup(t, "rejected")
			req := WorkflowRoundReworkRequest{WorkflowRoundRequest: q, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "denied"}}
			var want error = applications.ErrDenied
			switch mode {
			case "completed":
				rootRoundTerminal(t, f, q.InstanceID, "completed")
			case "old":
				rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, workflowcatalog.Head{FlowID: r.FlowID, Revision: r.ExpectedWorkflowRevision}))
				want = workflowcatalog.ErrConflict
			case "field":
				req.Changes[f.secret] = "forbidden"
			case "revoked-edit":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.edit')", f.app); e != nil {
					t.Fatal(e)
				}
			case "fence":
				rootFenceAcquire(t, f, recordOperationID(t, f))
				want = nil
			}
			_, e := f.service.ReworkWorkflowRound(f.ctx, f.principal, req, applications.Metadata{RequestID: "round-boundary"})
			if mode == "fence" {
				var fence *appstructure.Error
				if !errors.As(e, &fence) || fence.Code != "APPLICATION_RECORD_FENCED" {
					t.Fatal("unknown fence bypassed", e)
				}
			} else if !errors.Is(e, want) {
				t.Fatalf("%s want %v got %v", mode, want, e)
			}
			rootManualNoReceipt(t, f, req.OperationID)
			p, e := f.service.PreviewWorkflowRound(f.ctx, f.principal, q)
			if e != nil || p.RecordVersion != 1 {
				t.Fatal("denied edit changed row", e)
			}
			if mode == "fence" {
				_, e = f.service.StartWorkflowRound(f.ctx, f.principal, rootRoundStartRequest(t, f, q, r, "review"), applications.Metadata{RequestID: "fenced-round-start"})
				var fence *appstructure.Error
				if !errors.As(e, &fence) || fence.Code != "APPLICATION_RECORD_FENCED" {
					t.Fatal("fenced start accepted", e)
				}
				rootFenceCount(t, f, 1)
			}
		})
	}
}
func TestRootRoundServiceReplayBoundToExactOriginalRequest(t *testing.T) {
	f, q, r := rootRoundSetup(t, "rejected")
	req := rootRoundStartRequest(t, f, q, r, "resubmit")
	got, e := f.service.StartWorkflowRound(f.ctx, f.principal, req, applications.Metadata{RequestID: "round-replay"})
	if e != nil {
		t.Fatal(e)
	}
	bad := req
	bad.ExpectedRecordVersion++
	if _, e = f.service.StartWorkflowRound(f.ctx, f.principal, bad, applications.Metadata{RequestID: "round-collision"}); !errors.Is(e, applications.ErrOperationConflict) {
		t.Fatal("same key changed request", e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	again, e := f.service.StartWorkflowRound(f.ctx, f.principal, req, applications.Metadata{RequestID: "round-recovery"})
	if e != nil || again != got {
		t.Fatal("minimal original receipt unrecoverable", e)
	}
	rootTriggeredCount(t, f, q.RecordID, 2)
}
