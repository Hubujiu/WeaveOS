package apprecordservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowprojection"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type rootCloseSnapshot struct {
	workflowcatalog.Head
	CloseEpoch int64
}

func rootCloseHead(t *testing.T, f rootTaskFixture) rootCloseSnapshot {
	t.Helper()
	var h rootCloseSnapshot
	rootCatalogTx(t, f.recordFixture, func(tx pgx.Tx) error {
		var e error
		h.Head, e = (workflowcatalog.Catalog{}).GetInTx(f.ctx, tx, f.app, f.head.FlowID)
		if e != nil {
			return e
		}
		return tx.QueryRow(f.ctx, "SELECT close_epoch FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID).Scan(&h.CloseEpoch)
	})
	return h
}
func rootRequestClose(t *testing.T, f rootTaskFixture) rootCloseSnapshot {
	t.Helper()
	h := rootCloseHead(t, f)
	rootCatalogTx(t, f.recordFixture, func(tx pgx.Tx) error {
		var e error
		h.Head, e = (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, f.head.FlowID, h.Revision)
		if e != nil {
			return e
		}
		return tx.QueryRow(f.ctx, "SELECT close_epoch FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID).Scan(&h.CloseEpoch)
	})
	if h.State != "closing" {
		t.Fatalf("fixture has active instance but close state=%s", h.State)
	}
	return h
}
func rootCheckClosed(t *testing.T, f rootTaskFixture, before rootCloseSnapshot) {
	t.Helper()
	h := rootCloseHead(t, f)
	if h.State != "disabled" || h.Revision != before.Revision+1 || h.CloseEpoch != before.CloseEpoch {
		t.Fatalf("last confirmed instance did not finalize exactly one close: before=%+v after=%+v", before, h)
	}
}
func TestRootWorkflowCloseFinalizesWithLastConfirmedCompletion(t *testing.T) {
	f := rootTaskSetup(t, false)
	r := rootAction(t, f, rootActionRequest(t, f, "agree"))
	c, p := rootActionRead(t, f, r)
	before := rootRequestClose(t, f)
	receipt, body := f.receipt(t, c, "completed", "")
	f.apply(t, c, p, receipt, body, true)
	rootCheckClosed(t, f, before)
	f.state(t, "completed", 2, 2, 0, 1)
	if got := f.apply(t, c, p, receipt, body, true); !got.Duplicate {
		t.Fatal("replay was not duplicate")
	}
	rootCheckClosed(t, f, before)
}
func TestRootWorkflowCloseDoesNotDisableWithoutCloseRequest(t *testing.T) {
	f := rootTaskSetup(t, false)
	before := rootCloseHead(t, f)
	r := rootAction(t, f, rootActionRequest(t, f, "agree"))
	c, p := rootActionRead(t, f, r)
	receipt, body := f.receipt(t, c, "completed", "")
	f.apply(t, c, p, receipt, body, true)
	after := rootCloseHead(t, f)
	if after.State != "enabled" || after.Revision != before.Revision {
		t.Fatalf("completion invented close request: %+v", after)
	}
}
func TestRootWorkflowCloseCountsAcceptedStartingUntilConfirmedNoEffect(t *testing.T) {
	f := rootTaskSetup(t, false)
	second := f.rootProjection
	second.instance = rootCatalogReserve(t, f.recordFixture, rootCatalogReserveInput(t, f.recordFixture, f.head))
	r := rootAction(t, f, rootActionRequest(t, f, "agree"))
	c, p := rootActionRead(t, f, r)
	before := rootRequestClose(t, f)
	receipt, body := f.receipt(t, c, "completed", "")
	f.apply(t, c, p, receipt, body, true)
	h := rootCloseHead(t, f)
	if h.State != "closing" || h.Revision != before.Revision {
		t.Fatal("accepted starting intent was omitted from drain")
	}
	start, payload := rootRecoveryAccept(t, second)
	cancelled, raw := second.receipt(t, start, "unchanged", "cancelled")
	second.apply(t, start, payload, cancelled, raw, true)
	rootCheckClosed(t, f, before)
}
func TestRootWorkflowCloseOuterRollbackKeepsPendingAndClosing(t *testing.T) {
	f := rootTaskSetup(t, false)
	r := rootAction(t, f, rootActionRequest(t, f, "reject"))
	c, p := rootActionRead(t, f, r)
	before := rootRequestClose(t, f)
	receipt, body := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, receipt, body, false)
	h := rootCloseHead(t, f)
	if h.State != "closing" || h.Revision != before.Revision {
		t.Fatal("uncommitted result completed closure")
	}
	f.state(t, "active", 1, 1, 1, 1)
	rootActionCount(t, f, 1, 1, 1)
	f.apply(t, c, p, receipt, body, true)
	rootCheckClosed(t, f, before)
}
func TestRootWorkflowCloseWriteFaultRollsBackProjectionForRecovery(t *testing.T) {
	f := rootTaskSetup(t, false)
	r := rootAction(t, f, rootActionRequest(t, f, "agree"))
	c, p := rootActionRead(t, f, r)
	before := rootRequestClose(t, f)
	receipt, body := f.receipt(t, c, "completed", "")
	name := "root_close_" + strings.ReplaceAll(f.app, "-", "")
	sql := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root close finalization fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER UPDATE ON applications.workflow_definitions FOR EACH ROW WHEN (NEW.app_id='%s' AND NEW.state='disabled') EXECUTE FUNCTION applications.%s()", name, name, f.app, name)
	if _, e := f.owner.Exec(f.ctx, sql); e != nil {
		t.Fatal(e)
	}
	cleanup := func() {
		_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.workflow_definitions; DROP FUNCTION IF EXISTS applications.%s()", name, name))
	}
	t.Cleanup(cleanup)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	_, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, receipt, body)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "P0001" || !strings.Contains(pg.Message, "root close finalization fault") {
		t.Fatalf("did not reach real close write failure: %v", e)
	}
	// The projection savepoint must protect state even if its caller commits.
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.state(t, "active", 1, 1, 1, 1)
	rootActionCount(t, f, 1, 1, 1)
	h := rootCloseHead(t, f)
	if h.State != "closing" || h.Revision != before.Revision {
		t.Fatal("failed closure leaked a catalog change")
	}
	cleanup()
	f.apply(t, c, p, receipt, body, true)
	rootCheckClosed(t, f, before)
}
