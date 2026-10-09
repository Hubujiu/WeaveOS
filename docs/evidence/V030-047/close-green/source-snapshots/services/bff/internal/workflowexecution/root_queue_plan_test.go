package workflowexecution

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"
)

type rootQueueMeasurement struct {
	work, sortKiB, sharedHits, executionMs float64
	index                                  bool
	median, p95                            time.Duration
	bytesPerCall                           uint64
}

func rootQueueWork(node map[string]any) (float64, bool, float64) {
	number := func(key string) float64 { v, _ := node[key].(float64); return v }
	loops := number("Actual Loops")
	work := (number("Actual Rows") + number("Rows Removed by Filter") + number("Rows Removed by Index Recheck")) * loops
	found := node["Index Name"] == "ix_workflow_execution_due"
	memory := float64(0)
	if node["Sort Space Type"] == "Memory" {
		memory = number("Sort Space Used")
	}
	if children, ok := node["Plans"].([]any); ok {
		for _, child := range children {
			if m, ok := child.(map[string]any); ok {
				w, i, k := rootQueueWork(m)
				work += w
				found = found || i
				memory += k
			}
		}
	}
	return work, found, memory
}
func TestRootQueueClaimUsesBoundedDueIndexAtTwentyThousandRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated migrated PostgreSQL required")
	}
	pool, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, "SET LOCAL statement_timeout='15s'; SET LOCAL lock_timeout='2s'"); e != nil {
		t.Fatal(e)
	}
	// Scheduler-only synthetic rows, rolled back; never a claim of business authorization.
	var dueID string
	e = tx.QueryRow(ctx, `WITH inputs AS MATERIALIZED (
 SELECT gen_random_uuid() AS id,n FROM generate_series(1,20001) AS g(n)
 ), commands AS (
 INSERT INTO applications.workflow_commands(command_id,command_json,command_hash,state)
 SELECT id,jsonb_build_object('CommandID',id,'ProtocolVersion',CASE WHEN n<=10000 THEN 1 ELSE 2 END,'Action','withdraw'),decode(repeat('01',32),'hex'),'pending' FROM inputs
 RETURNING command_id
 ), queued AS (
 INSERT INTO applications.workflow_dispatch(command_id,protocol_version,next_attempt_at)
 SELECT c.command_id,CASE WHEN i.n<=10000 THEN 1 ELSE 2 END,
 CASE WHEN i.n=20001 THEN statement_timestamp()-interval '1 second' ELSE statement_timestamp()+interval '1 hour' END
 FROM commands c JOIN inputs i ON i.id=c.command_id RETURNING command_id
 ) SELECT q.command_id::text FROM queued q JOIN inputs i ON i.id=q.command_id WHERE i.n=20001`).Scan(&dueID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "ANALYZE applications.workflow_dispatch; ANALYZE applications.workflow_commands"); e != nil {
		t.Fatal(e)
	}
	measure := func(withoutIndex bool) rootQueueMeasurement {
		t.Helper()
		if _, e = tx.Exec(ctx, "SAVEPOINT root_queue_measure"); e != nil {
			t.Fatal(e)
		}
		if withoutIndex {
			if _, e = tx.Exec(ctx, "DROP INDEX applications.ix_workflow_execution_due"); e != nil {
				t.Fatal(e)
			}
		}
		if _, e = tx.Exec(ctx, "SET LOCAL ROLE auth_app; SAVEPOINT root_queue_one"); e != nil {
			t.Fatal(e)
		}
		var raw []byte
		if e = tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+claimSQL).Scan(&raw); e != nil {
			t.Fatal(e)
		}
		var result []map[string]any
		if e = json.Unmarshal(raw, &result); e != nil || len(result) != 1 {
			t.Fatalf("plan decode: %v", e)
		}
		plan, ok := result[0]["Plan"].(map[string]any)
		if !ok {
			t.Fatal("missing actual plan")
		}
		work, usedIndex, sortKiB := rootQueueWork(plan)
		hits, _ := plan["Shared Hit Blocks"].(float64)
		ms, _ := result[0]["Execution Time"].(float64)
		if _, e = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT root_queue_one; RELEASE SAVEPOINT root_queue_one"); e != nil {
			t.Fatal(e)
		}
		timings := make([]time.Duration, 0, 20)
		var allocations uint64
		for i := 0; i < 21; i++ {
			if _, e = tx.Exec(ctx, "SAVEPOINT root_queue_one"); e != nil {
				t.Fatal(e)
			}
			var id, token string
			var attempts int
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			e = tx.QueryRow(ctx, claimSQL).Scan(&id, &token, &attempts)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			if e != nil || id != dueID || token == "" || attempts != 1 {
				t.Fatalf("actual claim mismatch: %s %d %v", id, attempts, e)
			}
			if i > 0 {
				timings = append(timings, elapsed)
				allocations += after.TotalAlloc - before.TotalAlloc
			}
			if _, e = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT root_queue_one; RELEASE SAVEPOINT root_queue_one"); e != nil {
				t.Fatal(e)
			}
		}
		if _, e = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT root_queue_measure; RELEASE SAVEPOINT root_queue_measure"); e != nil {
			t.Fatal(e)
		}
		sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })
		return rootQueueMeasurement{work: work, index: usedIndex, sortKiB: sortKiB, sharedHits: hits, executionMs: ms, median: timings[len(timings)/2], p95: timings[18], bytesPerCall: allocations / 20}
	}
	indexed := measure(false)
	baseline := measure(true)
	if !indexed.index || indexed.work > 256 {
		t.Fatalf("due claim scans beyond bounded candidate set: index=%v work=%.0f", indexed.index, indexed.work)
	}
	if baseline.index || baseline.work < 10000 || baseline.work <= indexed.work*10 {
		t.Fatalf("negative index control did not exercise large scan: %+v / %+v", indexed, baseline)
	}
	var restored bool
	if e = tx.QueryRow(ctx, "SELECT to_regclass('applications.ix_workflow_execution_due') IS NOT NULL").Scan(&restored); e != nil || !restored {
		t.Fatal("negative control did not restore index")
	}
	var server string
	if e = tx.QueryRow(ctx, "SHOW server_version").Scan(&server); e != nil {
		t.Fatal(e)
	}
	t.Logf("isolated scheduler: 10000 legacy + 10000 future v2 + 1 due; Go=%s PG=%s; 20 warm query-only samples; rollback excludes all synthetic data", runtime.Version(), server)
	t.Logf("indexed work=%.0f PG_exec_ms=%.3f shared_hits=%.0f sort_memory_KiB=%.0f client_median=%s p95=%s client_alloc_B_per_call=%d", indexed.work, indexed.executionMs, indexed.sharedHits, indexed.sortKiB, indexed.median, indexed.p95, indexed.bytesPerCall)
	t.Logf("without_due_index work=%.0f PG_exec_ms=%.3f shared_hits=%.0f sort_memory_KiB=%.0f client_median=%s p95=%s client_alloc_B_per_call=%d", baseline.work, baseline.executionMs, baseline.sharedHits, baseline.sortKiB, baseline.median, baseline.p95, baseline.bytesPerCall)
	t.Log("Allocation is Go client query allocation, sort memory is only reported PG sort workspace; neither is whole-process RSS or production throughput. No latency ratio is asserted.")
}
