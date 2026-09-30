package audit

import (
	"context"
	"testing"
)

// Q25 keeps original monthly/yearly rules and all fifteen event columns.
func TestPersonnelArchivePreservesSafeSummaryAndOriginalRetention(t *testing.T) {
	l, c := clean(t)
	ctx := context.Background()
	insert := `INSERT INTO auth.authentication_events(id,event_type,outcome,reason_code,request_id,occurred_at,object_type,object_id,change_summary) VALUES($1,'personnel_changed','success','IDENTITY_UPDATED','personnel-archive','2026-08-01T00:00:00Z','identity','20000000-0000-4000-8000-000000000001','{"before":{"name":"old"},"after":{"name":"new"}}')`
	if _, err := l.Exec(ctx, insert, eventID); err != nil {
		t.Fatal(err)
	}
	result, err := Maintain(ctx, l, c, now)
	if err != nil || result.Archived != 1 {
		t.Fatal("new personnel event must archive using original monthly rule", err)
	}
	var kind, id string
	var correct bool
	if err = c.QueryRow(ctx, `SELECT object_type,object_id::text,change_summary='{"before":{"name":"old"},"after":{"name":"new"}}'::jsonb FROM archive.authentication_events WHERE id=$1`, eventID).Scan(&kind, &id, &correct); err != nil {
		t.Fatal(err)
	}
	if kind != "identity" || id != "20000000-0000-4000-8000-000000000001" || !correct {
		t.Fatal("all new object and summary columns must survive cold copy")
	}
	if _, err = l.Exec(ctx, insert, eventID); err != nil {
		t.Fatal(err)
	}
	if _, err = Maintain(ctx, l, c, now); err != nil {
		t.Fatal("same fifteen-column retry must deduplicate", err)
	}
	if count(t, l, "auth.authentication_events") != 0 || count(t, c, "archive.authentication_events") != 1 {
		t.Fatal("cold retry must retain exactly one event")
	}
	if _, err = l.Exec(ctx, insert, eventID); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Exec(ctx, `UPDATE archive.authentication_events SET change_summary='{"before":{"name":"different"},"after":{"name":"new"}}' WHERE id=$1`, eventID); err != nil {
		t.Fatal(err)
	}
	if _, err = Maintain(ctx, l, c, now); err == nil {
		t.Fatal("conflicting new summary must stop deletion of hot history")
	}
	if count(t, l, "auth.authentication_events") != 1 {
		t.Fatal("summary conflict must preserve hot event")
	}
	if _, err = l.Exec(ctx, "UPDATE auth.authentication_events SET occurred_at='2025-09-26T12:00:00Z'"); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Exec(ctx, "UPDATE archive.authentication_events SET occurred_at='2025-09-26T12:00:00Z'"); err != nil {
		t.Fatal(err)
	}
	if _, err = Maintain(ctx, l, c, now); err != nil {
		t.Fatal(err)
	}
	if count(t, l, "auth.authentication_events") != 0 || count(t, c, "archive.authentication_events") != 0 {
		t.Fatal("one-calendar-year boundary still expires personnel events in both stores")
	}
}
