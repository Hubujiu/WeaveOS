package persistence_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func testConn(t *testing.T) *pgx.Conn {
	t.Helper()
	url := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("WEAVEOS_TEST_DATABASE_URL must point to an isolated PostgreSQL 18 test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect isolated PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func TestAuthSchemaMatchesApprovedDictionary(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	for _, table := range []string{"auth.users", "auth.password_credentials", "auth.invitations", "auth.authentication_events"} {
		var present bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Errorf("approved v0.1.0 table %s is missing", table)
		}
	}
}
