package apprecordservice

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5/pgconn"
	"sync"
	"testing"
)

func rootRoundSchema(t *testing.T, f recordFixture) {
	t.Helper()
	var present bool
	if e := f.owner.QueryRow(f.ctx, "SELECT to_regclass('applications.workflow_rounds') IS NOT NULL").Scan(&present); e != nil {
		t.Fatal(e)
	}
	if !present {
		t.Fatal("FLOW14: durable instance round identity is missing")
	}
}
func rootRoundNumber(t *testing.T, f recordFixture, id string) int64 {
	t.Helper()
	var n int64
	if e := f.runtime.QueryRow(f.ctx, "SELECT round_number FROM applications.workflow_rounds WHERE instance_id=$1", id).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func rootRoundTerminal(t *testing.T, f recordFixture, id, state string) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state=$2 WHERE id=$1", id, state); e != nil {
		t.Fatal(e)
	}
}
func TestRootRoundStoreIndependentMonotonicOrderAndReplay(t *testing.T) {
	f := newRecordFixture(t)
	rootRoundSchema(t, f)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	r := rootCatalogReserveInput(t, f, h)
	first := rootCatalogReserve(t, f, r)
	if n := rootRoundNumber(t, f, first.ID); n != 1 {
		t.Fatalf("first round=%d want1", n)
	}
	rootCatalogReserve(t, f, r)
	rootRoundTerminal(t, f, first.ID, "rejected")
	// Backdated wall clocks never replace the persistent acceptance order.
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET created_at='2099-01-01' WHERE id=$1", first.ID); e != nil {
		t.Fatal(e)
	}
	r.InstanceID = recordOperationID(t, f)
	second := rootCatalogReserve(t, f, r)
	if n := rootRoundNumber(t, f, second.ID); n != 2 {
		t.Fatalf("second round=%d want2", n)
	}
	r.InstanceID = recordOperationID(t, f)
	r.RecordID = f.otherRecord
	other := rootCatalogReserve(t, f, r)
	if rootRoundNumber(t, f, other.ID) != 1 {
		t.Fatal("other record shared counter")
	}
	h2 := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	b := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h2))
	if rootRoundNumber(t, f, b.ID) != 1 {
		t.Fatal("other flow shared counter")
	}
	var count int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_rounds WHERE app_id=$1", f.app).Scan(&count); e != nil || count != 4 {
		t.Fatalf("replay created round count=%d err=%v", count, e)
	}
}
func TestRootRoundStoreLinkedSuccessorAndOldParentRefusal(t *testing.T) {
	for _, kind := range []string{"resubmit", "review"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecordFixture(t)
			rootRoundSchema(t, f)
			h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
			r := rootCatalogReserveInput(t, f, h)
			old := rootCatalogReserve(t, f, r)
			rootRoundTerminal(t, f, old.ID, "rejected")
			r.InstanceID = recordOperationID(t, f)
			r.PreviousInstanceID = old.ID
			r.RoundKind = kind
			next := rootCatalogReserve(t, f, r)
			var parent, storedKind string
			if e := f.runtime.QueryRow(f.ctx, "SELECT previous_instance_id::text,round_kind FROM applications.workflow_rounds WHERE instance_id=$1", next.ID).Scan(&parent, &storedKind); e != nil || parent != old.ID || storedKind != kind {
				t.Fatalf("lost successor link %s %s %v", parent, storedKind, e)
			}
			rootCatalogReserve(t, f, r)
			r.InstanceID = recordOperationID(t, f)
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			if _, e = (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, r); e == nil {
				t.Fatal("old parent admitted a second successor")
			}
		})
	}
}
func TestRootRoundStoreInvalidParentAndStateFailClosed(t *testing.T) {
	for _, change := range []string{"active", "completed-resubmit", "foreign-record", "wrong-starter", "missing-parent", "initial-with-parent"} {
		t.Run(change, func(t *testing.T) {
			f := newRecordFixture(t)
			rootRoundSchema(t, f)
			h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
			r := rootCatalogReserveInput(t, f, h)
			old := rootCatalogReserve(t, f, r)
			rootRoundTerminal(t, f, old.ID, "rejected")
			r.InstanceID = recordOperationID(t, f)
			r.PreviousInstanceID = old.ID
			r.RoundKind = "resubmit"
			switch change {
			case "active":
				rootRoundTerminal(t, f, old.ID, "active")
			case "completed-resubmit":
				rootRoundTerminal(t, f, old.ID, "completed")
			case "foreign-record":
				r.RecordID = f.otherRecord
			case "wrong-starter":
				r.ActorID = f.other
			case "missing-parent":
				r.PreviousInstanceID = ""
			case "initial-with-parent":
				r.RoundKind = "initial"
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			if _, e = (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, r); e == nil {
				t.Fatalf("%s accepted", change)
			}
		})
	}
}
func TestRootRoundStoreConcurrentNumbersAndRollback(t *testing.T) {
	f := newRecordFixture(t)
	rootRoundSchema(t, f)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	var wg sync.WaitGroup
	errch := make(chan error, 4)
	ids := make([]string, 4)
	for i := range ids {
		ids[i] = recordOperationID(t, f)
	}
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				errch <- e
				return
			}
			defer tx.Rollback(context.Background())
			r := workflowcatalog.ReserveInput{AppID: f.app, FlowID: h.FlowID, InstanceID: id, RecordID: f.ownRecord, ActorID: f.actor, ExpectedRevision: h.Revision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1}
			_, e = (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, r)
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			errch <- e
		}(id)
	}
	wg.Wait()
	close(errch)
	for e := range errch {
		if e != nil {
			t.Fatal(e)
		}
	}
	var n, min, max int64
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*),min(round_number),max(round_number) FROM applications.workflow_rounds WHERE app_id=$1", f.app).Scan(&n, &min, &max); e != nil || n != 4 || min != 1 || max != 4 {
		t.Fatalf("not serial 1..4 %d/%d/%d %v", n, min, max, e)
	}
	r := rootCatalogReserveInput(t, f, h)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, r); e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	r.InstanceID = recordOperationID(t, f)
	got := rootCatalogReserve(t, f, r)
	if rootRoundNumber(t, f, got.ID) != 5 {
		t.Fatal("rollback consumed durable round")
	}
}
func TestRootRoundStoreRuntimeCannotRewriteOrDeleteHistory(t *testing.T) {
	f := newRecordFixture(t)
	rootRoundSchema(t, f)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	r := rootCatalogReserveInput(t, f, h)
	v := rootCatalogReserve(t, f, r)
	for _, sql := range []string{"UPDATE applications.workflow_rounds SET round_number=99 WHERE instance_id=$1", "DELETE FROM applications.workflow_rounds WHERE instance_id=$1", "UPDATE applications.workflow_instances SET previous_instance_id=$1 WHERE id=$1"} {
		_, e := f.runtime.Exec(f.ctx, sql, v.ID)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "42501" {
			t.Fatalf("runtime mutation not denied %v", e)
		}
	}
	if rootRoundNumber(t, f, v.ID) != 1 {
		t.Fatal("history changed")
	}
	// Owner cleanup models catalog removal only, not public deletion success.
	rootRoundTerminal(t, f, v.ID, "completed")
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.workflow_instances WHERE id=$1", v.ID); e != nil {
		t.Fatal(e)
	}
	if rootRoundNumber(t, f, v.ID) != 1 {
		t.Fatal("catalog removal deleted independent round history")
	}
}
