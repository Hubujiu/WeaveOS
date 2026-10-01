package personnel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestQ36EventSafeProjectionReferencesAndRR(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	target := newTarget(t, f)
	var account, actor string
	if err := f.owner.QueryRow(ctx, "SELECT account FROM auth.users WHERE id=$1", target).Scan(&account); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, "SELECT account FROM auth.users WHERE id=$1", f.actor.UserID).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ at, action, kind, id, summary string }{
		{"2026-10-01T01:00:00.000001Z", "MEMBER_IDENTITIES_UPDATED", "member", target, `{"before":{"identityIds":["` + f.i1 + `"],"password":"forbidden-secret"},"after":{"identityIds":["` + f.i2 + `"],"version":22},"request":"private"}`},
		{"2026-10-01T02:00:00Z", "IDENTITY_UPDATED", "identity", f.i1, `{"before":{"name":"Old"},"after":{"name":"New"}}`},
	} {
		if _, err := f.owner.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary,occurred_at) VALUES('personnel_changed','success',$1,$2,'q36-event-safe',$3,$4,$5::jsonb,$6::timestamptz)`, f.actor.UserID, e.action, e.kind, e.id, e.summary, e.at); err != nil {
			t.Fatal(err)
		}
	}
	input := EventQueryInput{EventQuery: EventQuery{PageQuery: PageQuery{Search: actor, Page: 2, PageSize: 1}, From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}
	tx, err := f.app.read(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	first, err := scanEventProjection(ctx, tx, input)
	if err != nil {
		t.Fatalf("safe full event projection required: %v", err)
	}
	if first.Total != 2 || len(first.Items) != 1 || first.Items[0].Display.Object != account || first.Items[0].Display.Detail != "身份：I1 → I2" || first.Items[0].Display.Action != "分配身份" || first.Items[0].Display.Outcome != "已完成" {
		t.Fatalf("independent full-user/reference labels: %+v", first)
	}
	raw, _ := json.Marshal(first.Items)
	if strings.Contains(string(raw), "forbidden-secret") || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "version") {
		t.Fatal("nonallowlisted summary data escaped", string(raw))
	}
	page, err := scanEventPage(ctx, tx, input)
	if err != nil || len(page) != 1 || page[0].ID != first.Items[0].ID {
		t.Fatal("page path disagrees", err)
	}
	input.Page = 1
	input.Sort = &QuerySort{Key: "occurredAt", Direction: "asc"}
	asc, err := scanEventPage(ctx, tx, input)
	if err != nil || asc[0].ID != first.Items[0].ID {
		t.Fatal("server time sorting", err)
	}
	var group FilterGroup
	_ = json.Unmarshal([]byte(`{"operator":"and","children":[{"field":"detail","operator":"eq","value":"身份：I1 → I2"}]}`), &group)
	input.Filter = &group
	filtered, err := scanEventProjection(ctx, tx, input)
	if err != nil || filtered.Total != 1 {
		t.Fatal("safe display filter before pagination", err)
	}
	if _, err = f.owner.Exec(ctx, "UPDATE personnel.identities SET name='I2 renamed' WHERE id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	frozen, err := scanEventProjection(ctx, tx, input)
	if err != nil || frozen.Fingerprint != filtered.Fingerprint {
		t.Fatal("event projection escaped caller RR", err)
	}
	tx2, err := f.app.read(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(ctx)
	fresh, err := scanEventProjection(ctx, tx2, input)
	if err != nil || fresh.Total != 0 || fresh.Fingerprint == filtered.Fingerprint {
		t.Fatal("referenced rename must change full result", err)
	}
}
