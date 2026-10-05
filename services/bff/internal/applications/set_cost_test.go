package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Root review P2: two fixed List data statements, no per-member round trips.
// Counts are real pgx queries, excluding BEGIN/COMMIT/ROLLBACK and setup.
type statementCounter struct {
	active         atomic.Bool
	queries        atomic.Int64
	menuValidation atomic.Int64
}

func (c *statementCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	q := strings.ToLower(strings.TrimSpace(d.SQL))
	if c.active.Load() && !strings.HasPrefix(q, "begin") && q != "commit" && q != "rollback" && !strings.HasPrefix(q, "set role") {
		c.queries.Add(1)
		if strings.Contains(q, "applications.menu_resources") {
			c.menuValidation.Add(1)
		}
	}
	return ctx
}
func (c *statementCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (c *statementCounter) reset() {
	c.active.Store(false)
	c.queries.Store(0)
	c.menuValidation.Store(0)
	c.active.Store(true)
}
func countedApplication(t *testing.T, f *webFixture) (*Application, *statementCounter) {
	t.Helper()
	c := &statementCounter{}
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = c
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &Application{Pool: p}, c
}
func resetScaleApps(t *testing.T, f *webFixture) {
	t.Helper()
	_, err := f.owner.Exec(context.Background(), `DELETE FROM personnel.identity_permissions WHERE permission_code IN (SELECT 'app.'||id::text||'.access' FROM applications.apps);
 DELETE FROM personnel.template_permissions WHERE permission_code IN (SELECT 'app.'||id::text||'.access' FROM applications.apps);
 DELETE FROM personnel.permission_catalog WHERE app_id IN (SELECT id::text FROM applications.apps);
 TRUNCATE applications.workflow_evidence_members,applications.workflow_evidence_documents,applications.workflow_evidence_blobs,applications.workflow_tasks,applications.workflow_execution_events,applications.workflow_publications,applications.workflow_engine_receipts,applications.workflow_instances,applications.workflow_deployments,applications.workflow_versions,applications.workflow_definitions,applications.record_change_values,applications.record_change_events,applications.field_option_tombstones,applications.record_drafts,applications.record_write_audit,applications.record_command_fences,applications.table_field_dependencies,applications.form_views,applications.fields,applications.logical_tables,applications.directories,applications.operations,applications.grant_fields,applications.grants,applications.menu_resources,applications.group_members,applications.permission_groups,applications.apps`)
	if err != nil {
		t.Fatal(err)
	}
}
func seedOwnedApps(t *testing.T, f *webFixture, n int) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.owner.Query(ctx, `INSERT INTO applications.apps(name,owner_user_id,created_at)
 SELECT 'b5-cost-'||i,$1,'2026-01-01'::timestamptz+i*interval '1 microsecond' FROM generate_series(1,$2::int) i RETURNING id::text,name`, f.actor, n)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, n)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatal(err)
		}
		i, err := strconv.Atoi(strings.TrimPrefix(name, "b5-cost-"))
		if err != nil {
			t.Fatal(err)
		}
		ids[i-1] = id
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, "INSERT INTO applications.menu_resources(app_id,resource_kind,resource_id) SELECT id,'application',id FROM applications.apps WHERE owner_user_id=$1", f.actor); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.permission_catalog(code,name,category,app_id) SELECT 'app.'||id::text||'.access',name,'application',id::text FROM applications.apps WHERE owner_user_id=$1", f.actor); err != nil {
		t.Fatal(err)
	}
	return ids
}
func logCost(t *testing.T, kind string, n int, counts []int64, times []float64) {
	t.Helper()
	sorted := append([]float64(nil), times...)
	sort.Float64s(sorted)
	b, _ := json.Marshal(map[string]any{"kind": kind, "size": n, "dataStatementCounts": counts, "latencyMs": times, "medianMs": sorted[len(sorted)/2], "scope": "same isolated PostgreSQL18.6; actual auth_app; direct application method; setup and tx-control statements excluded"})
	t.Log("B5_COST " + string(b))
}
func TestB5ListHasTwoDataStatementsAtControlledScale(t *testing.T) {
	for _, n := range []int{0, 10, 100, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			f := fixture(t, false)
			resetScaleApps(t, f)
			t.Cleanup(func() { resetScaleApps(t, f) })
			expected := seedOwnedApps(t, f, n)
			a, c := countedApplication(t, f)
			p := principal(t, f)
			p.BootstrapAdmin = true // Caller flag must not grant root.
			counts := []int64{}
			times := []float64{}
			for range 5 {
				c.reset()
				start := time.Now()
				items, err := a.List(context.Background(), p)
				times = append(times, float64(time.Since(start).Microseconds())/1000)
				counts = append(counts, c.queries.Load())
				c.active.Store(false)
				if err != nil {
					t.Fatal(err)
				}
				if items == nil || len(items) != n {
					t.Fatalf("untruncated stable list must contain all %d authorized apps", n)
				}
				for i, item := range items {
					if item.ID != expected[i] {
						t.Fatal("list must preserve created_at/id order")
					}
				}
			}
			logCost(t, "list", n, counts, times)
			for _, count := range counts {
				if count != 2 {
					t.Errorf("List must use two fixed data statements at A=%d; got %v", n, counts)
					break
				}
			}
		})
	}
}
func seedMembers(t *testing.T, f *webFixture, n int) []string {
	t.Helper()
	rows, err := f.owner.Query(context.Background(), "INSERT INTO auth.users(account) SELECT 'b5-member-'||gen_random_uuid() FROM generate_series(1,$1::int) RETURNING id::text", n)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	return ids
}
func TestB5MemberReplacementHasConstantStatementsAtThousandMembers(t *testing.T) {
	f := fixture(t, false)
	resetScaleApps(t, f)
	t.Cleanup(func() { resetScaleApps(t, f) })
	app := seedOwnedApps(t, f, 1)[0]
	base := "/api/v1/applications/" + app + "/permission-groups"
	gid := value(t, data(t, f.call("POST", base, map[string]any{"name": "cost", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201), "id")
	ids := seedMembers(t, f, 1000)
	a, c := countedApplication(t, f)
	p := principal(t, f)
	rev := int64(2)
	for _, n := range []int{10, 100, 1000} {
		counts := []int64{}
		times := []float64{}
		for range 3 {
			in := Input{OperationID: f.operation(t), ExpectedPolicyRevision: rev, MemberIDs: ids[:n]}
			c.reset()
			start := time.Now()
			result, err := a.Write(context.Background(), p, "members.replace", app, gid, in, Metadata{RequestID: "cost-members"})
			times = append(times, float64(time.Since(start).Microseconds())/1000)
			counts = append(counts, c.queries.Load())
			c.active.Store(false)
			if err != nil || result.Status != 200 {
				t.Fatal("member batch must commit unchanged policy semantics", err)
			}
			rev++
			var stored int
			if err := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.group_members WHERE app_id=$1 AND group_id=$2", app, gid).Scan(&stored); err != nil || stored != n {
				t.Fatal("full member set must persist without truncation")
			}
		}
		logCost(t, "members.replace", n, counts, times)
		for _, count := range counts {
			if count > 20 {
				t.Errorf("member replacement must use a fixed <=20 data statements at M=%d; got %v", n, counts)
				break
			}
		}
	}
}
func TestB5GrantValidationIsOneCompleteTupleSetQuery(t *testing.T) {
	f := fixture(t, true)
	app, _ := f.create(t)
	base := "/api/v1/applications/" + app + "/permission-groups"
	gid := value(t, data(t, f.call("POST", base, map[string]any{"name": "grant-cost", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201), "id")
	a, c := countedApplication(t, f)
	// Known app precedes canonical max UUID after sorting. Validation must check
	// complete tuples together, fail atomically and never combine source grants.
	in := Input{OperationID: f.operation(t), ExpectedPolicyRevision: 2, Grants: []Grant{{ResourceKind: "application", ResourceID: app, Action: "menu.enter", RowScope: "all", Fields: []string{}}, {ResourceKind: "application", ResourceID: "ffffffff-ffff-ffff-ffff-ffffffffffff", Action: "menu.enter", RowScope: "all", Fields: []string{}}}}
	c.reset()
	_, err := a.Write(context.Background(), principal(t, f), "grants.replace", app, gid, in, Metadata{RequestID: "cost-grants"})
	c.active.Store(false)
	if err != ErrResourceInvalid {
		t.Fatalf("mixed known/unknown resources must reject whole replacement: %v", err)
	}
	if count := c.menuValidation.Load(); count != 1 {
		t.Errorf("grant validation must issue one complete-tuple set query; got %d", count)
	}
	var rev, n int
	if err := f.owner.QueryRow(context.Background(), "SELECT policy_revision,(SELECT count(*) FROM applications.grants WHERE app_id=$1) FROM applications.apps WHERE id=$1", app).Scan(&rev, &n); err != nil || rev != 2 || n != 0 {
		t.Fatal("invalid tuple set must leave revision/grants unchanged")
	}
}
