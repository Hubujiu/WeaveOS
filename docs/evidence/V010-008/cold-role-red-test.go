package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Q9 + approved audit dictionary/DDL: independent PostgreSQL databases, twelve
// original fields, no cross-database user FK, and no ordinary archive query index.
func db(t *testing.T, key string) *pgx.Conn {
	t.Helper()
	url := os.Getenv(key)
	if url == "" {
		t.Fatal(key + " must name an isolated PostgreSQL 18 database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal("isolated PostgreSQL connection unavailable")
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}
func source(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../../..", path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestColdArchiveSchema(t *testing.T) {
	c := db(t, "WEAVEOS_TEST_ARCHIVE_DATABASE_URL")
	ctx := context.Background()
	var cold, live string
	if err := c.QueryRow(ctx, "SELECT current_database()").Scan(&cold); err != nil {
		t.Fatal(err)
	}
	if err := db(t, "WEAVEOS_TEST_DATABASE_URL").QueryRow(ctx, "SELECT current_database()").Scan(&live); err != nil {
		t.Fatal(err)
	}
	if cold == live {
		t.Fatal("cold archive must be a different database")
	}
	var exists bool
	if err := c.QueryRow(ctx, "SELECT to_regclass('archive.authentication_events') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	// A published migration is applied once; subsequent runs validate its catalog.
	if !exists {
		if _, err := c.Exec(ctx, source(t, "db/archive-migrations/00001_archive.sql")); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.QueryRow(ctx, "SELECT to_regclass('archive.authentication_events') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("Q9: cold archive table is absent")
	}
	rows, err := c.Query(ctx, "SELECT column_name FROM information_schema.columns WHERE table_schema='archive' AND table_name='authentication_events' ORDER BY ordinal_position")
	if err != nil {
		t.Fatal(err)
	}
	var cols []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, s)
	}
	rows.Close()
	want := "id,event_type,outcome,actor_user_id,subject_user_id,account_fingerprint,client_ip,user_agent,session_ref,reason_code,request_id,occurred_at"
	if strings.Join(cols, ",") != want {
		t.Fatal("archive must preserve all twelve approved fields")
	}
	var fks, indexes int
	if err := c.QueryRow(ctx, "SELECT count(*) FROM pg_constraint WHERE conrelid='archive.authentication_events'::regclass AND contype='f'").Scan(&fks); err != nil {
		t.Fatal(err)
	}
	if err := c.QueryRow(ctx, "SELECT count(*) FROM pg_indexes WHERE schemaname='archive'").Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if fks != 0 || indexes != 1 {
		t.Fatal("cold storage only needs its event-id primary key; no live-user FK or query indexes")
	}
}
func TestRuntimeRolesRestrictAuditAndBootstrap(t *testing.T) {
	c := db(t, "WEAVEOS_TEST_DATABASE_URL")
	ctx := context.Background()
	if _, err := c.Exec(ctx, source(t, "infra/runtime/roles.sql")); err != nil {
		t.Fatal(err)
	}
	var present bool
	if err := c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='auth_app')").Scan(&present); err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("ADR004/DDL: restricted application role absent")
	}
	for _, q := range []string{
		"SELECT has_table_privilege('auth_app','auth.authentication_events','UPDATE')",
		"SELECT has_table_privilege('auth_app','auth.authentication_events','DELETE')",
		"SELECT has_column_privilege('auth_app','auth.authentication_events','client_ip','SELECT')",
		"SELECT has_column_privilege('auth_app','auth.users','is_bootstrap_admin','UPDATE')",
		"SELECT has_column_privilege('auth_app','auth.users','is_bootstrap_admin','INSERT')",
		"SELECT has_schema_privilege('auth_app','auth','CREATE')",
	} {
		var granted bool
		if err := c.QueryRow(ctx, q).Scan(&granted); err != nil {
			t.Fatal(err)
		}
		if granted {
			t.Errorf("forbidden runtime privilege: %s", q)
		}
	}
	var granted bool
	if err := c.QueryRow(ctx, "SELECT has_table_privilege('auth_app','auth.authentication_events','INSERT')").Scan(&granted); err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Error("application must be able to append authentication events")
	}
	for _, role := range []string{"auth_reader", "auth_maintenance"} {
		if err := c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", role).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Errorf("separate controlled %s role absent", role)
			continue
		}
		if err := c.QueryRow(ctx, "SELECT has_table_privilege($1,'auth.password_credentials','SELECT')", role).Scan(&granted); err != nil {
			t.Fatal(err)
		}
		if granted {
			t.Error("audit roles must not read password hashes")
		}
	}
}

func TestColdMaintenanceRoleCanOnlyMaintainAudit(t *testing.T) {
	c := db(t, "WEAVEOS_TEST_ARCHIVE_DATABASE_URL")
	ctx := context.Background()
	if _, err := c.Exec(ctx, source(t, "infra/runtime/cold-roles.sql")); err != nil {
		t.Fatal(err)
	}
	for _, privilege := range []string{"SELECT", "INSERT", "DELETE"} {
		var granted bool
		if err := c.QueryRow(ctx, "SELECT has_table_privilege('auth_maintenance','archive.authentication_events',$1)", privilege).Scan(&granted); err != nil {
			t.Fatal(err)
		}
		if !granted {
			t.Errorf("controlled maintenance must have cold %s", privilege)
		}
	}
	for _, role := range []string{"auth_app", "auth_reader"} {
		var granted bool
		if err := c.QueryRow(ctx, "SELECT has_schema_privilege($1,'archive','USAGE')", role).Scan(&granted); err != nil {
			t.Fatal(err)
		}
		if granted {
			t.Error("ordinary runtime and audit reader must not access cold archive")
		}
	}
}
