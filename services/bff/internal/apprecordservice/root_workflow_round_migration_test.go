package apprecordservice

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"strings"
	"testing"
	"time"
)

func rootRoundMigrationFixture(t *testing.T) (context.Context, *pgx.Conn, string, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	c, e := pgx.Connect(ctx, os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	var id string
	if e = c.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&id); e != nil {
		t.Fatal(e)
	}
	schema := pgx.Identifier{"round_migration_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
	_, e = c.Exec(ctx, "CREATE SCHEMA "+schema+"; CREATE TABLE "+schema+".apps(id uuid PRIMARY KEY); CREATE TABLE "+schema+".workflow_definitions(app_id uuid,id uuid PRIMARY KEY); CREATE TABLE "+schema+".workflow_instances(id uuid PRIMARY KEY,app_id uuid NOT NULL,table_id uuid NOT NULL,view_id uuid NOT NULL,record_id uuid NOT NULL,flow_id uuid NOT NULL,initiator_id uuid NOT NULL,definition_version bigint NOT NULL,state text NOT NULL,created_at timestamptz NOT NULL)")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	raw, e := os.ReadFile("../../../../db/migrations/00030_workflow_rounds.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("exact one Down required")
	}
	return ctx, c, schema, strings.ReplaceAll(parts[0], "applications.", schema+"."), strings.ReplaceAll(parts[1], "applications.", schema+".")
}
func rootRoundLegacyRows(t *testing.T, ctx context.Context, c *pgx.Conn, s string, tied bool) {
	t.Helper()
	time2 := "2026-01-02"
	if tied {
		time2 = "2026-01-01"
	}
	_, e := c.Exec(ctx, "INSERT INTO "+s+".apps VALUES('10000000-0000-0000-0000-000000000001'); INSERT INTO "+s+".workflow_definitions VALUES('10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000005'); INSERT INTO "+s+".workflow_instances SELECT ('10000000-0000-0000-0000-00000000000'||n)::uuid,'10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000006',1,'completed',CASE n WHEN 7 THEN '2026-01-01'::timestamptz ELSE '"+time2+"'::timestamptz END FROM generate_series(7,8) n")
	if e != nil {
		t.Fatal(e)
	}
}
func TestRootRoundMigrationEmptyCycleAndNonemptyRefusal(t *testing.T) {
	ctx, c, s, up, down := rootRoundMigrationFixture(t)
	for _, sql := range []string{up, down, up} {
		if e := rootLifecycleMigrationApply(ctx, c, sql); e != nil {
			t.Fatal(e)
		}
	}
	rootRoundLegacyRows(t, ctx, c, s, false)
	e := rootLifecycleMigrationApply(ctx, c, down)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatalf("nonempty Down did not refuse %v", e)
	}
	var n int
	if e = c.QueryRow(ctx, "SELECT count(*) FROM "+s+".workflow_rounds").Scan(&n); e != nil || n != 2 {
		t.Fatalf("Down lost durable history %d %v", n, e)
	}
}
func TestRootRoundMigrationBackfillsWithoutInventingLinks(t *testing.T) {
	ctx, c, s, up, down := rootRoundMigrationFixture(t)
	rootRoundLegacyRows(t, ctx, c, s, false)
	if e := rootLifecycleMigrationApply(ctx, c, up); e != nil {
		t.Fatal(e)
	}
	var n int
	if e := c.QueryRow(ctx, "SELECT count(*) FROM "+s+".workflow_rounds WHERE round_kind='legacy' AND previous_instance_id IS NULL AND ((instance_id='10000000-0000-0000-0000-000000000007' AND round_number=1) OR (instance_id='10000000-0000-0000-0000-000000000008' AND round_number=2))").Scan(&n); e != nil || n != 2 {
		t.Fatalf("legacy facts invented or reordered %d %v", n, e)
	}
	if e := rootLifecycleMigrationApply(ctx, c, down); e == nil {
		t.Fatal("legacy history allowed Down")
	}
}
func TestRootRoundMigrationAmbiguousLegacyTimestampsFailAtomically(t *testing.T) {
	ctx, c, s, up, _ := rootRoundMigrationFixture(t)
	rootRoundLegacyRows(t, ctx, c, s, true)
	e := rootLifecycleMigrationApply(ctx, c, up)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatalf("ambiguous old order was guessed %v", e)
	}
	var n int
	var absent bool
	if e = c.QueryRow(ctx, "SELECT count(*),to_regclass($1) IS NULL FROM "+s+".workflow_instances", s+".workflow_rounds").Scan(&n, &absent); e != nil || n != 2 || !absent {
		t.Fatalf("refused migration mutated history %d %v %v", n, absent, e)
	}
}
