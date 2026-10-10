package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func historyRequest(f rootTaskFixture, size int) WorkflowEventHistoryRequest {
	return WorkflowEventHistoryRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, PageSize: size}
}
func historyNoEffect(t *testing.T, f rootTaskFixture) string {
	t.Helper()
	c, p := f.rootProjection.accept(t, "agree", f.task.ID, "", 1, 1)
	r, b := f.receipt(t, c, "unchanged", "task_inactive")
	f.apply(t, c, p, r, b, true)
	return c.CommandID
}
func historyRead(t *testing.T, f rootTaskFixture, p session.Principal, q WorkflowEventHistoryRequest) WorkflowEventHistoryResult {
	t.Helper()
	v, e := f.service.ReadWorkflowEventHistory(f.ctx, p, q)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func historyStartID(t *testing.T, f rootTaskFixture) string {
	t.Helper()
	var id string
	if e := f.owner.QueryRow(f.ctx, "SELECT command_id::text FROM applications.workflow_execution_events WHERE instance_id=$1 AND action='start'", f.instance.ID).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
func TestRootEventHistoryDeterministicPagesAndSafeSummary(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	ids := []string{historyStartID(t, f)}
	for i := 0; i < 4; i++ {
		ids = append(ids, historyNoEffect(t, f))
	}
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET created_at='2026-01-01T00:00:00Z' WHERE app_id=$1", f.app)
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	q := historyRequest(f, 2)
	seen := []string{}
	for page := 0; page < 3; page++ {
		r := historyRead(t, f, f.principal, q)
		want := 2
		if page == 2 {
			want = 1
		}
		if len(r.Items) != want || r.HasMore != (page < 2) || (r.NextPageToken != nil) != (page < 2) {
			t.Fatalf("wrong page %d %+v", page, r)
		}
		for _, v := range r.Items {
			seen = append(seen, v.ID)
			detail := eventRead(t, f, f.principal, WorkflowEventRequest{f.app, f.view, f.ownRecord, v.ID})
			if !reflect.DeepEqual(v, detail.Event) {
				t.Fatal("summary differs from confirmed detail")
			}
			raw, _ := json.Marshal(v)
			var obj map[string]any
			json.Unmarshal(raw, &obj)
			if len(obj) != 14 {
				t.Fatal("unsafe DTO", obj)
			}
		}
		if r.NextPageToken != nil {
			q.PageToken = *r.NextPageToken
		}
	}
	if !reflect.DeepEqual(seen, ids) {
		t.Fatalf("same-time lost/duplicated events got %v want %v", seen, ids)
	}
}
func TestRootEventHistoryNewEventsDoNotJumpBack(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	old := historyStartID(t, f)
	middle := historyNoEffect(t, f)
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET created_at='2026-01-01T00:00:00Z' WHERE command_id=$1", old)
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET created_at='2026-01-02T00:00:00Z' WHERE command_id=$1", middle)
	q := historyRequest(f, 1)
	first := historyRead(t, f, f.principal, q)
	if len(first.Items) != 1 || first.Items[0].ID != middle || first.NextPageToken == nil {
		t.Fatal("wrong first cursor")
	}
	latest := historyNoEffect(t, f)
	q.PageToken = *first.NextPageToken
	r := historyRead(t, f, f.principal, q)
	if len(r.Items) != 1 || r.Items[0].ID != old || r.HasMore {
		t.Fatal("new event shifted old page chain", r)
	}
	q.PageToken = ""
	r = historyRead(t, f, f.principal, q)
	if r.Items[0].ID != latest {
		t.Fatal("fresh first page missed event")
	}
}
func TestRootEventHistoryAuthorityAndResourceIsolation(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := historyRequest(f, 20)
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("history-free actor saw events", e)
	}
	eventGrant(t, f, "all", f.public)
	r := historyRead(t, f, f.principal, q)
	if len(r.Items) != 1 {
		t.Fatal("start missing")
	}
	q.RecordID = f.otherRecord
	r = historyRead(t, f, f.principal, q)
	if r.Items == nil || len(r.Items) != 0 || r.HasMore || r.NextPageToken != nil {
		t.Fatal("other record inherited events", r)
	}
	q.RecordID = f.ownRecord
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app)
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read'", f.app)
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, q); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("revoked actor read events", e)
	}
}
func TestRootEventHistoryCursorCannotCrossBinding(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	historyNoEffect(t, f)
	q := historyRequest(f, 1)
	first := historyRead(t, f, f.principal, q)
	if first.NextPageToken == nil {
		t.Fatal("no continuation")
	}
	q.PageToken = *first.NextPageToken
	p := f.principal
	p.SessionRef = recordOperationID(t, f.recordFixture)
	p.Record.SessionRef = p.SessionRef
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, p, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("session splice accepted", e)
	}
	bad := q
	bad.PageSize = 2
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("page size splice", e)
	}
	bad = q
	bad.RecordID = f.otherRecord
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("record splice", e)
	}
	bad = q
	bad.PageToken = "bad"
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("bad token", e)
	}
	eventSQL(t, f, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", f.app)
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("policy revision stale cursor", e)
	}
}
func TestRootEventHistoryCorruptLedgerFailsNoPartialPage(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	id := historyNoEffect(t, f)
	eventSQL(t, f, "UPDATE applications.workflow_commands SET command_hash=decode(repeat('00',32),'hex') WHERE command_id=$1", id)
	r, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, 20))
	if !errors.Is(e, ErrUnavailable) || len(r.Items) != 0 || r.NextPageToken != nil {
		t.Fatal("corrupt ledger partial page", r, e)
	}
}

