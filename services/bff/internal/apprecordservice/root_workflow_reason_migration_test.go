package apprecordservice

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// V075 P25 / V064 data increment37: execute the unchanged migration in a
// random isolated schema, never downgrade the shared integration database.
func TestRootWorkflowReasonMigrationEmptyCycleAndNonemptyRefusal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	c, err := pgx.Connect(ctx, os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	var id string
	if err = c.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{"reason_migration_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
	if _, err = c.Exec(ctx, "CREATE SCHEMA "+schema+"; CREATE TABLE "+schema+".workflow_commands(command_id uuid PRIMARY KEY,command_json jsonb NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	raw, err := os.ReadFile("../../../../db/migrations/00037_workflow_action_reasons.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("single exact Down required")
	}
	up, down := strings.ReplaceAll(parts[0], "applications.", schema+"."), strings.ReplaceAll(parts[1], "applications.", schema+".")
	for _, sql := range []string{up, down, up} {
		if err = rootLifecycleMigrationApply(ctx, c, sql); err != nil {
			t.Fatal(err)
		}
	}
	const original = "  原因\n保留 🦊  "
	if _, err = c.Exec(ctx, "INSERT INTO "+schema+".workflow_commands VALUES($1,'{\"Action\":\"reject\"}')", id); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "\u0085\u00a0\u3000\t", strings.Repeat("🦊", 501)} {
		_, err = c.Exec(ctx, "INSERT INTO "+schema+".workflow_action_reasons VALUES($1,$2)", id, invalid)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "23514" {
			t.Fatalf("invalid audit text not rejected: %v", err)
		}
	}
	if _, err = c.Exec(ctx, "INSERT INTO "+schema+".workflow_action_reasons VALUES($1,$2)", id, original); err != nil {
		t.Fatal(err)
	}
	err = rootLifecycleMigrationApply(ctx, c, down)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "55000" {
		t.Fatalf("nonempty audit Down did not refuse: %v", err)
	}
	var got string
	if err = c.QueryRow(ctx, "SELECT reason FROM "+schema+".workflow_action_reasons WHERE command_id=$1", id).Scan(&got); err != nil || got != original {
		t.Fatalf("refused Down changed original audit text: %v", err)
	}
}
