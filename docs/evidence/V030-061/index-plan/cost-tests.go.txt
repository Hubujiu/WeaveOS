package apprecordservice

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Synthetic candidate access path only. This does not measure authorization,
// graph reads or production throughput. Temporary objects roll back in full.
func TestRootWorkflowInboxCandidateAccessPaths(t *testing.T) {
	f := newRecordFixture(t)
	tx, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	_, e = tx.Exec(f.ctx, `CREATE TEMP TABLE inbox_cost(id uuid PRIMARY KEY,app_id uuid NOT NULL,assignee_id uuid NOT NULL,created_at timestamptz NOT NULL,closed_command_id uuid) ON COMMIT DROP;
 INSERT INTO inbox_cost SELECT md5('task-'||n)::uuid,md5('app-'||(n%37))::uuid,md5('actor-'||(n%1000))::uuid,'2026-01-01T00:00:00Z'::timestamptz+n*interval '1 second',CASE WHEN n%7=0 THEN md5('closed')::uuid ELSE NULL END FROM generate_series(1,100000)n;
 CREATE INDEX inbox_cost_app_order ON inbox_cost(app_id,assignee_id,created_at,id) WHERE closed_command_id IS NULL;
 ANALYZE inbox_cost;`)
	if e != nil {
		t.Fatal(e)
	}
	want := []string{}
	for n := 100000; n >= 1 && len(want) < 20; n-- {
		if n%1000 == 1 && n%7 != 0 {
			want = append(want, fmt.Sprintf("2026-01-%02dT%02d:%02d:%02dZ", 1+n/86400, n/3600%24, n/60%60, n%60))
		}
	}
	q := `SELECT to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM inbox_cost WHERE assignee_id=md5('actor-1')::uuid AND closed_command_id IS NULL ORDER BY created_at DESC,id DESC LIMIT 20`
	for _, mode := range []string{"existing_app_prefix", "personal_actor_prefix"} {
		if mode == "personal_actor_prefix" {
			if _, e = tx.Exec(f.ctx, "CREATE INDEX inbox_cost_personal_order ON inbox_cost(assignee_id,created_at DESC,id DESC) WHERE closed_command_id IS NULL; ANALYZE inbox_cost"); e != nil {
				t.Fatal(e)
			}
		}
		rows, e := tx.Query(f.ctx, q)
		if e != nil {
			t.Fatal(e)
		}
		got := []string{}
		for rows.Next() {
			var v string
			if e = rows.Scan(&v); e != nil {
				t.Fatal(e)
			}
			got = append(got, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("candidate result changed", mode, got, want, e)
		}
		var plan json.RawMessage
		if e = tx.QueryRow(f.ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+q).Scan(&plan); e != nil {
			t.Fatal(e)
		}
		var bytes int64
		if mode == "personal_actor_prefix" {
			if e = tx.QueryRow(f.ctx, "SELECT pg_relation_size('inbox_cost_personal_order')").Scan(&bytes); e != nil {
				t.Fatal(e)
			}
		}
		t.Logf("candidate access path mode=%s rows=100000 actor_candidates=100 new_index_bytes=%d plan=%s", mode, bytes, plan)
	}
}
