package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryScaleTrace struct {
	mu                      sync.Mutex
	locked                  time.Time
	held                    time.Duration
	fullOutside, fullInside int
}

func (s *queryScaleTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.HasPrefix(d.SQL, memberCandidateSQL) && strings.Contains(d.SQL, "department_labels AS") {
		if s.locked.IsZero() {
			s.fullOutside++
		} else {
			s.fullInside++
		}
	}
	return context.WithValue(ctx, queryTraceSQLKey{}, d.SQL)
}
func (s *queryScaleTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	sql, _ := ctx.Value(queryTraceSQLKey{}).(string)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sql == "SELECT personnel.lock_query_revisions()" && d.Err == nil {
		s.locked = time.Now()
	}
	if (sql == "commit" || sql == "rollback") && !s.locked.IsZero() {
		s.held = time.Since(s.locked)
		s.locked = time.Time{}
	}
}
func TestQ36ScaleShallowDeepDifferentMatchesAndWriteLockDuration(t *testing.T) {
	f, _ := queryWeb(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("q36-scale-%d-", time.Now().UnixNano())
	target := newTarget(t, f.fixture)
	t.Cleanup(func() {
		for _, table := range []string{"personnel.department_members", "personnel.member_identities"} {
			_, _ = f.owner.Exec(ctx, "DELETE FROM "+table+" WHERE user_id IN (SELECT id FROM auth.users WHERE starts_with(account,$1))", prefix)
		}
		_, _ = f.owner.Exec(ctx, "DELETE FROM auth.users WHERE starts_with(account,$1)", prefix)
	})
	for _, sql := range []string{
		`INSERT INTO auth.users(account,created_at) SELECT $1||lpad(n::text,5,'0'),'2026-01-01'::timestamptz+n*interval '1 second' FROM generate_series(0,9999) n`,
		`INSERT INTO personnel.member_identities(user_id,identity_id) SELECT id,$2 FROM auth.users WHERE starts_with(account,$1)`,
		`INSERT INTO personnel.department_members(user_id,department_id) SELECT u.id,d.id FROM auth.users u CROSS JOIN personnel.departments d WHERE starts_with(u.account,$1) AND d.is_root`,
	} {
		args := []any{prefix}
		if strings.Contains(sql, "$2") {
			args = append(args, f.i1)
		}
		if _, err := f.owner.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"auth.users", "personnel.member_identities", "personnel.department_members"} {
		if _, err := f.owner.Exec(ctx, "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}
	tr := &queryScaleTrace{}
	cfg := f.app.Pool.Config()
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f.app.Pool = pool
	p := f.actor
	p.SessionRef = "11111111-1111-4111-8111-111111111111"
	for i, c := range []struct {
		suffix  string
		matches int
	}{{"000", 100}, {"00", 1000}, {"", 10000}} {
		search := prefix + c.suffix
		input := MemberQueryInput{MemberQuery: MemberQuery{PageQuery: PageQuery{Page: 1, PageSize: 20, Search: search}}}
		start := time.Now()
		baseline, err := f.app.SearchMembers(ctx, p, input)
		if err != nil || baseline.Total != int64(c.matches) {
			t.Fatalf("baseline M=%d total=%d err=%v", c.matches, baseline.Total, err)
		}
		initial := time.Since(start)
		input.QueryVersion = baseline.QueryVersion
		for _, page := range []int{1, c.matches / 20} {
			input.Page = page
			start = time.Now()
			got, err := f.app.SearchMembers(ctx, p, input)
			elapsed := time.Since(start)
			if err != nil || len(got.Items) != 20 || got.Total != int64(c.matches) {
				t.Fatal("bounded complete page", err)
			}
			want := prefix + fmt.Sprintf("%05d", (page-1)*20)
			if got.Items[0].Account != want {
				t.Fatalf("deep page first item: %s want %s", got.Items[0].Account, want)
			}
			sql, args, _, err := memberPageQuery(input)
			if err != nil {
				t.Fatal(err)
			}
			var raw []byte
			if err = f.owner.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("WEAVEOS_Q36_PLAN_DIR"); dir != "" {
				if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("member-M%d-page%d.json", c.matches, page)), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var plan []map[string]any
			if err = json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			t.Logf("N=10000 M=%d page=%d K=20 initial_full_ms=%.3f reused_query_ms=%.3f page_sql_ms=%v", c.matches, page, float64(initial)/float64(time.Millisecond), float64(elapsed)/float64(time.Millisecond), plan[0]["Execution Time"])
		}
		// Force a full original-query recheck, but preserve all selected members.
		if _, err = f.owner.Exec(ctx, "UPDATE personnel.identities SET description=$2 WHERE id=$1", f.i2, fmt.Sprint(c.matches)); err != nil {
			t.Fatal(err)
		}
		tr.mu.Lock()
		tr.fullOutside = 0
		tr.fullInside = 0
		tr.held = 0
		tr.mu.Unlock()
		ids := []string{f.i2}
		if i%2 == 1 {
			ids = []string{}
		}
		start = time.Now()
		_, err = f.app.SetMemberIdentities(ctx, p, target, ids, int64(i), RequestMetadata{RequestID: "q36-scale-write", QueryWriteGuard: QueryWriteGuard{QueryVersion: baseline.QueryVersion}})
		if err != nil {
			t.Fatal(err)
		}
		tr.mu.Lock()
		held, outside, inside := tr.held, tr.fullOutside, tr.fullInside
		tr.mu.Unlock()
		if outside != 1 || inside != 0 || held <= 0 {
			t.Fatalf("full validation outside locked transaction only: outside=%d inside=%d held=%v", outside, inside, held)
		}
		t.Logf("N=10000 M=%d mutation_total_ms=%.3f revision_locks_held_ms=%.3f full_projection_outside=%d inside=%d", c.matches, float64(time.Since(start))/float64(time.Millisecond), float64(held)/float64(time.Millisecond), outside, inside)
	}
}

func TestQ36EventScaleShallowDeepDifferentMatches(t *testing.T) {
	f, _ := queryWeb(t)
	ctx := context.Background()
	startTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := f.owner.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary,occurred_at)
 SELECT 'personnel_changed','success',$1,'IDENTITY_UPDATED','q36-event-scale','identity',$2,jsonb_build_object('before',jsonb_build_object('name','old'),'after',jsonb_build_object('name','new')),$3::timestamptz+n*interval '1 second' FROM generate_series(0,4999) n`, f.actor.UserID, f.i2, startTime); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, "ANALYZE auth.authentication_events"); err != nil {
		t.Fatal(err)
	}
	p := f.actor
	p.SessionRef = "11111111-1111-4111-8111-111111111111"
	for _, matches := range []int{100, 1000, 5000} {
		input := EventQueryInput{EventQuery: EventQuery{PageQuery: PageQuery{Page: 1, PageSize: 20}, From: startTime, To: startTime.Add(time.Duration(matches) * time.Second)}}
		start := time.Now()
		baseline, err := f.app.SearchEvents(ctx, p, input)
		if err != nil || baseline.Total != int64(matches) {
			t.Fatalf("event baseline M=%d total=%d err=%v", matches, baseline.Total, err)
		}
		initial := time.Since(start)
		input.QueryVersion = baseline.QueryVersion
		for _, page := range []int{1, matches / 20} {
			input.Page = page
			start = time.Now()
			got, err := f.app.SearchEvents(ctx, p, input)
			elapsed := time.Since(start)
			if err != nil || got.Total != int64(matches) || len(got.Items) != 20 {
				t.Fatal("complete event page", err)
			}
			want := startTime.Add(time.Duration(matches-(page-1)*20-1) * time.Second)
			if !got.Items[0].OccurredAt.Equal(want) {
				t.Fatal("deep event page must preserve full-result order")
			}
			sql, args, _, err := eventProjectionQuery(input, true)
			if err != nil {
				t.Fatal(err)
			}
			var raw []byte
			if err = f.owner.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("WEAVEOS_Q36_PLAN_DIR"); dir != "" {
				if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("event-M%d-page%d.json", matches, page)), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var plan []map[string]any
			if err = json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			t.Logf("events N=5000 M=%d page=%d K=20 initial_full_ms=%.3f reused_query_ms=%.3f page_sql_ms=%v", matches, page, float64(initial)/float64(time.Millisecond), float64(elapsed)/float64(time.Millisecond), plan[0]["Execution Time"])
		}
	}
}
