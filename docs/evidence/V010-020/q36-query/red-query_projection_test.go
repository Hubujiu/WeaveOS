package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestQ36MemberCompleteProjectionAndRR(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("q36-projection-%d-", time.Now().UnixNano())
	ids := []string{}
	for i := 0; i < 3; i++ {
		id := newTarget(t, f)
		ids = append(ids, id)
		if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=$2,created_at=$3 WHERE id=$1", id, prefix+fmt.Sprint(i), time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	input := MemberQueryInput{MemberQuery: MemberQuery{PageQuery: PageQuery{Search: prefix, Page: 2, PageSize: 1}}}
	initial, err := f.app.MemberProjection(ctx, f.actor, input)
	if err != nil {
		t.Fatalf("complete live projection unavailable: %v", err)
	}
	if initial.Total != 3 || len(initial.Items) != 1 || initial.Items[0].ID != ids[1] || len(initial.Fingerprint) != 64 {
		t.Fatalf("full total and page mismatch: %+v", initial)
	}
	// Internal/configuration versions are not visible projection changes.
	if _, err = f.owner.Exec(ctx, "INSERT INTO personnel.member_configuration(user_id,version) VALUES($1,9)", ids[2]); err != nil {
		t.Fatal(err)
	}
	unchanged, err := f.app.MemberProjection(ctx, f.actor, input)
	if err != nil || unchanged.Fingerprint != initial.Fingerprint {
		t.Fatalf("object version must not alter display digest: %v", err)
	}
	// A read already in RR must remain a coherent snapshot across concurrent commit.
	tx, err := f.app.read(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	before, err := scanMemberProjection(ctx, tx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", ids[2], prefix+"renamed"); err != nil {
		t.Fatal(err)
	}
	sameSnapshot, err := scanMemberProjection(ctx, tx, input)
	if err != nil || sameSnapshot.Fingerprint != before.Fingerprint {
		t.Fatal("RR scan mixed two snapshots", err)
	}
	changed, err := f.app.MemberProjection(ctx, f.actor, input)
	if err != nil || changed.Fingerprint == initial.Fingerprint {
		t.Fatal("off-page visible change must alter complete digest", err)
	}
	if changed.Items[0].ID != ids[1] {
		t.Fatal("off-page change unexpectedly changed requested page")
	}
	if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", ids[2], prefix+"2"); err != nil {
		t.Fatal(err)
	}
	restored, err := f.app.MemberProjection(ctx, f.actor, input)
	if err != nil || restored.Fingerprint != initial.Fingerprint {
		t.Fatal("ABA must return same current-result fingerprint", err)
	}
	// Selected filter object's name/existence is relevant even for empty results.
	raw, _ := json.Marshal(map[string]any{"operator": "and", "children": []any{map[string]any{"field": "identityIds", "operator": "eq", "value": f.i1}}})
	var group FilterGroup
	_ = json.Unmarshal(raw, &group)
	input.Filter = &group
	empty, err := f.app.MemberProjection(ctx, f.actor, input)
	if err != nil || empty.Total != 0 {
		t.Fatal("expected empty selected identity query", err)
	}
	if _, err = f.owner.Exec(ctx, "UPDATE personnel.identities SET name='renamed selected' WHERE id=$1", f.i1); err != nil {
		t.Fatal(err)
	}
	renamed, err := f.app.MemberProjection(ctx, f.actor, input)
	if err != nil || renamed.Fingerprint == empty.Fingerprint {
		t.Fatal("selected identity rename must change empty-query context", err)
	}
}
