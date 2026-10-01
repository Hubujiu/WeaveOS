package personnel

import (
	"context"
	"testing"
	"time"
)

func TestQ36EventSearchUsesOnlyAllowlistedSummary(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.owner.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary,occurred_at) VALUES('personnel_changed','success',$1,'IDENTITY_UPDATED','q36-summary-search','identity',$2,'{"before":{"password":"hidden-summary-token"},"after":{"name":"Visible query name"}}','2026-10-01T00:00:00Z')`, f.actor.UserID, f.i1); err != nil {
		t.Fatal(err)
	}
	tx, err := f.app.read(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, tc := range []struct {
		search string
		total  int64
	}{{"hidden-summary-token", 0}, {"Visible query name", 1}} {
		r, err := scanEventProjection(ctx, tx, EventQueryInput{EventQuery: EventQuery{PageQuery: PageQuery{Search: tc.search}, From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}})
		if err != nil || r.Total != tc.total {
			t.Errorf("safe search %q: total=%d want=%d err=%v", tc.search, r.Total, tc.total, err)
		}
	}
}
