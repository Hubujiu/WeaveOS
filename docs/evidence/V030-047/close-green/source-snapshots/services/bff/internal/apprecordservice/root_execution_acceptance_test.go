package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"sync"
	"testing"
)

func rootRecoveryCommand(t *testing.T, f rootProjection) (fc.Command, fc.ExecutionPayload) {
	t.Helper()
	c := fc.Command{ProtocolVersion: 2, CommandID: recordOperationID(t, f.recordFixture), AppID: f.app, TableID: f.table, ViewID: f.view, RecordID: f.ownRecord, InstanceID: f.instance.ID, ActorID: f.actor, Action: "start", RecordVersion: 1, SchemaVersion: 1, FenceEpoch: 1, FlowID: f.head.FlowID, DefinitionVersion: 1, VersionID: f.version}
	p := fc.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-durable-evidence:" + c.CommandID)), Start: &fc.ExecutionStart{AllowWithdraw: true, Approvers: map[string][]string{f.first: {f.actor}, f.second: {f.actor, f.other}}}, Routes: map[string]bool{}}
	raw, e := fc.EncodeExecutionPayload(c.Action, p)
	if e != nil {
		t.Fatal(e)
	}
	c.PayloadHash = sha256.Sum256(raw)
	// Keep unrelated test commands outside subsequent worker fixtures' due window.
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), "UPDATE applications.workflow_dispatch SET next_attempt_at='9999-12-31T00:00:00Z' WHERE command_id=$1", c.CommandID)
	})
	return c, p
}
func rootRecoveryAccept(t *testing.T, f rootProjection) (fc.Command, fc.ExecutionPayload) {
	t.Helper()
	c, p := rootRecoveryCommand(t, f)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if e = tx.QueryRow(f.ctx, rootAcquireSQL, f.app, f.table, f.view, f.ownRecord, c.CommandID, int64(1), int64(1)).Scan(&c.FenceEpoch); e != nil {
		t.Fatal(e)
	}
	got, e := (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, c, p)
	if e != nil || got.Command != c || got.State != "pending" {
		t.Fatalf("durable accept: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	return c, p
}
func rootRecoveryPayload(t *testing.T, f rootProjection, c fc.Command) fc.ExecutionPayload {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	p, e := (fc.Ledger{Namespace: "applications"}).ExecutionPayloadInTx(f.ctx, tx, c)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestRootRecoveryAcceptPersistsOriginalPayloadAndSchedulingIdentity(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := rootRecoveryAccept(t, f)
	var raw []byte
	var protocol, attempts int
	var token *string
	var pending bool
	e := f.owner.QueryRow(f.ctx, `SELECT c.execution_payload,d.protocol_version,d.attempts,d.lease_token::text,d.lease_until IS NULL AND d.last_error IS NULL FROM applications.workflow_commands c JOIN applications.workflow_dispatch d USING(command_id) WHERE c.command_id=$1`, c.CommandID).Scan(&raw, &protocol, &attempts, &token, &pending)
	want, _ := fc.EncodeExecutionPayload(c.Action, p)
	if e != nil || !bytes.Equal(raw, want) || protocol != 2 || attempts != 0 || token != nil || !pending {
		t.Fatalf("original durable inputs not atomic: %v", e)
	}
	f.ledger(t, c, "pending", 1, 1)
	got := rootRecoveryPayload(t, f, c)
	again, _ := fc.EncodeExecutionPayload(c.Action, got)
	if !bytes.Equal(again, want) {
		t.Fatal("stored payload reconstruction changed original bytes")
	}
	got.Start.Approvers[f.first][0] = f.other
	got.Routes[f.first] = true
	next := rootRecoveryPayload(t, f, c)
	again, _ = fc.EncodeExecutionPayload(c.Action, next)
	if !bytes.Equal(again, want) {
		t.Fatal("returned payload mutation changed durable input")
	}
}
func TestRootRecoveryAcceptCallerRollbackDropsPayloadLedgerQueueAndFence(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := rootRecoveryCommand(t, f)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if e = tx.QueryRow(f.ctx, rootAcquireSQL, f.app, f.table, f.view, f.ownRecord, c.CommandID, int64(1), int64(1)).Scan(&c.FenceEpoch); e != nil {
		t.Fatal(e)
	}
	if _, e = (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, c, p); e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"workflow_commands", "workflow_dispatch", "record_command_fences"} {
		var n int
		if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE command_id=$1", c.CommandID).Scan(&n); e != nil || n != 0 {
			t.Fatalf("%s survived caller rollback: %d %v", table, n, e)
		}
	}
}
func TestRootRecoveryAcceptSavepointPreventsPartialCommandAfterQueueFailure(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := rootRecoveryCommand(t, f)
	fn := "root_recovery_" + strings.ReplaceAll(c.CommandID, "-", "")
	sql := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root recovery dispatch fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.workflow_dispatch FOR EACH ROW WHEN (NEW.command_id='%s'::uuid) EXECUTE FUNCTION applications.%s()", fn, fn, c.CommandID, fn)
	if _, e := f.owner.Exec(f.ctx, sql); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER %s ON applications.workflow_dispatch; DROP FUNCTION applications.%s()", fn, fn))
	})
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	_, e = (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, c, p)
	var fault *pgconn.PgError
	if !errors.As(e, &fault) || fault.Code != "P0001" || !strings.Contains(fault.Message, "root recovery dispatch fault") {
		t.Fatalf("did not reach real queue failure: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatalf("savepoint did not restore outer transaction: %v", e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_id=$1", c.CommandID).Scan(&n); e != nil || n != 0 {
		t.Fatalf("partial command survived failed accept: %v", e)
	}
}
func TestRootRecoveryAcceptExactReplayConflictAndTerminalReplay(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := rootRecoveryAccept(t, f)
	l := fc.Ledger{Namespace: "applications"}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	got, e := l.AcceptExecutionInTx(f.ctx, tx, c, p)
	if e != nil || got.State != "pending" {
		t.Fatalf("pending replay rejected: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	changed := p
	changed.Routes = map[string]bool{f.first: true}
	raw, _ := fc.EncodeExecutionPayload(c.Action, changed)
	other := c
	other.PayloadHash = sha256.Sum256(raw)
	tx, e = f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, e = l.AcceptExecutionInTx(f.ctx, tx, other, changed)
	if !errors.Is(e, fc.ErrConflict) {
		t.Fatalf("same key with different original input accepted: %v", e)
	}
	_ = tx.Rollback(f.ctx)
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	f.apply(t, c, p, r, b, true)
	tx, e = f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	got, e = l.AcceptExecutionInTx(f.ctx, tx, c, p)
	if e != nil || got.State != "success" || got.Receipt == nil || *got.Receipt != r {
		t.Fatalf("terminal replay lost original result: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.ledger(t, c, "success", 0, 0)
	persisted := rootRecoveryPayload(t, f, c)
	original, _ := fc.EncodeExecutionPayload(c.Action, p)
	raw, _ = fc.EncodeExecutionPayload(c.Action, persisted)
	if !bytes.Equal(raw, original) {
		t.Fatal("finalization discarded recovery evidence")
	}
}
func TestRootRecoveryLegacyV2CannotBePromotedOrHavePayloadGuessed(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	l := fc.Ledger{Namespace: "applications"}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = l.AcceptExecutionInTx(f.ctx, tx, c, p); !errors.Is(e, fc.ErrConflict) {
		t.Fatalf("legacy accepted command was rewritten: %v", e)
	}
	if _, e = l.ExecutionPayloadInTx(f.ctx, tx, c); !errors.Is(e, fc.ErrConflict) {
		t.Fatalf("legacy payload was invented: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	var empty bool
	var protocol int
	if e = f.owner.QueryRow(f.ctx, "SELECT c.execution_payload IS NULL,d.protocol_version FROM applications.workflow_commands c JOIN applications.workflow_dispatch d USING(command_id) WHERE c.command_id=$1", c.CommandID).Scan(&empty, &protocol); e != nil || !empty || protocol != 1 {
		t.Fatalf("legacy storage changed: %v", e)
	}
}
func TestRootRecoveryPayloadBindingCorruptionAndMissingCommand(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := rootRecoveryAccept(t, f)
	l := fc.Ledger{Namespace: "applications"}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	wrong := c
	wrong.RecordVersion++
	if _, e = l.ExecutionPayloadInTx(f.ctx, tx, wrong); !errors.Is(e, fc.ErrConflict) {
		t.Fatalf("wrong command binding read payload: %v", e)
	}
	missing := c
	missing.CommandID = recordOperationID(t, f.recordFixture)
	if _, e = l.ExecutionPayloadInTx(f.ctx, tx, missing); !errors.Is(e, fc.ErrMissing) {
		t.Fatalf("missing command not distinguished: %v", e)
	}
	_ = tx.Rollback(f.ctx)
	raw, _ := fc.EncodeExecutionPayload(c.Action, p)
	raw[8] ^= 1
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_commands SET execution_payload=$2 WHERE command_id=$1", c.CommandID, raw); e != nil {
		t.Fatal(e)
	}
	tx, e = f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = l.ExecutionPayloadInTx(f.ctx, tx, c); !errors.Is(e, fc.ErrConflict) {
		t.Fatalf("payload hash corruption was accepted: %v", e)
	}
}
func TestRootRecoveryPayloadImmutableRuntimeAndNarrowSchedulerGrants(t *testing.T) {
	f := rootProjectionFixture(t)
	c, _ := rootRecoveryAccept(t, f)
	for _, q := range []string{
		"UPDATE applications.workflow_commands SET execution_payload=NULL WHERE command_id=$1",
		"UPDATE applications.workflow_dispatch SET protocol_version=1 WHERE command_id=$1",
		"UPDATE applications.workflow_dispatch SET created_at=now() WHERE command_id=$1",
	} {
		_, e := f.runtime.Exec(f.ctx, q, c.CommandID)
		var denied *pgconn.PgError
		if !errors.As(e, &denied) || denied.Code != "42501" {
			t.Fatalf("immutable column was writable: %v", e)
		}
	}
	token := recordOperationID(t, f.recordFixture)
	tag, e := f.runtime.Exec(f.ctx, "UPDATE applications.workflow_dispatch SET attempts=1,lease_token=$2,lease_until=now()+interval '45 seconds',next_attempt_at=now()+interval '45 seconds',last_error='DEPENDENCY_UNAVAILABLE' WHERE command_id=$1", c.CommandID, token)
	if e != nil || tag.RowsAffected() != 1 {
		t.Fatalf("limited scheduler metadata is not writable: %v", e)
	}
}
func TestRootRecoveryConcurrentExactAcceptanceKeepsOneQueue(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := rootRecoveryCommand(t, f)
	fenceTx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer fenceTx.Rollback(f.ctx)
	if e = fenceTx.QueryRow(f.ctx, rootAcquireSQL, f.app, f.table, f.view, f.ownRecord, c.CommandID, int64(1), int64(1)).Scan(&c.FenceEpoch); e != nil {
		t.Fatal(e)
	}
	if e = fenceTx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				failures <- e
				return
			}
			defer tx.Rollback(f.ctx)
			got, e := (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, c, p)
			if e == nil && (got.Command != c || got.State != "pending") {
				e = fmt.Errorf("wrong replay identity")
			}
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			failures <- e
		}()
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	f.ledger(t, c, "pending", 1, 1)
}
