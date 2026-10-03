package appquery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Opt-in core capacity probe. It uses real PostgreSQL and FingerprintRows, but
// intentionally does not claim Session, Redis, live grants, reference display,
// dynamic form metadata or the shared Q36 lifecycle. Run only in isolation.
func TestCorePGScaleMatrix(t *testing.T) {
	if os.Getenv("WEAVEOS_V015_SCALE") != "1" {
		t.Skip("opt-in isolated PG scale probe")
	}
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	maxContexts := 200
	if raw := os.Getenv("WEAVEOS_V015_MAX_CONTEXTS"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 200 {
			t.Fatal("invalid max contexts")
		}
		maxContexts = n
	}
	onlyN, onlyContexts := 0, 0
	if raw := os.Getenv("WEAVEOS_V015_ONLY_N"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v != 10000 && v != 100000 && v != 1000000 {
			t.Fatal("invalid only N")
		}
		onlyN = v
	}
	if raw := os.Getenv("WEAVEOS_V015_ONLY_CONTEXTS"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v != 1 && v != 20 && v != 200 {
			t.Fatal("invalid only contexts")
		}
		onlyContexts = v
	}
	selectivity := os.Getenv("WEAVEOS_V015_SELECTIVITY")
	if selectivity != "" && selectivity != "broad" && selectivity != "ten_percent" && selectivity != "point_one_percent" {
		t.Fatal("invalid selectivity")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	for _, n := range []int{10000, 100000, 1000000} {
		if onlyN != 0 && n != onlyN {
			continue
		}
		name := fmt.Sprintf("v015_scale_%d", n)
		qualified := pgx.Identifier{name}.Sanitize()
		column := pgx.Identifier{"f_" + strings.ReplaceAll(fieldN, "-", "")}.Sanitize()
		create := `CREATE TEMP TABLE ` + qualified + ` (seq integer PRIMARY KEY,id uuid NOT NULL,created_by uuid NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,` + column + ` numeric NOT NULL,f_text text NOT NULL)`
		if _, err = db.Exec(ctx, create); err != nil {
			t.Fatal(err)
		}
		seedStart := time.Now()
		insert := `INSERT INTO ` + qualified + ` SELECT s,md5(s::text)::uuid,CASE WHEN s%2=0 THEN 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'::uuid ELSE 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'::uuid END,'2026-01-01 UTC'::timestamptz+s*interval '1 second','2026-01-01 UTC'::timestamptz+s*interval '1 second',(s%1000)::numeric,repeat('x',32)||s::text FROM generate_series(1,$1) s`
		if _, err = db.Exec(ctx, insert, n); err != nil {
			t.Fatal(err)
		}
		for _, index := range []string{
			`CREATE INDEX ON ` + qualified + `(created_at DESC,id DESC)`,
			`CREATE INDEX ON ` + qualified + `(created_by,created_at DESC,id DESC)`,
			`CREATE INDEX ON ` + qualified + `(` + column + `,id)`,
		} {
			if _, err = db.Exec(ctx, index); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = db.Exec(ctx, `ANALYZE `+qualified); err != nil {
			t.Fatal(err)
		}
		var totalRelationBytes int64
		if err = db.QueryRow(ctx, `SELECT pg_total_relation_size(to_regclass($1))`, `pg_temp.`+name).Scan(&totalRelationBytes); err != nil {
			t.Fatal(err)
		}
		t.Logf("SEED n=%d elapsed=%s relationAndIndexBytes=%d", n, time.Since(seedStart), totalRelationBytes)
		for _, contexts := range []int{1, 20, 200} {
			if onlyContexts != 0 && contexts != onlyContexts {
				continue
			}
			if contexts > maxContexts {
				t.Logf("NOT_RUN n=%d contexts=%d cap=%d", n, contexts, maxContexts)
				continue
			}
			where, expected := `TRUE`, n
			switch selectivity {
			case "ten_percent":
				where = `r.` + column + `<100`
				expected = n / 10
			case "point_one_percent":
				where = `r.` + column + `<1`
				expected = n / 1000
			}
			selectSQL := `SELECT jsonb_build_array(r.id,r.created_at,r.updated_at,r.` + column + `,r.f_text) FROM ` + qualified + ` r WHERE ` + where + ` ORDER BY r.created_at DESC,r.id DESC`
			var first Projection
			var streamed int64
			latencies := make([]time.Duration, 0, contexts)
			start := time.Now()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for i := 0; i < contexts; i++ {
				oneStart := time.Now()
				rows, e := db.Query(ctx, selectSQL)
				if e != nil {
					t.Fatal(e)
				}
				p, e := FingerprintRows(ctx, rows)
				if e != nil {
					t.Fatal(e)
				}
				if p.Total != int64(expected) {
					t.Fatalf("n=%d context=%d total=%d", n, i, p.Total)
				}
				if i == 0 {
					first = p
				} else if first != p {
					t.Fatalf("same projection changed within benchmark: n=%d context=%d", n, i)
				}
				streamed += p.Total
				latencies = append(latencies, time.Since(oneStart))
			}
			runtime.ReadMemStats(&after)
			var usage syscall.Rusage
			_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			quantile := func(p float64) time.Duration { index := int(p * float64(len(latencies)-1)); return latencies[index] }
			t.Logf("REHASH n=%d selectivity=%s contexts=%d rows=%d bytesPerContext=%d total=%s perContext=%s heapBefore=%d heapAfter=%d totalAllocDelta=%d fingerprint=%s", n, selectivity, contexts, streamed, first.StreamedBytes, time.Since(start), time.Since(start)/time.Duration(contexts), before.HeapAlloc, after.HeapAlloc, after.TotalAlloc-before.TotalAlloc, first.Fingerprint)
			t.Logf("REHASH_QUANTILES n=%d contexts=%d p50=%s p95=%s p99=%s", n, contexts, quantile(.5), quantile(.95), quantile(.99))
			t.Logf("GO_RSS n=%d contexts=%d maxRSSKiB=%d", n, contexts, usage.Maxrss)
		}
		for _, scope := range []struct {
			name, where string
			args        []any
		}{{"all", "TRUE", nil}, {"own", "created_by=$1", []any{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}}, {"selective", column + `<10`, nil}} {
			start := time.Now()
			var count int64
			q := `SELECT count(*) FROM ` + qualified + ` WHERE ` + scope.where
			if err = db.QueryRow(ctx, q, scope.args...).Scan(&count); err != nil {
				t.Fatal(err)
			}
			t.Logf("COUNT n=%d scope=%s rows=%d elapsed=%s", n, scope.name, count, time.Since(start))
		}
		for _, p := range []int{20, 100} {
			for _, offset := range []int{0, n / 2} {
				start := time.Now()
				rows, e := db.Query(ctx, `SELECT id::text FROM `+qualified+` ORDER BY created_at DESC,id DESC LIMIT $1 OFFSET $2`, p, offset)
				if e != nil {
					t.Fatal(e)
				}
				seen := 0
				for rows.Next() {
					var id string
					if e = rows.Scan(&id); e != nil {
						t.Fatal(e)
					}
					seen++
				}
				if e = rows.Err(); e != nil {
					t.Fatal(e)
				}
				rows.Close()
				t.Logf("PAGE n=%d pageSize=%d offset=%d rows=%d elapsed=%s", n, p, offset, seen, time.Since(start))
			}
		}
		filters := []struct {
			name string
			raw  json.RawMessage
		}{{"one", json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + fieldN + `","operator":"lt","value":"10"}]}`)}}
		children := make([]string, 20)
		for i := 0; i < 20; i++ {
			children[i] = fmt.Sprintf(`{"fieldId":"%s","operator":"eq","value":"%d"}`, fieldN, i)
		}
		filters = append(filters, struct {
			name string
			raw  json.RawMessage
		}{"twenty", json.RawMessage(`{"operator":"or","children":[` + strings.Join(children, ",") + `]}`)})
		for _, filter := range filters {
			plan, e := Compile(filter.raw, nil, []Field{{ID: fieldN, Kind: Number}}, 1)
			if e != nil {
				t.Fatal(e)
			}
			start := time.Now()
			var count int64
			if e = db.QueryRow(ctx, `SELECT count(*) FROM `+qualified+` r WHERE `+plan.Predicate, plan.Arguments...).Scan(&count); e != nil {
				t.Fatal(e)
			}
			t.Logf("FILTER_COUNT n=%d leaves=%s rows=%d elapsed=%s", n, filter.name, count, time.Since(start))
		}
		var explain []byte
		if err = db.QueryRow(ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) SELECT id FROM `+qualified+` ORDER BY created_at DESC,id DESC LIMIT 100 OFFSET $1`, n/2).Scan(&explain); err != nil {
			t.Fatal(err)
		}
		t.Logf("EXPLAIN_PAGE n=%d json=%s", n, explain)
		for _, kindQuery := range []struct{ name, sql string }{
			{"count", `SELECT count(*) FROM ` + qualified},
			{"full_projection", `SELECT jsonb_build_array(r.id,r.created_at,r.updated_at,r.` + column + `,r.f_text) FROM ` + qualified + ` r ORDER BY r.created_at DESC,r.id DESC`},
		} {
			var raw []byte
			if err = db.QueryRow(ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) `+kindQuery.sql).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var parsed []map[string]any
			if err = json.Unmarshal(raw, &parsed); err != nil || len(parsed) != 1 {
				t.Fatal("invalid EXPLAIN JSON", err)
			}
			plan, ok := parsed[0]["Plan"].(map[string]any)
			if !ok {
				t.Fatal("missing plan")
			}
			t.Logf("PLAN n=%d kind=%s executionMs=%v node=%v localHit=%v localRead=%v tempRead=%v tempWritten=%v", n, kindQuery.name, parsed[0]["Execution Time"], plan["Node Type"], plan["Local Hit Blocks"], plan["Local Read Blocks"], plan["Temp Read Blocks"], plan["Temp Written Blocks"])
		}
	}
}
