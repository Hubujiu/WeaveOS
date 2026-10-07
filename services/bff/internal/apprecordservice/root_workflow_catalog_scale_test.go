package apprecordservice

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rootCatalogQueryCapture struct {
	mu   sync.Mutex
	sql  string
	args []any
}

func (c *rootCatalogQueryCapture) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if strings.Contains(d.SQL, "workflow_versions") && strings.Contains(d.SQL, "workflow_instances") {
		c.mu.Lock()
		c.sql = d.SQL
		c.args = append([]any(nil), d.Args...)
		c.mu.Unlock()
	}
	return ctx
}
func (*rootCatalogQueryCapture) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func rootCatalogExaminedRows(v any) float64 {
	switch x := v.(type) {
	case []any:
		var n float64
		for _, e := range x {
			n += rootCatalogExaminedRows(e)
		}
		return n
	case map[string]any:
		loops, _ := x["Actual Loops"].(float64)
		if loops == 0 {
			loops = 1
		}
		rows, _ := x["Actual Rows"].(float64)
		filtered, _ := x["Rows Removed by Filter"].(float64)
		joined, _ := x["Rows Removed by Join Filter"].(float64)
		n := (rows + filtered + joined) * loops
		for k, e := range x {
			if k == "Plans" || k == "Plan" {
				n += rootCatalogExaminedRows(e)
			}
		}
		return n
	}
	return 0
}

// Count only the relation under this contract. Unrelated test fixtures can
// increase definitions/instances scan work without reading archived versions.
func rootCatalogVersionRows(v any) (rows float64, found bool) {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			n, ok := rootCatalogVersionRows(item)
			rows += n
			found = found || ok
		}
	case map[string]any:
		if x["Relation Name"] == "workflow_versions" {
			found = true
			loops, _ := x["Actual Loops"].(float64)
			n, _ := x["Actual Rows"].(float64)
			removed, _ := x["Rows Removed by Filter"].(float64)
			recheck, _ := x["Rows Removed by Index Recheck"].(float64)
			rows += (n + removed + recheck) * loops
		}
		for _, key := range []string{"Plan", "Plans"} {
			n, ok := rootCatalogVersionRows(x[key])
			rows += n
			found = found || ok
		}
	}
	return rows, found
}
func TestRootCatalogVersionWorkOracle(t *testing.T) {
	for _, tc := range []struct {
		plan  string
		want  float64
		found bool
	}{
		{`[{"Plan":{"Plans":[{"Relation Name":"workflow_definitions","Actual Rows":1,"Rows Removed by Filter":5000,"Actual Loops":1},{"Relation Name":"workflow_versions","Actual Rows":1,"Actual Loops":1}]}}]`, 1, true},
		{`[{"Plan":{"Relation Name":"workflow_versions","Actual Rows":1,"Rows Removed by Filter":4999,"Actual Loops":1}}]`, 5000, true},
		{`[{"Plan":{"Relation Name":"workflow_versions","Actual Rows":1,"Actual Loops":5000}}]`, 5000, true},
		{`[{"Plan":{"Relation Name":"workflow_definitions","Actual Rows":1,"Actual Loops":1}}]`, 0, false},
	} {
		var plan any
		if e := json.Unmarshal([]byte(tc.plan), &plan); e != nil {
			t.Fatal(e)
		}
		n, found := rootCatalogVersionRows(plan)
		if n != tc.want || found != tc.found {
			t.Fatalf("version-work oracle got %g/%v want %g/%v", n, found, tc.want, tc.found)
		}
	}
}
func TestRootCatalogCompatibilityDoesNotScanArchivedVersions(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, true))
	// One live version, 4,999 immutable retired versions. No business-row growth.
	_, e := f.owner.Exec(f.ctx, `INSERT INTO applications.workflow_versions
  (app_id,flow_id,version,version_id,schema_version,graph_json,bpmn_xml,allow_withdraw,created_by)
  SELECT app_id,flow_id,n,gen_random_uuid(),schema_version,graph_json,bpmn_xml,allow_withdraw,created_by
  FROM applications.workflow_versions CROSS JOIN generate_series(2,5000) n
  WHERE app_id=$1 AND flow_id=$2 AND version=1`, f.app, h.FlowID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "INSERT INTO applications.workflow_deployments(app_id,flow_id,version,deployment_id) VALUES($1,$2,5000,'scale-latest')", f.app, h.FlowID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_definitions SET current_version=5000,candidate_version=5000 WHERE app_id=$1 AND id=$2", f.app, h.FlowID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "ANALYZE applications.workflow_definitions; ANALYZE applications.workflow_versions; ANALYZE applications.workflow_instances"); e != nil {
		t.Fatal(e)
	}
	capture := &rootCatalogQueryCapture{}
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = capture
	p, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	tx, e := p.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	start := time.Now()
	got, e := (workflowcatalog.Catalog{}).CheckCompatibilityInTx(f.ctx, tx, f.app, f.table, rootCatalogFields(f, appquery.Text)[1:])
	elapsed := time.Since(start)
	if e != nil || len(got) != 1 || got[0].Version != 5000 {
		t.Fatalf("live dependency %+v %v", got, e)
	}
	capture.mu.Lock()
	sql, args := capture.sql, append([]any(nil), capture.args...)
	capture.mu.Unlock()
	if sql == "" {
		t.Fatal("did not observe actual catalog compatibility database query")
	}
	// EXPLAIN the exact production query and actual bind parameters, not a
	// lookalike query authored solely for this test.
	var raw []byte
	if e = tx.QueryRow(f.ctx, "EXPLAIN (ANALYZE,FORMAT JSON) "+sql, args...).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var plan any
	if e = json.Unmarshal(raw, &plan); e != nil {
		t.Fatal(e)
	}
	examined := rootCatalogExaminedRows(plan)
	t.Logf("catalog actual query: retired=4999 live=1 elapsed=%s aggregate_plan_rows=%g plan=%s", elapsed, examined, raw)
	versionRows, found := rootCatalogVersionRows(plan)
	t.Logf("catalog version relation examined_rows=%g", versionRows)
	if !found || versionRows > 8 {
		t.Fatalf("compatibility traversed archived history: version relation work=%g, found=%v for one live version; expected indexed live-key lookup independent of 4999 retired versions", versionRows, found)
	}
}
