package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Additional schedules refine the already-RED cleanup/CAS/revocation oracles.
// Blocking is observed in pg_stat_activity, not inferred from sleeps.
func TestDraftQ36CleanupWaitsForNewerCommit(t *testing.T) {
	f := draftsFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d := requireDraft(t, f, draftInput())
	saver, e := f.app.write(ctx, f.actor)
	if e != nil {
		t.Fatal(e)
	}
	defer saver.Rollback(context.Background())
	if _, e = saver.Exec(ctx, `UPDATE personnel.drafts SET payload_json='{"name":"new tab","parentId":null}',draft_version=2 WHERE id=$1 AND owner_user_id=$2`, d.ID, f.actor.UserID); e != nil {
		t.Fatal(e)
	}
	var pid int32
	if e = saver.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); e != nil {
		t.Fatal(e)
	}
	result := make(chan error, 1)
	go func() {
		tx, e := f.app.write(ctx, f.actor)
		if e != nil {
			result <- e
			return
		}
		defer tx.Rollback(context.Background())
		removed, e := CleanupDraft(ctx, tx, f.actor, &DraftReference{d.ID, 1}, DraftDepartment, nil)
		if e == nil && removed {
			e = errors.New("cleanup deleted newer uncommitted save")
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
		result <- e
	}()
	awaitBlockedBy(t, f, pid)
	if e = saver.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = <-result; e != nil {
		t.Fatal(e)
	}
	got, e := f.app.GetDraft(ctx, f.actor, d.ID)
	if e != nil || got.Version != 2 || !strings.Contains(string(got.Payload), "new tab") {
		t.Fatalf("RC recheck must preserve newer draft: %v", e)
	}
}
func TestDraftQ36SaveHoldsAuthorizationUntilCommit(t *testing.T) {
	f := draftsFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d := requireDraft(t, f, draftInput())
	blocker, e := f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer blocker.Rollback(context.Background())
	if _, e = blocker.Exec(ctx, "SELECT id FROM personnel.drafts WHERE id=$1 FOR UPDATE", d.ID); e != nil {
		t.Fatal(e)
	}
	var pid int32
	if e = blocker.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); e != nil {
		t.Fatal(e)
	}
	saved := make(chan error, 1)
	go func() {
		_, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"name":"authorized save","parentId":null}`)})
		saved <- e
	}()
	awaitBlockedBy(t, f, pid)
	revoker, e := f.owner.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		t.Fatal(e)
	}
	defer revoker.Rollback(context.Background())
	// Match actual permission-revocation lock: catalog is locked by AuthorizeWrite.
	if _, e = revoker.Exec(ctx, "SET LOCAL lock_timeout='100ms'"); e != nil {
		t.Fatal(e)
	}
	_, e = revoker.Exec(ctx, "UPDATE personnel.permission_catalog SET enabled=false WHERE code='personnel.manage'")
	var locked *pgconn.PgError
	if !errors.As(e, &locked) || locked.Code != "55P03" {
		t.Fatalf("revoker must wait on actual authorization lock: %v", e)
	}
	_ = revoker.Rollback(ctx)
	if e = blocker.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = <-saved; e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template); e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{2, draftInput().Payload}); !errors.Is(e, ErrDenied) {
		t.Fatalf("later save must reject revoke: %v", e)
	}
}
func TestDraftQ36UnicodeCanonicalBoundaryAndOriginalBase(t *testing.T) {
	f := draftsFixture(t)
	ctx := context.Background()
	exact := strings.Replace(string(draftBoundary(65534)), "x", "界", 1)
	if len(exact) != 65536 {
		t.Fatal("UTF-8 fixture exact bytes")
	}
	d := requireDraft(t, f, DraftCreateInput{Kind: DraftTemplate, Payload: json.RawMessage(exact)})
	if len(d.Payload) != 65536 {
		t.Fatal("canonical UTF-8 boundary")
	}
	tooBig := strings.Replace(string(draftBoundary(65536)), "x", "界", 1)
	if _, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(tooBig)}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("size counted by UTF-8 bytes: %v", e)
	}
	// Encoded source exceeds 64KiB but its canonical representation remains exact.
	escaped := strings.Replace(exact, "界", `\u754c`, 1) + strings.Repeat(" ", 200)
	requireDraft(t, f, DraftCreateInput{Kind: DraftTemplate, Payload: json.RawMessage(escaped)})
	// A removed reference and original business version remain explicit inputs.
	target := "00000000-0000-4000-8000-000000000088"
	base := int64(7)
	in := draftInput()
	in.TargetID = &target
	in.BaseVersion = &base
	d = requireDraft(t, f, in)
	next, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"parentId":"00000000-0000-4000-8000-000000000099","name":"<>&\ud83d\ude00"}`)})
	if e != nil || next.BaseVersion == nil || *next.BaseVersion != 7 || next.TargetID == nil || *next.TargetID != target {
		t.Fatalf("payload update must retain original base/target: %v", e)
	}
	if string(next.Payload) != `{"name":"<>&😀","parentId":"00000000-0000-4000-8000-000000000099"}` {
		t.Fatal("valid Unicode pair and HTML chars canonicalized losslessly")
	}
	if _, e = f.owner.Exec(ctx, "UPDATE personnel.drafts SET draft_version=9007199254740991 WHERE id=$1", d.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{maxSafeVersion, in.Payload}); !errors.Is(e, ErrDraftConflict) {
		t.Fatalf("safe version cannot overflow: %v", e)
	}
}

// OpenAPI uniqueItems compares JSON strings. Drafts preserve user inputs;
// actual business submission independently normalizes UUID relation sets.
func TestDraftQ36UniqueItemsPreservesUUIDSpellings(t *testing.T) {
	f := draftsFixture(t)
	target := f.actor.UserID
	base := int64(0)
	payload := json.RawMessage(`{"identityIds":["00000000-0000-4000-8000-0000000000ab","00000000-0000-4000-8000-0000000000AB"]}`)
	d := requireDraft(t, f, DraftCreateInput{Kind: DraftMemberIdentities, TargetID: &target, BaseVersion: &base, Payload: payload})
	if string(d.Payload) != string(payload) {
		t.Fatal("case-distinct valid JSON values are preserved per uniqueItems")
	}
}