func TestRootEventHistoryExplicitExpiryAndDomainIsolation(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	historyNoEffect(t, f)
	q := historyRequest(f, 1)
	first := historyRead(t, f, f.principal, q)
	q.PageToken = *first.NextPageToken
	m, e := f.service.WorkflowEventHistory.Load(f.ctx, f.principal.SessionRef, q.PageToken)
	if e != nil {
		t.Fatal(e)
	}
	var c eventHistoryCursor
	if json.Unmarshal(m.Criteria, &c) != nil {
		t.Fatal("cursor fixture")
	}
	ttl := c.Expires - time.Now().Unix()
	if ttl < 590 || ttl > 600 {
		t.Fatal("not fixed ten minute expiry", ttl)
	}
	// Explicit past expiry, not a wall-clock sleep or a fake successful TTL test.
	c.Expires = 1
	m.Criteria, _ = json.Marshal(c)
	m.Fingerprint = eventHistoryFingerprint(m.Criteria)
	expired, e := f.service.WorkflowEventHistory.Create(f.ctx, f.principal.SessionRef, m)
	if e != nil {
		t.Fatal(e)
	}
	bad := q
	bad.PageToken = expired
	if _, e = f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("explicitly expired cursor accepted", e)
	}
	opts, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opts)
	defer client.Close()
	foreign := querycontext.NewStore(client, "foreign-history", "consumer-"+strings.ReplaceAll(f.app, "-", ""), querycontext.Policy{Validate: func(querycontext.Metadata) bool { return true }})
	m, e = f.service.WorkflowEventHistory.Load(f.ctx, f.principal.SessionRef, q.PageToken)
	if e != nil {
		t.Fatal(e)
	}
	token, e := foreign.Create(f.ctx, f.principal.SessionRef, m)
	if e != nil {
		t.Fatal(e)
	}
	bad.PageToken = token
	if _, e = f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("other token namespace crossed", e)
	}
	// Re-reading a token must not rewrite its application-level expiration.
	again, e := f.service.WorkflowEventHistory.Load(f.ctx, f.principal.SessionRef, q.PageToken)
	if e != nil || !reflect.DeepEqual(again.Criteria, m.Criteria) {
		t.Fatal("touch extended cursor expiry", e)
	}
}
func TestRootEventHistoryRedisFailureNoPartialSuccess(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	historyNoEffect(t, f)
	opts, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opts)
	if e = client.Close(); e != nil {
		t.Fatal(e)
	}
	s := *f.service
	s.WorkflowEventHistory = newWorkflowEventHistoryStore(client, "closed-history")
	r, e := s.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, 1))
	if e == nil || len(r.Items) != 0 || r.NextPageToken != nil {
		t.Fatal("failed cursor storage returned partial page", r, e)
	}
}
func TestRootEventHistoryCrossFlowAndPending(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	graph := rootCatalogGraph(t, f.recordFixture, false)
	h := rootCatalogReady(t, f.recordFixture, graph)
	instance := rootCatalogReserve(t, f.recordFixture, rootCatalogReserveInput(t, f.recordFixture, h))
	var version string
	if e := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); e != nil {
		t.Fatal(e)
	}
	p := rootProjection{recordFixture: f.recordFixture, head: h, instance: instance, version: version, first: graph.Nodes[1].ID, second: graph.Nodes[1].ID}
	c, payload := rootRecoveryAccept(t, p)
	r := historyRead(t, f, f.principal, historyRequest(f, 20))
	if len(r.Items) != 1 {
		t.Fatal("pending second flow became confirmed")
	}
	task := p.task(t, p.first, p.actor, 1)
	receipt, body := p.receipt(t, c, "active", "", task)
	p.apply(t, c, payload, receipt, body, true)
	r = historyRead(t, f, f.principal, historyRequest(f, 20))
	if len(r.Items) != 2 || r.Items[0].ID != c.CommandID || r.Items[0].FlowID != h.FlowID || r.Items[1].FlowID != f.head.FlowID {
		t.Fatal("cross-flow history wrong", r)
	}
}
func TestRootEventHistoryWithdrawReturnSummary(t *testing.T) {
	for _, action := range []string{"withdraw", "return"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			eventGrant(t, f, "all", f.public)
			op := rootLifecycleAccept(t, f, rootLifecycleActionReq(t, f, action))
			c, payload := rootActionRead(t, f, op)
			state := "withdrawn"
			var tasks []fc.ExecutionTask
			if action == "return" {
				state = "active"
				tasks = []fc.ExecutionTask{f.rootProjection.task(t, f.first, f.actor, 2)}
			}
			receipt, body := f.receipt(t, c, state, "", tasks...)
			f.apply(t, c, payload, receipt, body, true)
			list := historyRead(t, f, f.principal, historyRequest(f, 20))
			if len(list.Items) != 2 || list.Items[0].ID != c.CommandID || list.Items[0].Action != action || list.Items[0].Outcome != "success" {
				t.Fatal("wrong confirmed lifecycle", list)
			}
			event := list.Items[0]
			if action == "withdraw" && (event.NodeID != nil || event.TargetNodeID != nil) {
				t.Fatal("withdraw invented node")
			}
			if action == "return" && (event.NodeID == nil || *event.NodeID != f.first || event.TargetNodeID == nil || *event.TargetNodeID != f.first) {
				t.Fatal("return lost source/target")
			}
			detail := eventRead(t, f, f.principal, WorkflowEventRequest{f.app, f.view, f.ownRecord, c.CommandID})
			if !reflect.DeepEqual(detail.Event, event) {
				t.Fatal("detail/list lifecycle mismatch")
			}
		})
	}
}
func TestRootEventHistoryOneSnapshotAndLiveNextRequest(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	id := historyNoEffect(t, f)
	trace := &eventSnapshotTrace{change: func() error {
		_, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.history')", f.app)
		if e != nil {
			return e
		}
		_, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_commands SET command_hash=decode(repeat('00',32),'hex') WHERE command_id=$1", id)
		return e
	}}
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = trace
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	s := *f.service
	s.Pool = pool
	r, e := s.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, 20))
	if !trace.changed || trace.err != nil || e != nil || len(r.Items) != 2 {
		t.Fatal("mixed history/ledger snapshots", e, trace.err)
	}
	if _, e = s.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, 20)); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("next request used old authority", e)
	}
	owner := f.principal
	owner.UserID = f.other
	owner.SessionRef = f.other
	if _, e = s.ReadWorkflowEventHistory(f.ctx, owner, historyRequest(f, 20)); !errors.Is(e, ErrUnavailable) {
		t.Fatal("fresh owner did not observe corrupt ledger", e)
	}
}

