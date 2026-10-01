package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type queryTraceSQLKey struct{}
type queryRowTrace struct {
	mu            sync.Mutex
	max           int64
	afterRevision func()
}

func (q *queryRowTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTraceSQLKey{}, d.SQL)
}
func (q *queryRowTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	q.mu.Lock()
	if d.CommandTag.Select() && d.CommandTag.RowsAffected() > q.max {
		q.max = d.CommandTag.RowsAffected()
	}
	var hook func()
	sql, _ := ctx.Value(queryTraceSQLKey{}).(string)
	if strings.Contains(sql, "max(revision)") {
		hook = q.afterRevision
		q.afterRevision = nil
	}
	q.mu.Unlock()
	if hook != nil {
		hook()
	}
}
func (q *queryRowTrace) take() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	v := q.max
	q.max = 0
	return v
}
func queryWeb(t *testing.T) (*webFixture, *queryRowTrace) {
	t.Helper()
	f := setupWeb(t)
	opts, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	f.app.Queries = NewQueryContextStore(client, "query-http-"+strings.ReplaceAll(f.actor.UserID, "-", ""))
	trace := &queryRowTrace{}
	cfg := f.app.Pool.Config()
	cfg.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f.app.Pool = pool
	return f, trace
}
func queryBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func memberQueryResponse(t *testing.T, w *httptest.ResponseRecorder) MembersQueryPage {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("query must succeed: %d %s", w.Code, w.Body)
	}
	var e struct{ Data MembersQueryPage }
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Data.QueryVersion == "" {
		t.Fatal("missing query context")
	}
	return e.Data
}
func queryError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status || !strings.Contains(w.Body.String(), code) {
		t.Fatalf("want %d %s; got %d %s", status, code, w.Code, w.Body)
	}
}
func TestQ36HTTPMembersQueryLifecycleAndActualPagePath(t *testing.T) {
	f, trace := queryWeb(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("q36-http-%d-", time.Now().UnixNano())
	ids := []string{}
	for i := 0; i < 3; i++ {
		id := newTarget(t, f.fixture)
		ids = append(ids, id)
		if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=$2,created_at=$3 WHERE id=$1", id, prefix+fmt.Sprint(i), time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	body := map[string]any{"search": prefix, "pageSize": 1}
	request := func() *httptest.ResponseRecorder {
		return f.request("POST", "/api/v1/personnel/members/search", queryBody(t, body), true, true)
	}
	queryError(t, f.request("POST", "/api/v1/personnel/members/search", queryBody(t, body), true, false), 403, "COMMON_CSRF_REJECTED")
	before := queryRevisionsForTest(t, f.owner)
	first := memberQueryResponse(t, request())
	if first.Total != 3 || first.Items[0].ID != ids[0] {
		t.Fatalf("initial full result %+v", first)
	}
	if queryRevisionsForTest(t, f.owner) != before {
		t.Fatal("read-only POST mutated business revisions")
	}
	body["queryVersion"] = first.QueryVersion
	body["page"] = 2
	trace.take()
	page := memberQueryResponse(t, request())
	if page.Items[0].ID != ids[1] || page.QueryVersion != first.QueryVersion {
		t.Fatal("stable page context")
	}
	if n := trace.take(); n > 1 {
		t.Fatalf("unchanged revision read more than page rows from DB: %d", n)
	}
	other := newTarget(t, f.fixture)
	if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=account||'-unrelated' WHERE id=$1", other); err != nil {
		t.Fatal(err)
	}
	trace.take()
	memberQueryResponse(t, request())
	if n := trace.take(); n < 3 {
		t.Fatal("changed revision did not precisely recheck full result")
	}
	saved, err := f.app.Queries.Load(ctx, "11111111-1111-4111-8111-111111111111", first.QueryVersion)
	if err != nil {
		t.Fatal(err)
	}
	r := queryRevisionsForTest(t, f.owner)
	r.Activity = 0
	if saved.Revisions != r {
		t.Fatal("unrelated change did not CAS-advance verified revision")
	}
	if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", ids[2], prefix+"changed"); err != nil {
		t.Fatal(err)
	}
	queryError(t, request(), 409, "COMMON_QUERY_CHANGED")
	body["search"] = prefix + "0"
	body["page"] = 1
	queryError(t, request(), 409, "COMMON_QUERY_CHANGED")
	if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", ids[2], prefix+"2"); err != nil {
		t.Fatal(err)
	}
	body["search"] = prefix
	memberQueryResponse(t, request()) // ABA restored current result.
	body["search"] = prefix + "0"
	changed := memberQueryResponse(t, request())
	if changed.Total != 1 || changed.QueryVersion == first.QueryVersion {
		t.Fatal("new criteria needs independent immutable baseline")
	}
	legacy := memberQueryResponse(t, f.request("GET", "/api/v1/personnel/members?search="+prefix+"&page=2&pageSize=1", "", true, false))
	if legacy.Page.Page != 2 || legacy.Items[0].ID != ids[1] {
		t.Fatal("legacy valid page semantics")
	}
	queryError(t, f.request("GET", "/api/v1/personnel/members?filter=%7B%7D", "", true, false), 400, "COMMON_INVALID_ARGUMENT")
	queryError(t, f.request("POST", "/api/v1/personnel/members/search", `{"page":2}`, true, true), 400, "COMMON_INVALID_ARGUMENT")
	sid, csrf, err := f.store.Create(ctx, session.Record{UserID: f.actor.UserID, SessionRef: "22222222-2222-4222-8222-222222222222", AuthVersion: "1"})
	if err != nil {
		t.Fatal(err)
	}
	defer f.store.Revoke(ctx, sid)
	foreign := *f
	foreign.sid = sid
	foreign.csrf = csrf
	queryError(t, foreign.request("POST", "/api/v1/personnel/members/search", queryBody(t, body), true, true), 409, "COMMON_QUERY_CONTEXT_EXPIRED")
	key, _ := f.app.Queries.prefix("11111111-1111-4111-8111-111111111111")
	if err = f.app.Queries.client.PExpire(ctx, key+first.QueryVersion, -time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	body["search"] = prefix
	queryError(t, request(), 409, "COMMON_QUERY_CONTEXT_EXPIRED")
	d := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, true)
	if d.Code != 201 {
		t.Fatal("explicit draft save must survive query expiry", d.Code, d.Body)
	}
	_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID)
	if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor.UserID); err != nil {
		t.Fatal(err)
	}
	queryError(t, request(), 401, "AUTH_UNAUTHENTICATED")
}
func TestQ36HTTPQueryRawUnicodeAndFixedEventRange(t *testing.T) {
	f, _ := queryWeb(t)
	big := map[string]any{"filter": map[string]any{"operator": "and", "children": []any{map[string]any{"field": "account", "operator": "eq", "value": strings.Repeat("中文", 1800)}}}}
	body := queryBody(t, big)
	if len(body) <= 8192 || len(body) >= 65536 {
		t.Fatal("large Chinese fixture invalid")
	}
	memberQueryResponse(t, f.request("POST", "/api/v1/personnel/members/search", body, true, true))
	for _, bad := range []string{strings.Repeat(" ", 65536) + "{}", `{"filter":{"operator":"and","children":[{"field":"account","operator":"eq","value":"\ud800"}]}}`, `{"sortBy":"occurredAt","sortDirection":"asc"}`, `{"search":null}`, `{"page":0}`, `{"page":1,"page":1}`} {
		queryError(t, f.request("POST", "/api/v1/personnel/members/search", bad, true, true), 400, "COMMON_INVALID_ARGUMENT")
	}
	first := f.request("POST", "/api/v1/personnel/events/search", `{}`, true, true)
	if first.Code != 200 {
		t.Fatalf("events route %d %s", first.Code, first.Body)
	}
	var e struct{ Data EventsQueryPage }
	if err := json.Unmarshal(first.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	second := f.request("POST", "/api/v1/personnel/events/search", queryBody(t, map[string]any{"queryVersion": e.Data.QueryVersion, "page": 2}), true, true)
	if second.Code != 200 {
		t.Fatal(second.Code, second.Body)
	}
	var n struct{ Data EventsQueryPage }
	_ = json.Unmarshal(second.Body.Bytes(), &n)
	if e.Data.Range != n.Data.Range || e.Data.QueryVersion != n.Data.QueryVersion || e.Data.Range.To.Sub(e.Data.Range.From) != 7*24*time.Hour {
		t.Fatal("relative range shifted during continuation")
	}
	queryError(t, f.request("POST", "/api/v1/personnel/events/search", `{"sortBy":"occurredAt"}`, true, true), 400, "COMMON_INVALID_ARGUMENT")
}
