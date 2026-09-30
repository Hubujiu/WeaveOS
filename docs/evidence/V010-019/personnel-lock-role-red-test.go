package persistence_test

import (
	"context"
	"os"
	"testing"
)

// Q25 least privilege: configuration CRUD, owner-controlled directory, safe view only.
// The isolated owner applies the role script under test; assertions come from Q25.
func TestPersonnelRuntimeRolesQ25(t *testing.T) {
	c := testConn(t)
	ctx := context.Background()
	roles, err := os.ReadFile("../../../../infra/runtime/roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, string(roles)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		role, table, privilege string
		want                   bool
	}{
		{"auth_app", "personnel.identities", "SELECT", true},
		{"auth_app", "personnel.identities", "INSERT", true},
		{"auth_app", "personnel.identities", "UPDATE", true},
		{"auth_app", "personnel.identities", "DELETE", true},
		{"auth_app", "personnel.permission_catalog", "SELECT", true},
		{"auth_app", "personnel.permission_catalog", "INSERT", false},
		{"auth_app", "personnel.permission_catalog", "UPDATE", false},
		{"auth_app", "personnel.permission_catalog", "DELETE", false},
		{"auth_app", "personnel.activity_events", "SELECT", true},
		{"auth_app", "auth.authentication_events", "SELECT", false},
		{"auth_app", "auth.authentication_events", "INSERT", true},
		{"auth_app", "auth.authentication_events", "UPDATE", false},
		{"auth_maintenance", "personnel.identities", "UPDATE", false},
		{"auth_reader", "personnel.identities", "UPDATE", false},
		{"auth_backup", "personnel.identities", "SELECT", true},
	} {
		t.Run(tc.role+"/"+tc.table+"/"+tc.privilege, func(t *testing.T) {
			var got bool
			if err := c.QueryRow(ctx, "SELECT has_table_privilege($1,$2,$3)", tc.role, tc.table, tc.privilege).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Q25 privilege want %v, got %v", tc.want, got)
			}
		})
	}
	var writableRoot bool
	if err := c.QueryRow(ctx, "SELECT has_column_privilege('auth_app','auth.users','is_bootstrap_admin','UPDATE')").Scan(&writableRoot); err != nil {
		t.Fatal(err)
	}
	if writableRoot {
		t.Fatal("runtime must never mutate trusted Bootstrap Admin fact")
	}
	var lockPresent bool
	if err := c.QueryRow(ctx, "SELECT to_regprocedure('personnel.lock_permission_catalog(uuid)') IS NOT NULL").Scan(&lockPresent); err != nil {
		t.Fatal(err)
	}
	if !lockPresent {
		t.Fatal("read-only catalog locking helper absent")
	}
	for _, role := range []string{"auth_app", "auth_reader", "auth_maintenance", "auth_backup"} {
		var executable bool
		if err := c.QueryRow(ctx, "SELECT has_function_privilege($1,'personnel.lock_permission_catalog(uuid)','EXECUTE')", role).Scan(&executable); err != nil {
			t.Fatal(err)
		}
		if executable != (role == "auth_app") {
			t.Fatalf("restricted catalog lock execution for %s", role)
		}
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE auth_app"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.activity_events").Scan(&count); err != nil {
		t.Fatalf("safe view must be readable by actual runtime role: %v", err)
	}
}