type historyCostTrace struct {
	mu      sync.Mutex
	queries []string
}

func (t *historyCostTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	t.mu.Lock()
	t.queries = append(t.queries, d.SQL)
	t.mu.Unlock()
	return ctx
}
func (*historyCostTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func TestRootEventHistoryBoundedBatchCost(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	for i := 0; i < 100; i++ {
		historyNoEffect(t, f)
	}
	trace := &historyCostTrace{}
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = trace
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if e = pool.Ping(f.ctx); e != nil {
		t.Fatal(e)
	}
	s := *f.service
	s.Pool = pool
	for _, size := range []int{1, 20, 100} {
		trace.mu.Lock()
		trace.queries = nil
		trace.mu.Unlock()
		start := time.Now()
		r, e := s.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, size))
		elapsed := time.Since(start)
		if e != nil || len(r.Items) != size || !r.HasMore {
			t.Fatal("bounded page missing", size, e)
		}
		trace.mu.Lock()
		queries := append([]string(nil), trace.queries...)
		trace.mu.Unlock()
		if len(queries) > 20 {
			t.Fatal("per-event SQL growth", size, len(queries))
		}
		for _, q := range queries {
			if strings.Contains(q, "workflow_evidence_") || strings.Contains(q, "graph_json") || strings.Contains(q, "payload_bytes") || strings.Contains(q, "result_bytes") {
				t.Fatal("metadata page loaded full evidence", q)
			}
		}
		t.Logf("V063_COST page=%d confirmedCandidates=101 statementsIncludingTx=%d elapsed=%s isolated warm fixture, not production throughput", size, len(queries), elapsed)
	}
}
