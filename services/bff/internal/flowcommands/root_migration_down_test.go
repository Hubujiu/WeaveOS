package flowcommands

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

// Runs the exact migration with only its fixed schema identifier isolated.
// It must protect a command accepted concurrently with a proposed rollback.
func TestRootFormalMigrationDownSeesConcurrentAcceptedHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	owner, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer owner.Close(context.Background())
	other, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close(context.Background())
	var id string
	if e = owner.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&id); e != nil {
		t.Fatal(e)
	}
	schema := "root_down_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = owner.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	defer owner.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
	raw, e := os.ReadFile("../../../../db/migrations/00014_workflow_command_ledger.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("expected one migration Down section")
	}
	up := strings.ReplaceAll(parts[0], "applications.", quoted+".")
	down := strings.ReplaceAll(parts[1], "applications.", quoted+".")
	if _, e = owner.Exec(ctx, up); e != nil {
		t.Fatal(e)
	}
	accepted, e := owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer accepted.Rollback(context.Background())
	if _, e = accepted.Exec(ctx, "INSERT INTO "+quoted+".workflow_commands(command_id,command_json,command_hash,state) VALUES($1::uuid,jsonb_build_object('CommandID',$1::text),decode(repeat('aa',32),'hex'),'pending')", id); e != nil {
		t.Fatal(e)
	}
	var pid int
	if e = other.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); e != nil {
		t.Fatal(e)
	}
	rollbackTx, e := other.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	result := make(chan error, 1)
	go func() {
		_, err := rollbackTx.Exec(ctx, down)
		_ = rollbackTx.Rollback(context.Background())
		result <- err
	}()
	blocked := false
	for !blocked {
		select {
		case err := <-result:
			t.Fatalf("Down did not serialize with an accepted command: %v", err)
		case <-ctx.Done():
			t.Fatal("Down never reached the real PostgreSQL lock wait")
		default:
		}
		if e = accepted.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND NOT granted)", pid).Scan(&blocked); e != nil {
			t.Fatal(e)
		}
		if !blocked {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if e = accepted.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-result:
	case <-ctx.Done():
		t.Fatal("Down did not finish after competing transaction committed")
	}
	var pgerr *pgconn.PgError
	if !errors.As(e, &pgerr) || pgerr.Code != "55000" {
		t.Fatalf("Down must recheck committed history under its table lock; got %v", e)
	}
	var n int
	if e = owner.QueryRow(ctx, "SELECT count(*) FROM "+quoted+".workflow_commands WHERE command_id=$1", id).Scan(&n); e != nil || n != 1 {
		t.Fatalf("history count %d err %v", n, e)
	}
}
