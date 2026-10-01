package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Oracle: approved PLAN §5, contracts/query-drafts.md, frozen OpenAPI draft schemas.
// Every fixture uses real PG18 and SET ROLE auth_app; synthetic inputs are not a mock store.
func draftsFixture(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	t.Cleanup(func() {
		if _, err := f.owner.Exec(context.Background(), "DELETE FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID); err != nil {
			t.Error(err)
		}
	})
	return f
}
func draftInput() DraftCreateInput {
	return DraftCreateInput{Kind: DraftDepartment, Payload: json.RawMessage(`{"name":"","parentId":null}`)}
}
func requireDraft(t *testing.T, f *fixture, in DraftCreateInput) PersonnelDraft {
	t.Helper()
	d, e := f.app.CreateDraft(context.Background(), f.actor, in)
	if e != nil {
		t.Fatalf("explicit incomplete save must succeed: %v", e)
	}
	return d
}
func TestDraftQ36LifecycleAndIsolation(t *testing.T) {
	f := draftsFixture(t)
	other := draftsFixture(t)
	ctx := context.Background()
	d := requireDraft(t, f, draftInput())
	if d.ID == "" || d.Version != 1 || d.BaseVersion != nil || d.TargetID != nil || d.CreatedAt.IsZero() {
		t.Fatal("new draft initial metadata")
	}
	for _, op := range []func() error{
		func() error { _, e := other.app.GetDraft(ctx, other.actor, d.ID); return e },
		func() error {
			_, e := other.app.UpdateDraft(ctx, other.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"name":"stolen","parentId":null}`)})
			return e
		},
		func() error { return other.app.DeleteDraft(ctx, other.actor, d.ID, 1) },
	} {
		if e := op(); !errors.Is(e, ErrMissing) {
			t.Fatalf("owner isolation must hide foreign ID: %v", e)
		}
	}
	list, e := other.app.ListDrafts(ctx, other.actor)
	if e != nil || len(list.Items) != 0 {
		t.Fatalf("foreign drafts must not list: %v", e)
	}
	next, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"name":"next","parentId":null}`)})
	if e != nil || next.Version != 2 || next.BaseVersion != nil || !next.CreatedAt.Equal(d.CreatedAt) {
		t.Fatalf("payload-only CAS metadata: %v", e)
	}
	if e := f.app.DeleteDraft(ctx, f.actor, d.ID, 1); !errors.Is(e, ErrDraftConflict) {
		t.Fatalf("stale delete must conflict: %v", e)
	}
	if e := f.app.DeleteDraft(ctx, f.actor, d.ID, 2); e != nil {
		t.Fatal(e)
	}
	if _, e := f.app.GetDraft(ctx, f.actor, d.ID); !errors.Is(e, ErrMissing) {
		t.Fatalf("deleted draft absent: %v", e)
	}
}
func TestDraftQ36ConcurrentCapacityAndCAS(t *testing.T) {
	f := draftsFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := make(chan struct{})
	done := make(chan error, 40)
	for range 40 {
		go func() { <-start; _, e := f.app.CreateDraft(ctx, f.actor, draftInput()); done <- e }()
	}
	close(start)
	success, limit := 0, 0
	for range 40 {
		e := <-done
		if e == nil {
			success++
		} else if errors.Is(e, ErrDraftLimit) {
			limit++
		} else {
			t.Fatalf("quota must return domain error: %v", e)
		}
	}
	if success != 20 || limit != 20 {
		t.Fatalf("hard owner capacity: success=%d limit=%d", success, limit)
	}
	var count int
	if e := f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID).Scan(&count); e != nil || count != 20 {
		t.Fatalf("real PG quota: %d %v", count, e)
	}
	list, e := f.app.ListDrafts(ctx, f.actor)
	if e != nil || len(list.Items) != 20 {
		t.Fatal("all twenty summaries required")
	}
	d := list.Items[0]
	start = make(chan struct{})
	done = make(chan error, 2)
	for _, name := range []string{"tab-A", "tab-B"} {
		go func(name string) {
			<-start
			_, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(fmt.Sprintf(`{"name":%q,"parentId":null}`, name))})
			done <- e
		}(name)
	}
	close(start)
	success, conflict := 0, 0
	for range 2 {
		e := <-done
		if e == nil {
			success++
		} else if errors.Is(e, ErrDraftConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("two tabs: %d success %d conflicts", success, conflict)
	}
	if e := f.app.DeleteDraft(ctx, f.actor, d.ID, 2); e != nil {
		t.Fatal(e)
	}
	requireDraft(t, f, draftInput())
}
func TestDraftQ36LiveAuthorization(t *testing.T) {
	for _, mode := range []string{"permission", "disabled", "auth-version"} {
		t.Run(mode, func(t *testing.T) {
			f := draftsFixture(t)
			ctx := context.Background()
			d := requireDraft(t, f, draftInput())
			want := ErrDenied
			switch mode {
			case "permission":
				_, e := f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template)
				if e != nil {
					t.Fatal(e)
				}
			case "disabled":
				_, e := f.owner.Exec(ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor.UserID)
				if e != nil {
					t.Fatal(e)
				}
				want = session.ErrUnauthorized
			case "auth-version":
				_, e := f.owner.Exec(ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor.UserID)
				if e != nil {
					t.Fatal(e)
				}
				want = session.ErrUnauthorized
			}
			for _, op := range []func() error{
				func() error { _, e := f.app.ListDrafts(ctx, f.actor); return e }, func() error { _, e := f.app.GetDraft(ctx, f.actor, d.ID); return e }, func() error { _, e := f.app.CreateDraft(ctx, f.actor, draftInput()); return e }, func() error {
					_, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, draftInput().Payload})
					return e
				}, func() error { return f.app.DeleteDraft(ctx, f.actor, d.ID, 1) },
			} {
				if e := op(); !errors.Is(e, want) {
					t.Fatalf("live %s must block every operation: %v", mode, e)
				}
			}
			var count int
			if e := f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.drafts WHERE owner_user_id=$1 AND draft_version=1", f.actor.UserID).Scan(&count); e != nil || count != 1 {
				t.Fatal("denied operations changed draft")
			}
		})
	}
}
func TestDraftQ36VariantsAndStrictPayload(t *testing.T) {
	f := draftsFixture(t)
	ctx := context.Background()
	target := "00000000-0000-4000-8000-000000000099"
	zero := int64(0)
	valid := []DraftCreateInput{
		{DraftMemberIdentities, &target, &zero, json.RawMessage(`{"identityIds":[]}`)},
		{DraftMemberGroups, &target, &zero, json.RawMessage(`{"operation":"move","departmentId":null,"sourceDepartmentId":null}`)},
		draftInput(),
		{Kind: DraftIdentity, Payload: json.RawMessage(`{"name":"","description":"","templateIds":[],"permissionCodes":[""]}`)},
		{Kind: DraftTemplate, Payload: json.RawMessage(`{"name":"","description":"","permissionCodes":[]}`)},
	}
	for _, in := range valid {
		d := requireDraft(t, f, in)
		if string(d.Payload) == "" {
			t.Fatal("payload detail required")
		}
		if e := f.app.DeleteDraft(ctx, f.actor, d.ID, 1); e != nil {
			t.Fatal(e)
		}
	}
	bad := []string{`{"name":"","parentId":null,"password":"secret"}`, `{"name":"","parentId":null,"session":"secret"}`, `{"name":"","parentId":null,"inviteCode":"secret"}`, `{"name":"","parentId":null,"metadata":{}}`, `{"name":"a","name":"b","parentId":null}`, `{"name":"a"}`, `{"name":null,"parentId":null}`, `{"name":{},"parentId":null}`, `{"name":"a","parentId":"missing"}`, `[]`, `null`, `{"name":"a","parentId":null} {}`, `{"name":"\ud800","parentId":null}`, `{"name":"\u0000","parentId":null}`, `{"name":"` + strings.Repeat("a", 101) + `","parentId":null}`}
	bad = append(bad, string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', ',', '"', 'p', 'a', 'r', 'e', 'n', 't', 'I', 'd', '"', ':', 'n', 'u', 'l', 'l', '}'}))
	for i, payload := range bad {
		in := draftInput()
		in.Payload = json.RawMessage(payload)
		if _, e := f.app.CreateDraft(ctx, f.actor, in); !errors.Is(e, ErrInvalid) {
			t.Fatalf("reject unsafe/malformed payload %d: %v", i, e)
		}
	}
	for _, in := range []DraftCreateInput{{Kind: "password", Payload: json.RawMessage(`{}`)}, {Kind: DraftMemberIdentities, Payload: json.RawMessage(`{"identityIds":[]}`)}, {DraftDepartment, &target, &zero, draftInput().Payload}, {Kind: DraftDepartment, TargetID: &target, Payload: draftInput().Payload}, {Kind: DraftMemberIdentities, TargetID: &target, BaseVersion: &zero, Payload: json.RawMessage(`{"identityIds":["00000000-0000-4000-8000-000000000001","00000000-0000-4000-8000-000000000001"]}`)}, {Kind: DraftTemplate, Payload: json.RawMessage(`{"name":"","description":"","permissionCodes":null}`)}} {
		if _, e := f.app.CreateDraft(ctx, f.actor, in); !errors.Is(e, ErrInvalid) {
			t.Fatalf("kind/target/base/variant invalid: %v", e)
		}
	}
	d := requireDraft(t, f, draftInput())
	if _, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"identityIds":[]}`)}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("persisted kind selects update variant: %v", e)
	}
}

// Construct exact canonical boundary independently of the service serializer.
func draftBoundary(size int) json.RawMessage {
	codes := make([]string, 401)
	for i := range codes {
		codes[i] = fmt.Sprintf("%03d", i) + strings.Repeat("x", 157)
	}
	prefix := `{"description":"","name":"","permissionCodes":[`
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = `"` + c + `"`
	}
	body := prefix + strings.Join(parts, ",") + `,"` + strings.Repeat("z", size-len(prefix+strings.Join(parts, ",")+`,""`+`]}`)) + `"]}`
	return json.RawMessage(body)
}
func TestDraftQ36CanonicalBytes(t *testing.T) {
	f := draftsFixture(t)
	ctx := context.Background()
	exact := draftBoundary(65536)
	if len(exact) != 65536 {
		t.Fatal("independent exact fixture")
	}
	d := requireDraft(t, f, DraftCreateInput{Kind: DraftTemplate, Payload: exact})
	var stored string
	if e := f.owner.QueryRow(ctx, "SELECT payload_json FROM personnel.drafts WHERE id=$1", d.ID).Scan(&stored); e != nil || stored != string(exact) {
		t.Fatalf("canonical exact 65536 bytes persists: %v", e)
	}
	if _, e := f.app.CreateDraft(ctx, f.actor, DraftCreateInput{Kind: DraftTemplate, Payload: draftBoundary(65537)}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("65537 canonical bytes must reject: %v", e)
	}
	// Large source escapes/whitespace are not counted as canonical UTF-8 size.
	escaped := ` { "parentId" : null, "name" : "` + strings.Repeat(`\u754c`, 100) + `" } `
	in := draftInput()
	in.Payload = json.RawMessage(escaped)
	d = requireDraft(t, f, in)
	if string(d.Payload) != `{"name":"`+strings.Repeat("界", 100)+`","parentId":null}` {
		t.Fatal("canonical unescaped UTF-8")
	}
}
func TestDraftQ36PreserveTargetBaseAndNoBusinessEffects(t *testing.T) {
	f := draftsFixture(t)
	ctx := context.Background()
	var target string
	if e := f.owner.QueryRow(ctx, "INSERT INTO personnel.departments(name,parent_id) VALUES('draft-target',$1) RETURNING id::text", rootDepartment(t, f)).Scan(&target); e != nil {
		t.Fatal(e)
	}
	defer f.owner.Exec(ctx, "DELETE FROM personnel.departments WHERE id=$1", target)
	var revisions string
	var audits int
	if e := f.owner.QueryRow(ctx, "SELECT string_agg(scope||':'||revision,',' ORDER BY scope) FROM personnel.query_revisions").Scan(&revisions); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events").Scan(&audits); e != nil {
		t.Fatal(e)
	}
	base := int64(1)
	in := draftInput()
	in.TargetID = &target
	in.BaseVersion = &base
	var configurationRows int
	if e := f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.member_configuration WHERE user_id=$1", f.actor.UserID).Scan(&configurationRows); e != nil || configurationRows != 0 {
		t.Fatalf("fixture must start before authorization initializes version=0: rows=%d error=%v", configurationRows, e)
	}
	d := requireDraft(t, f, in)
	var authorizationVersion int64
	if e := f.owner.QueryRow(ctx, "SELECT version FROM personnel.member_configuration WHERE user_id=$1", f.actor.UserID).Scan(&authorizationVersion); e != nil || authorizationVersion != 0 {
		t.Fatalf("authorization initialization must create only internal version=0: %d %v", authorizationVersion, e)
	}
	var afterCreate string
	if e := f.owner.QueryRow(ctx, "SELECT string_agg(scope||':'||revision,',' ORDER BY scope) FROM personnel.query_revisions").Scan(&afterCreate); e != nil || afterCreate != revisions {
		t.Fatalf("draft save plus initial authorization row must not increment revisions: %v", e)
	}
	if _, e := f.owner.Exec(ctx, "DELETE FROM personnel.departments WHERE id=$1", target); e != nil {
		t.Fatal(e)
	}
	// This is a genuine business deletion. Rebaseline only after it so future
	// revision triggers are allowed to record it, then isolate draft-only effects.
	if e := f.owner.QueryRow(ctx, "SELECT string_agg(scope||':'||revision,',' ORDER BY scope) FROM personnel.query_revisions").Scan(&revisions); e != nil {
		t.Fatal(e)
	}
	next, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"name":"keep inputs","parentId":null}`)})
	if e != nil || next.BaseVersion == nil || *next.BaseVersion != 1 || next.TargetID == nil || *next.TargetID != target {
		t.Fatalf("deleted target cannot rebase or discard draft: %v", e)
	}
	got, e := f.app.GetDraft(ctx, f.actor, d.ID)
	if e != nil || got.Version != 2 {
		t.Fatal("draft survives target deletion")
	}
	var after string
	var afterAudits int
	_ = f.owner.QueryRow(ctx, "SELECT string_agg(scope||':'||revision,',' ORDER BY scope) FROM personnel.query_revisions").Scan(&after)
	_ = f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events").Scan(&afterAudits)
	if revisions != after || audits != afterAudits {
		t.Fatal("draft writes must not touch business revisions or audit")
	}
	list, e := f.app.ListDrafts(ctx, f.actor)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "payload") || strings.Contains(string(raw), "expires") {
		t.Fatal("summary only; no expiry")
	}
	if e := f.app.DeleteDraft(ctx, f.actor, d.ID, 2); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(ctx, "SELECT string_agg(scope||':'||revision,',' ORDER BY scope) FROM personnel.query_revisions").Scan(&after); e != nil || after != revisions {
		t.Fatalf("draft deletion must not increment business revisions: %v", e)
	}
}
func TestDraftQ36ExactTransactionalCleanup(t *testing.T) {
	f := draftsFixture(t)
	other := draftsFixture(t)
	ctx := context.Background()
	d := requireDraft(t, f, draftInput())
	foreign := requireDraft(t, other, draftInput())
	ref := &DraftReference{d.ID, 1}
	next, e := f.app.UpdateDraft(ctx, f.actor, d.ID, DraftUpdateInput{1, json.RawMessage(`{"name":"new tab","parentId":null}`)})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		ref    *DraftReference
		kind   DraftKind
		target *string
	}{
		{ref, DraftDepartment, nil}, {&DraftReference{foreign.ID, 1}, DraftDepartment, nil}, {&DraftReference{d.ID, 2}, DraftIdentity, nil}, {&DraftReference{d.ID, 2}, DraftDepartment, &foreign.ID},
	} {
		tx, e := f.app.write(ctx, f.actor)
		if e != nil {
			t.Fatal(e)
		}
		removed, e := CleanupDraft(ctx, tx, f.actor, tc.ref, tc.kind, tc.target)
		if e != nil || removed {
			t.Fatalf("wrong version/owner/kind/target must preserve: removed=%v %v", removed, e)
		}
		if e := tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
	}
	tx, e := f.app.write(ctx, f.actor)
	if e != nil {
		t.Fatal(e)
	}
	removed, e := CleanupDraft(ctx, tx, f.actor, &DraftReference{d.ID, next.Version}, DraftDepartment, nil)
	if e != nil || !removed {
		t.Fatalf("exact cleanup: %v %v", removed, e)
	}
	_ = tx.Rollback(ctx)
	if _, e := f.app.GetDraft(ctx, f.actor, d.ID); e != nil {
		t.Fatal("business rollback restores cleaned draft")
	}
	tx, e = f.app.write(ctx, f.actor)
	if e != nil {
		t.Fatal(e)
	}
	removed, e = CleanupDraft(ctx, tx, f.actor, &DraftReference{d.ID, 2}, DraftDepartment, nil)
	if e != nil || !removed {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.GetDraft(ctx, f.actor, d.ID); !errors.Is(e, ErrMissing) {
		t.Fatal("committed exact cleanup missing")
	}
	if _, e = other.app.GetDraft(ctx, other.actor, foreign.ID); e != nil {
		t.Fatal("foreign draft survives")
	}
}
