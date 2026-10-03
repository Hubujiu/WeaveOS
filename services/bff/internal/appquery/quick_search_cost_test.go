package appquery

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Opt-in PostgreSQL cost probe for the lead-selected literal ASCII-folded
// substring rule. It does not install a runtime query route or new index.
func TestQuickSearchLiteralPGCost(t *testing.T) {
	if os.Getenv("WEAVEOS_V015_QUICK_COST") != "1" {
		t.Skip("opt-in literal search PG cost probe")
	}
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_quick_cost(id integer PRIMARY KEY,body text NOT NULL);
 INSERT INTO v015_quick_cost SELECT s,'Prefix-'||s::text||repeat('x',16) FROM generate_series(1,1000000)s;
 INSERT INTO v015_quick_cost VALUES(1000001,'literal%_\end'),(1000002,'Éclair');
 CREATE INDEX v015_quick_cost_body_idx ON v015_quick_cost(body);
 ANALYZE v015_quick_cost;`)
	if err != nil {
		t.Fatal(err)
	}
	const predicate = `strpos(translate(body,'ABCDEFGHIJKLMNOPQRSTUVWXYZ','abcdefghijklmnopqrstuvwxyz'),translate($1,'ABCDEFGHIJKLMNOPQRSTUVWXYZ','abcdefghijklmnopqrstuvwxyz'))>0`
	for _, c := range []struct {
		term string
		want int64
	}{{"PREFIX", 1000000}, {"999999", 1}, {"%_\u005c", 1}, {"éclair", 0}, {"ÉCLAIR", 1}} {
		var count int64
		start := time.Now()
		err = db.QueryRow(ctx, `SELECT count(*) FROM v015_quick_cost WHERE `+predicate, c.term).Scan(&count)
		if err != nil || count != c.want {
			t.Fatalf("term=%q count=%d want=%d err=%v", c.term, count, c.want, err)
		}
		t.Logf("SEARCH term=%q count=%d elapsed=%s", c.term, count, time.Since(start))
	}
	for _, term := range []string{"PREFIX", "999999"} {
		var raw []byte
		if err = db.QueryRow(ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) SELECT count(*) FROM v015_quick_cost WHERE `+predicate, term).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var x []map[string]any
		if err = json.Unmarshal(raw, &x); err != nil || len(x) != 1 {
			t.Fatal(err)
		}
		t.Logf("PLAN term=%q executionMs=%v plan=%v", term, x[0]["Execution Time"], x[0]["Plan"])
	}
	var bytes int64
	if err = db.QueryRow(ctx, `SELECT pg_total_relation_size('pg_temp.v015_quick_cost'::regclass)`).Scan(&bytes); err != nil {
		t.Fatal(err)
	}
	t.Logf("STORAGE rows=1000002 tableAndIndexesBytes=%d", bytes)
}
