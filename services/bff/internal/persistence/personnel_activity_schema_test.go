package persistence_test

import (
	"context"
	"testing"
)

// Q25: application activity has a safe view; raw authentication history stays append-only.
func TestPersonnelActivityViewQ25(t *testing.T) {
	c := testConn(t)
	ctx := context.Background()
	var present bool
	if err := c.QueryRow(ctx, "SELECT to_regclass('personnel.activity_events') IS NOT NULL").Scan(&present); err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("Q25 safe activity view is absent")
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO auth.authentication_events(id,event_type,outcome,request_id,reason_code,object_type,object_id,change_summary) VALUES
	('00000000-0000-4000-8000-000000000081','personnel_changed','success','view-test','IDENTITY_CREATED','identity','00000000-0000-4000-8000-000000000082','{"name":"synthetic"}'),
	('00000000-0000-4000-8000-000000000083','login','success','view-test',NULL,NULL,NULL,NULL),
	('00000000-0000-4000-8000-000000000084','invitation_created','success','view-test',NULL,NULL,NULL,NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.activity_events WHERE id IN ('00000000-0000-4000-8000-000000000081','00000000-0000-4000-8000-000000000083','00000000-0000-4000-8000-000000000084')").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("only personnel and invitation activity visible, got %d", count)
	}
	rows, err := tx.Query(ctx, "SELECT column_name FROM information_schema.columns WHERE table_schema='personnel' AND table_name='activity_events' ORDER BY ordinal_position")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	expected := []string{"id", "occurred_at", "actor_account", "action", "object_type", "object_id", "change_summary", "outcome"}
	var actual []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		actual = append(actual, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(actual) != len(expected) {
		t.Fatalf("safe view fields: %v", actual)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Fatalf("safe view field %d: want %s, got %s", i, expected[i], actual[i])
		}
	}
}
