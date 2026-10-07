package apprecordservice

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"sync"
	"testing"
)

func rootTriggerReserve(t *testing.T, f recordFixture, in workflowcatalog.ReserveInput) workflowcatalog.TriggerReservation {
	t.Helper()
	var out workflowcatalog.TriggerReservation
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		out, e = (workflowcatalog.Catalog{}).ReserveTriggeredInTx(f.ctx, tx, in)
		return e
	})
	return out
}
func TestRootTriggerReservationFirstAndDuplicate(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	in := rootCatalogReserveInput(t, f, h)
	a := rootTriggerReserve(t, f, in)
	if a.Ignored || a.Instance.ID != in.InstanceID || a.Instance.State != "starting" {
		t.Fatalf("first %+v", a)
	}
	in.InstanceID = recordOperationID(t, f)
	b := rootTriggerReserve(t, f, in)
	if !b.Ignored || b.Instance.ID != a.Instance.ID {
		t.Fatalf("duplicate %+v", b)
	}
	var n int
	if e := f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3", f.app, h.FlowID, f.ownRecord).Scan(&n); e != nil || n != 1 {
		t.Fatalf("duplicate inserted %d %v", n, e)
	}
}
func TestRootTriggerReservationReplayAndIdentityConflict(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	in := rootCatalogReserveInput(t, f, h)
	a := rootTriggerReserve(t, f, in)
	b := rootTriggerReserve(t, f, in)
	if b.Ignored || b.Instance.ID != a.Instance.ID {
		t.Fatalf("exact replay %+v", b)
	}
	in.RecordID = f.otherRecord
	rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ReserveTriggeredInTx(f.ctx, tx, in)
		return e
	})
}
func TestRootTriggerReservationIndependentFlowsAndRecords(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	first := rootTriggerReserve(t, f, rootCatalogReserveInput(t, f, h))
	in := rootCatalogReserveInput(t, f, h)
	in.RecordID = f.otherRecord
	other := rootTriggerReserve(t, f, in)
	h2 := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	b := rootTriggerReserve(t, f, rootCatalogReserveInput(t, f, h2))
	if other.Ignored || b.Ignored || first.Instance.ID == other.Instance.ID || first.Instance.ID == b.Instance.ID {
		t.Fatal("independent flows or records collapsed")
	}
}
func TestRootTriggerReservationConcurrentExactlyOne(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	const n = 8
	ins := make([]workflowcatalog.ReserveInput, n)
	for i := range ins {
		ins[i] = rootCatalogReserveInput(t, f, h)
	}
	start := make(chan struct{})
	results := make(chan workflowcatalog.TriggerReservation, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for _, in := range ins {
		wg.Add(1)
		go func(in workflowcatalog.ReserveInput) {
			defer wg.Done()
			<-start
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				errs <- e
				return
			}
			defer tx.Rollback(f.ctx)
			v, e := (workflowcatalog.Catalog{}).ReserveTriggeredInTx(f.ctx, tx, in)
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			if e != nil {
				errs <- e
				return
			}
			results <- v
		}(in)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		t.Errorf("concurrent reservation: %v", e)
	}
	created := 0
	ids := map[string]bool{}
	for r := range results {
		if !r.Ignored {
			created++
		}
		ids[r.Instance.ID] = true
	}
	if created != 1 || len(ids) != 1 {
		t.Fatalf("created=%d identities=%v", created, ids)
	}
}
func TestRootTriggerReservationClosingRejectsNewAndCountsStarting(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	rootTriggerReserve(t, f, rootCatalogReserveInput(t, f, h))
	var closed workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		closed, e = (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	if closed.State != "closing" {
		t.Fatalf("starting intent not in drain %+v", closed)
	}
	in := rootCatalogReserveInput(t, f, closed)
	rootCatalogWantError(t, f, workflowcatalog.ErrClosing, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ReserveTriggeredInTx(f.ctx, tx, in)
		return e
	})
}
func TestRootTriggerReservationCallerRollbackAndStaleVersion(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	in := rootCatalogReserveInput(t, f, h)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	v, e := (workflowcatalog.Catalog{}).ReserveTriggeredInTx(f.ctx, tx, in)
	if e != nil {
		t.Fatal(e)
	}
	if v.Ignored {
		t.Fatal("first unexpectedly ignored")
	}
	if _, e = tx.Exec(f.ctx, "SELECT 1/0"); e == nil {
		t.Fatal("injection did not fail")
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE id=$1", in.InstanceID).Scan(&count); e != nil || count != 0 {
		t.Fatalf("partial commit %d %v", count, e)
	}
	rootTriggerReserve(t, f, in)
	in.InstanceID = recordOperationID(t, f)
	in.ExpectedRecordVersion = 2
	rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ReserveTriggeredInTx(f.ctx, tx, in)
		return e
	})
}

