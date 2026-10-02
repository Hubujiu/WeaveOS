package persistence_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Oracle: approved Q36 PLAN §§3–5. Real PG constraints, not implementation text.
func TestQ36QueryDraftSchema(t *testing.T) {
	c := testConn(t)
	ctx := context.Background()
	for _, name := range []string{"personnel.query_revisions", "personnel.drafts"} {
		var present bool
		if err := c.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Fatalf("approved Q36 schema %s is absent", name)
		}
	}
	var scopes []string
	if err := c.QueryRow(ctx, "SELECT array_agg(scope ORDER BY scope) FROM personnel.query_revisions").Scan(&scopes); err != nil {
		t.Fatal(err)
	}
	if strings.Join(scopes, ",") != "activity,configuration,people" {
		t.Fatalf("unexpected scopes: %v", scopes)
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var owner string
	if err = tx.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES ('q36-schema-'||gen_random_uuid()) RETURNING id::text").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	for slot := 1; slot <= 20; slot++ {
		_, err = tx.Exec(ctx, "INSERT INTO personnel.drafts(owner_user_id,slot,kind,payload_json) VALUES($1,$2,'identity','{}')", owner, slot)
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM personnel.drafts WHERE owner_user_id=$1", owner).Scan(&count); err != nil || count != 20 {
		t.Fatalf("twenty drafts must coexist: %d %v", count, err)
	}
	for _, tc := range []struct {
		name, sql, code string
		args            []any
	}{
		{"twenty-first slot", "INSERT INTO personnel.drafts(owner_user_id,slot,kind,payload_json) VALUES($1,21,'identity','{}')", "23514", []any{owner}},
		{"duplicate slot", "INSERT INTO personnel.drafts(owner_user_id,slot,kind,payload_json) VALUES($1,1,'identity','{}')", "23505", []any{owner}},
		{"unknown kind", "UPDATE personnel.drafts SET kind='password' WHERE owner_user_id=$1 AND slot=1", "23514", []any{owner}},
		{"member target required", "UPDATE personnel.drafts SET kind='member-identities' WHERE owner_user_id=$1 AND slot=1", "23514", []any{owner}},
		{"payload must be object", "UPDATE personnel.drafts SET payload_json='[]' WHERE owner_user_id=$1 AND slot=1", "23514", []any{owner}},
		{"version positive", "UPDATE personnel.drafts SET draft_version=0 WHERE owner_user_id=$1 AND slot=1", "23514", []any{owner}},
		{"canonical payload bytes", "UPDATE personnel.drafts SET payload_json=$2 WHERE owner_user_id=$1 AND slot=1", "23514", []any{owner, `{"name":"` + strings.Repeat("x", 65526) + `"}`}},
		{"unknown revision", "INSERT INTO personnel.query_revisions(scope) VALUES('global')", "23514", nil},
	} {
		if _, err = tx.Exec(ctx, "SAVEPOINT rejected_input"); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, tc.sql, tc.args...)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != tc.code {
			t.Errorf("%s expected %s, got %v", tc.name, tc.code, err)
		}
		if _, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT rejected_input"); err != nil {
			t.Fatal(err)
		}
	}
	exact := `{"name":"` + strings.Repeat("x", 65525) + `"}` // 65536 UTF-8 bytes; literal independent boundary.
	if len(exact) != 65536 {
		t.Fatalf("bad independent boundary fixture %d", len(exact))
	}
	if _, err = tx.Exec(ctx, "UPDATE personnel.drafts SET payload_json=$2 WHERE owner_user_id=$1 AND slot=1", owner, exact); err != nil {
		t.Fatalf("64KiB must be accepted: %v", err)
	}
	// Target intentionally has no FK: deleting a business object must preserve its draft.
	if _, err = tx.Exec(ctx, "UPDATE personnel.drafts SET target_id='00000000-0000-4000-8000-000000000099',base_version=1 WHERE owner_user_id=$1 AND slot=2", owner); err != nil {
		t.Fatal(err)
	}
	var expiryColumns int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='personnel' AND table_name='drafts' AND column_name IN ('expires_at','session_id')").Scan(&expiryColumns); err != nil || expiryColumns != 0 {
		t.Fatalf("draft lifetime must not depend on query/session TTL: %d %v", expiryColumns, err)
	}
}
