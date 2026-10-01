package persistence_test

import (
	"context"
	"os"
	"testing"
)

func TestQ36LeastPrivilegeDraftIntegration(t *testing.T) {
	c := testConn(t)
	ctx := context.Background()
	roles, err := os.ReadFile("../../../../infra/runtime/roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Exec(ctx, string(roles)); err != nil {
		t.Fatal(err)
	}
	for _, priv := range []string{"SELECT", "INSERT", "DELETE"} {
		var allowed bool
		if err = c.QueryRow(ctx, "SELECT has_table_privilege('auth_app','personnel.drafts',$1)", priv).Scan(&allowed); err != nil || !allowed {
			t.Errorf("draft %s required: %v %v", priv, allowed, err)
		}
	}
	for _, col := range []string{"payload_json", "draft_version", "updated_at"} {
		var allowed bool
		if err = c.QueryRow(ctx, "SELECT has_column_privilege('auth_app','personnel.drafts',$1,'UPDATE')", col).Scan(&allowed); err != nil || !allowed {
			t.Errorf("mutable draft %s: %v %v", col, allowed, err)
		}
	}
	for _, col := range []string{"owner_user_id", "kind", "target_id", "base_version", "slot", "created_at"} {
		var allowed bool
		if err = c.QueryRow(ctx, "SELECT has_column_privilege('auth_app','personnel.drafts',$1,'UPDATE')", col).Scan(&allowed); err != nil || allowed {
			t.Errorf("immutable draft %s must stay denied: %v %v", col, allowed, err)
		}
	}
	var rawAudit bool
	if err = c.QueryRow(ctx, "SELECT has_table_privilege('auth_app','auth.authentication_events','SELECT')").Scan(&rawAudit); err != nil || rawAudit {
		t.Fatal("must not expand raw audit read", err)
	}
}
