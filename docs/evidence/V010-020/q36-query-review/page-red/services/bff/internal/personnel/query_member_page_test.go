package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestQ36MemberSQLPageBeforeDisplayRelations(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("q36-page-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		for _, table := range []string{"personnel.department_members", "personnel.member_identities"} {
			if _, err := f.owner.Exec(ctx, "DELETE FROM "+table+" WHERE user_id IN (SELECT id FROM auth.users WHERE starts_with(account,$1))", prefix); err != nil {
				t.Error(err)
			}
		}
		if _, err := f.owner.Exec(ctx, "DELETE FROM auth.users WHERE starts_with(account,$1)", prefix); err != nil {
			t.Error(err)
		}
	})
	if _, err := f.owner.Exec(ctx, `INSERT INTO auth.users(account,created_at) SELECT $1||lpad(n::text,4,'0'),'2026-01-01'::timestamptz+n*interval '1 second' FROM generate_series(0,1999) n`, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, `INSERT INTO personnel.member_identities(user_id,identity_id) SELECT id,$2 FROM auth.users WHERE starts_with(account,$1)`, prefix, f.i1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, `INSERT INTO personnel.department_members(user_id,department_id) SELECT u.id,d.id FROM auth.users u CROSS JOIN personnel.departments d WHERE starts_with(u.account,$1) AND d.is_root`, prefix); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"auth.users", "personnel.member_identities", "personnel.department_members"} {
		if _, err := f.owner.Exec(ctx, "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}
	input := MemberQueryInput{MemberQuery: MemberQuery{PageQuery: PageQuery{Search: prefix, Page: 20, PageSize: 7}}}
	tx, err := f.app.read(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	fullSQL, fullArgs, _, err := memberProjectionQuery(input)
	if err != nil {
		t.Fatal(err)
	}
	explainMemberPage(t, ctx, tx, "full", fullSQL, fullArgs)
	query, args, _, err := memberPageQuery(input)
	if err != nil {
		t.Fatalf("bounded SQL page path required: %v", err)
	}
	plan := explainMemberPage(t, ctx, tx, "page", query, args)
	// Each fixture has one department and one identity. Display-label aggregate
	// input must be page-local, not all 2000 matching members (or all users).
	var labelInputs float64
	var inspect func(map[string]any)
	inspect = func(n map[string]any) {
		if n["Node Type"] == "Aggregate" && strings.Contains(fmt.Sprint(n["Output"]), "jsonb_agg") {
			for _, p := range n["Plans"].([]any) {
				child := p.(map[string]any)
				labelInputs += child["Actual Rows"].(float64) * child["Actual Loops"].(float64)
			}
		}
		if children, ok := n["Plans"].([]any); ok {
			for _, c := range children {
				inspect(c.(map[string]any))
			}
		}
	}
	inspect(plan)
	if labelInputs != 14 {
		t.Fatalf("only seven members' two labels may reach display aggregation; got %v input rows", labelInputs)
	}
	got, err := scanMemberPage(ctx, tx, input)
	if err != nil || len(got) != 7 {
		t.Fatalf("page length: %d %v", len(got), err)
	}
	for i, row := range got {
		if row.Account != prefix+fmt.Sprintf("%04d", 133+i) || len(row.Departments) != 1 || len(row.Identities) != 1 || row.Identities[0].Name != "I1" {
			t.Fatalf("wrong independent page/display expectation: %+v", row)
		}
	}
	// Complex membership filters still apply before paging.
	raw := fmt.Sprintf(`{"operator":"or","children":[{"field":"account","operator":"eq","value":%q},{"field":"identityIds","operator":"eq","value":%q}]}`, prefix+"0133", f.i2)
	var group FilterGroup
	if err = json.Unmarshal([]byte(raw), &group); err != nil {
		t.Fatal(err)
	}
	input.Filter = &group
	input.Page = 1
	filtered, err := scanMemberPage(ctx, tx, input)
	if err != nil || len(filtered) != 1 || filtered[0].Account != prefix+"0133" {
		t.Fatalf("full-condition selection before limit: %+v %v", filtered, err)
	}
	input.Filter = nil
	input.Page = 1000
	empty, err := scanMemberPage(ctx, tx, input)
	if err != nil || len(empty) != 0 {
		t.Fatalf("out-of-range page must be empty: %+v %v", empty, err)
	}
	// A page read must use its caller's RR, even across a concurrent commit.
	input.Page = 20
	if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", got[0].ID, prefix+"changed"); err != nil {
		t.Fatal(err)
	}
	same, err := scanMemberPage(ctx, tx, input)
	if err != nil || same[0].Account != got[0].Account {
		t.Fatal("page escaped caller RR", err)
	}
}

func explainMemberPage(t *testing.T, ctx context.Context, tx pgx.Tx, label, query string, args []any) map[string]any {
	t.Helper()
	var raw []byte
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT JSON) "+query, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s EXPLAIN %s", label, raw)
	var result []map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result[0]["Plan"].(map[string]any)
}
