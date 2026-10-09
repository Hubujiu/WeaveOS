//go:build weaveos_cost

package apprecordservice

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Synthetic SQL access-path comparison, not a production latency claim.
// Only transaction-local temporary tables/indexes are mutated.
func TestRootWorkflowReadSyntheticAccessPaths(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	conn, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close(context.Background())
	tx, e := conn.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `CREATE TEMP TABLE root_read_cost_instances (LIKE applications.workflow_instances INCLUDING ALL EXCLUDING INDEXES) ON COMMIT DROP;
ALTER TABLE root_read_cost_instances ADD PRIMARY KEY (id);
CREATE TEMP TABLE root_read_cost_definitions(app_id uuid NOT NULL,id uuid PRIMARY KEY,name text NOT NULL) ON COMMIT DROP;
INSERT INTO root_read_cost_definitions VALUES('10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000003','Synthetic flow');
INSERT INTO root_read_cost_instances(id,app_id,table_id,flow_id,view_id,record_id,initiator_id,definition_version,state,sequence,created_at,updated_at)
SELECT md5('instance-'||n)::uuid,'10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000004',md5('record-'||(n%100))::uuid,'10000000-0000-4000-8000-000000000005',1,'completed',1,'2026-01-01'::timestamptz+n*interval '1 second','2026-01-01'::timestamptz+n*interval '1 second' FROM generate_series(1,100000)n;
ANALYZE root_read_cost_instances; ANALYZE root_read_cost_definitions;`); e != nil {
		t.Fatal(e)
	}
	var record string
	if e = tx.QueryRow(ctx, "SELECT md5('record-0')::uuid::text").Scan(&record); e != nil {
		t.Fatal(e)
	}
	args := []any{"10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002", record}
	base := strings.ReplaceAll(strings.ReplaceAll(workflowReadSelect, "applications.workflow_instances", "root_read_cost_instances"), "applications.workflow_definitions", "root_read_cost_definitions") + workflowReadOrder
	var expected []string
	// 100,000 rows across 100 records: record 0 has n=100,200,...,100000.
	// In descending timestamp order, OFFSET 980 contains n=2000 down to 100.
	for n := 2000; n >= 100; n -= 100 {
		hash := md5.Sum([]byte(fmt.Sprintf("instance-%d", n)))
		expected = append(expected, fmt.Sprintf("%x-%x-%x-%x-%x", hash[:4], hash[4:6], hash[6:8], hash[8:10], hash[10:]))
	}
	for _, variant := range []string{"no_record_index", "prefix", "ordered"} {
		if variant != "no_record_index" {
			suffix := ""
			if variant == "ordered" {
				suffix = ",created_at DESC,id DESC"
			}
			if _, e = tx.Exec(ctx, "CREATE INDEX root_read_cost_candidate ON root_read_cost_instances(app_id,table_id,record_id"+suffix+")"); e != nil {
				t.Fatal(e)
			}
		}
		page := base + " LIMIT 20 OFFSET 980"
		rows, e := tx.Query(ctx, page, args...)
		if e != nil {
			t.Fatal(e)
		}
		var actual []string
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				t.Fatal(e)
			}
			var row WorkflowInstanceSummary
			if e = json.Unmarshal(raw, &row); e != nil {
				t.Fatal(e)
			}
			actual = append(actual, row.ID)
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("variant %s returned wrong independent deep-page identities: got %v want %v", variant, actual, expected)
		}
		indexRows, e := tx.Query(ctx, "SELECT pg_get_indexdef(indexrelid) FROM pg_index WHERE indrelid='root_read_cost_instances'::regclass ORDER BY indexrelid")
		if e != nil {
			t.Fatal(e)
		}
		var indexes []string
		for indexRows.Next() {
			var definition string
			if e = indexRows.Scan(&definition); e != nil {
				t.Fatal(e)
			}
			indexes = append(indexes, definition)
		}
		if e = indexRows.Err(); e != nil {
			t.Fatal(e)
		}
		indexRows.Close()
		expectedCount := 2
		if variant == "no_record_index" {
			expectedCount = 1
		}
		if len(indexes) != expectedCount {
			t.Fatalf("confounded %s index set: %v", variant, indexes)
		}
		t.Logf("ACCESS_PATH_INDEXES variant=%s definitions=%q", variant, indexes)
		var size int64
		if variant != "no_record_index" {
			if e = tx.QueryRow(ctx, "SELECT pg_relation_size('root_read_cost_candidate')").Scan(&size); e != nil {
				t.Fatal(e)
			}
		}
		for _, query := range []struct{ name, sql string }{{"first", base + " LIMIT 20"}, {"deep", page}, {"fingerprint", base}} {
			for sample := 0; sample < 3; sample++ {
				var plan []byte
				if e = tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query.sql, args...).Scan(&plan); e != nil {
					t.Fatal(e)
				}
				t.Logf("ACCESS_PATH variant=%s query=%s sample=%d rows=100000 related=1000 index_bytes=%d plan=%s", variant, query.name, sample, size, plan)
			}
		}
		if variant != "no_record_index" {
			if _, e = tx.Exec(ctx, "DROP INDEX root_read_cost_candidate"); e != nil {
				t.Fatal(e)
			}
		}
	}
}
