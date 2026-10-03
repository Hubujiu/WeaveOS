package appquery

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type compactScaleResult struct {
	compactCacheResult
	rowsTime, refsTime time.Duration
	payload            int64
}

// A test-only compact projection: SQL still determines the exact authorized
// matching row sequence. This probe intentionally has no production hook.
func runCompactScale(ctx context.Context, db *pgx.Conn, threshold int) (compactScaleResult, error) {
	var result compactScaleResult
	h := sha256.New()
	_, _ = h.Write([]byte("v015/compact-scale/v1|number:all;text:all;secret:own;ref:all\x00"))
	rowsStart := time.Now()
	rows, err := db.Query(ctx, `SELECT r.id,r.record_version,r.created_by=$1::uuid FROM v015_compact_scale_rows r WHERE r.number >= $2 ORDER BY r.created_at DESC,r.id DESC`, compactActor, threshold)
	if err != nil {
		return result, err
	}
	var version [8]byte
	for rows.Next() {
		var id pgtype.UUID
		var recordVersion int64
		var own bool
		if err = rows.Scan(&id, &recordVersion, &own); err != nil {
			break
		}
		if !id.Valid || recordVersion < 1 || recordVersion > MaxJSONVersion {
			err = ErrProjectionInvalid
			break
		}
		_, _ = h.Write(id.Bytes[:])
		binary.BigEndian.PutUint64(version[:], uint64(recordVersion))
		_, _ = h.Write(version[:])
		if own {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		result.count++
		result.payload += 25 // binary column payload only, excludes PG protocol framing
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	result.rowsTime = time.Since(rowsStart)
	binary.BigEndian.PutUint64(version[:], uint64(result.count))
	_, _ = h.Write(version[:])
	refsStart := time.Now()
	refs, err := db.Query(ctx, `SELECT DISTINCT s.id,s.display,s.deleted FROM v015_compact_scale_rows r JOIN v015_compact_scale_sources s ON s.id=r.ref_id WHERE r.number >= $1 ORDER BY s.id,s.display,s.deleted`, threshold)
	if err != nil {
		return result, err
	}
	_, _ = h.Write([]byte("visible-references\x00"))
	for refs.Next() {
		var id pgtype.UUID
		var label string
		var deleted bool
		if err = refs.Scan(&id, &label, &deleted); err != nil {
			break
		}
		if !id.Valid {
			err = ErrProjectionInvalid
			break
		}
		_, _ = h.Write(id.Bytes[:])
		binary.BigEndian.PutUint64(version[:], uint64(len(label)))
		_, _ = h.Write(version[:])
		_, _ = h.Write([]byte(label))
		if deleted {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		result.payload += int64(25 + len(label))
	}
	if err == nil {
		err = refs.Err()
	}
	refs.Close()
	if err != nil {
		return result, err
	}
	result.refsTime = time.Since(refsStart)
	result.digest = hex.EncodeToString(h.Sum(nil))
	result.revision = "data=7;policy=3;schema=2;view=2;source=5"
	return result, nil
}

func TestCompactPGScale(t *testing.T) {
	if os.Getenv("WEAVEOS_V015_COMPACT_SCALE") != "1" {
		t.Skip("opt-in isolated compact A capacity probe")
	}
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	n := 1000000
	if raw := os.Getenv("WEAVEOS_V015_COMPACT_N"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v != 10000 && v != 100000 && v != 1000000 {
			t.Fatal("invalid N")
		}
		n = v
	}
	stage := os.Getenv("WEAVEOS_V015_COMPACT_STAGE")
	if stage != "same" && stage != "distinct" {
		t.Fatal("stage must be same or distinct")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_compact_scale_sources(id uuid PRIMARY KEY,display text NOT NULL,deleted boolean NOT NULL DEFAULT false);
 INSERT INTO v015_compact_scale_sources(id,display) SELECT md5(s::text)::uuid,'label-'||s::text FROM generate_series(0,99)s;
 CREATE TEMP TABLE v015_compact_scale_rows(seq integer PRIMARY KEY,id uuid NOT NULL,created_by uuid NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,record_version bigint NOT NULL,number numeric NOT NULL,public_text text NOT NULL,secret text NOT NULL,ref_id uuid NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	seedStart := time.Now()
	_, err = db.Exec(ctx, `INSERT INTO v015_compact_scale_rows SELECT s,md5(s::text)::uuid,CASE WHEN s%2=0 THEN 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'::uuid ELSE 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'::uuid END,'2026-01-01 UTC'::timestamptz+s*interval '1 second','2026-01-01 UTC'::timestamptz+s*interval '1 second',1,(s%1000)::numeric,repeat('x',32)||s::text,repeat('secret',5),md5((s%100)::text)::uuid FROM generate_series(1,$1)s`, n)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`CREATE INDEX ON v015_compact_scale_rows(created_at DESC,id DESC)`, `CREATE INDEX ON v015_compact_scale_rows(number,id)`, `CREATE INDEX ON v015_compact_scale_rows(ref_id)`, `ANALYZE v015_compact_scale_rows`} {
		if _, err = db.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	var storage int64
	if err = db.QueryRow(ctx, `SELECT pg_total_relation_size('pg_temp.v015_compact_scale_rows'::regclass)`).Scan(&storage); err != nil {
		t.Fatal(err)
	}
	t.Logf("SEED stage=%s n=%d elapsed=%s tableAndIndexesBytes=%d", stage, n, time.Since(seedStart), storage)
	// Same exact criterion and effective scope: 200 independent hypothetical
	// Session contexts use a shared bounded digest cache only after their own
	// live Session/policy validation (not modeled here).
	cache := newCompactExperimentCache(32)
	start := time.Now()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var misses int
	times := make([]time.Duration, 0, 200)
	var first compactScaleResult
	for i := 0; i < 200; i++ {
		threshold := -1
		if stage == "distinct" {
			threshold = -(i + 1)
		} // 200 different canonical filters, all match N rows
		key := fmt.Sprintf("actor=%s|mask=number:all;text:all;secret:own;ref:all|table=fixture|number>=%d|data=7;policy=3;schema=2;view=2;source=5", compactActor, threshold)
		began := time.Now()
		var measured compactScaleResult
		got, e := cache.get(ctx, key, time.Now(), func(ctx context.Context) (compactCacheResult, error) {
			misses++
			var computeErr error
			measured, computeErr = runCompactScale(ctx, db, threshold)
			return measured.compactCacheResult, computeErr
		})
		if e != nil {
			t.Fatal(e)
		}
		if got.count != int64(n) {
			t.Fatalf("context %d count=%d", i, got.count)
		}
		if i == 0 {
			first = measured
		} else if got.digest != first.digest {
			t.Fatalf("same matching P had different compact signature at %d", i)
		}
		times = append(times, time.Since(began))
	}
	runtime.ReadMemStats(&after)
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	quant := func(p float64) time.Duration { return times[int(p*float64(len(times)-1))] }
	t.Logf("RESULT stage=%s n=%d contexts=200 misses=%d total=%s p50=%s p95=%s p99=%s cacheEntries=%d firstRows=%s firstRefs=%s firstPayloadBytes=%d allocDelta=%d heapAfter=%d", stage, n, misses, time.Since(start), quant(.5), quant(.95), quant(.99), cache.size(), first.rowsTime, first.refsTime, first.payload, after.TotalAlloc-before.TotalAlloc, after.HeapAlloc)
	if stage == "same" && misses != 1 || stage == "distinct" && misses != 200 {
		t.Fatalf("wrong cache misses: %d", misses)
	}
	if stage == "same" {
		fullStart := time.Now()
		rows, e := db.Query(ctx, `SELECT jsonb_build_array(r.id,r.created_by,r.created_at,r.updated_at,r.record_version,r.number,r.public_text,CASE WHEN r.created_by=$1::uuid THEN jsonb_build_object('secret',r.secret) ELSE '{}'::jsonb END,jsonb_build_object('id',r.ref_id,'display',s.display,'deleted',s.deleted)) FROM v015_compact_scale_rows r JOIN v015_compact_scale_sources s ON s.id=r.ref_id WHERE r.number >= -1 ORDER BY r.created_at DESC,r.id DESC`, compactActor)
		if e != nil {
			t.Fatal(e)
		}
		full, e := FingerprintRows(ctx, rows)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("FULL_ORACLE n=%d rows=%d total=%s jsonBytes=%d", n, full.Total, time.Since(fullStart), full.StreamedBytes)
	}
}
