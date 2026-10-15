package appstructure

// V075 P15 / V020 manager authority / V044 published triggers. Real PG/Redis/Session.
import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func rootManagerSearchHandler(t *testing.T, s rootWorkflowManagementFixture) http.Handler {
	t.Helper()
	opts, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opts)
	gen := "root-manager-" + s.f.actor
	if e = client.Ping(context.Background()).Err(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		defer client.Close()
		var cursor uint64
		for {
			keys, next, e := client.Scan(context.Background(), cursor, "ems:workflow-manager:query:"+gen+":*", 100).Result()
			if e != nil {
				t.Error(e)
				return
			}
			if len(keys) > 0 {
				if e = client.Del(context.Background(), keys...).Err(); e != nil {
					t.Error(e)
				}
			}
			cursor = next
			if cursor == 0 {
				return
			}
		}
	})
	return &appworkflows.Service{Application: &appworkflows.Application{Pool: s.f.runtime, Limits: s.f.service.Application.Limits, ManagerQueries: appworkflows.NewManagerQueryStore(client, gen)}, Authenticator: s.f.service.Authenticator}
}
func rootManagerSearchPath(s rootWorkflowManagementFixture) string {
	return "/api/v1/applications/" + s.f.app + "/forms/" + s.view + "/workflows/search"
}
func rootManagerSearchCall(t *testing.T, s rootWorkflowManagementFixture, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	return rootWorkflowHTTPRequest(t, s, h, "POST", rootManagerSearchPath(s), string(b), true, true, "", nil)
}
func rootManagerSearchBody(page, size int, token string) map[string]any {
	b := map[string]any{"page": page, "pageSize": size}
	if token != "" {
		b["queryVersion"] = token
	}
	return b
}
func rootManagerSearchCreate(t *testing.T, s rootWorkflowManagementFixture, name string) string {
	t.Helper()
	flow := uuid(t, s.f.owner)
	body := rootWorkflowHTTPBody(t, s, s.f.actor)
	body["name"] = name
	body["triggers"] = []any{map[string]any{"event": "manual", "condition": nil}}
	data(t, rootWorkflowHTTPCall(t, s, flow, "PUT", "definition", body), 201)
	return flow
}
func rootManagerSearchItems(t *testing.T, out map[string]any) []any {
	t.Helper()
	items, ok := out["items"].([]any)
	if !ok {
		t.Fatalf("items must be array: %+v", out)
	}
	return items
}
func TestRootManagerSearchClosedPublishedSummaryAndNoSideEffects(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootManagerSearchHandler(t, s)
	flow := rootManagerSearchCreate(t, s, "已发布流程")
	head := rootWorkflowHTTPConfirm(t, s, flow)
	body := rootWorkflowHTTPBody(t, s, s.f.actor)
	body["expectedRevision"] = head.Revision
	body["name"] = "待发布修改"
	body["triggers"] = []any{map[string]any{"event": "record.updated", "condition": nil}}
	saved := data(t, rootWorkflowHTTPCall(t, s, flow, "PUT", "definition", body), 200)
	var before int
	if e := s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE app_id=$1", s.f.app).Scan(&before); e != nil {
		t.Fatal(e)
	}
	w := rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, ""))
	out := data(t, w, 200)
	if len(out) != 5 || out["total"] != float64(1) || out["page"] != float64(1) || out["pageSize"] != float64(20) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("page %+v", out)
	}
	items := rootManagerSearchItems(t, out)
	if len(items) != 1 {
		t.Fatalf("items %+v", items)
	}
	item := items[0].(map[string]any)
	if len(item) != 9 || item["id"] != flow || item["name"] != "待发布修改" || item["revision"] != saved["revision"] || item["currentVersion"] != float64(1) || item["candidateVersion"] != float64(2) || item["state"] != "disabled" || item["inFlightCount"] != float64(0) || !reflect.DeepEqual(item["triggerEvents"], []any{"manual"}) {
		t.Fatalf("closed current summary %+v", item)
	}
	if strings.Contains(w.Body.String(), "graph") || strings.Contains(w.Body.String(), "assigneeIds") || strings.Contains(w.Body.String(), "trusted-isolated-receipt") {
		t.Fatal("private definition leaked")
	}
	token, ok := out["queryVersion"].(string)
	if !ok || len(token) != 43 || strings.Contains(token, flow) {
		t.Fatal("opaque token required")
	}
	var after int
	if e := s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE app_id=$1", s.f.app).Scan(&after); e != nil || after != before {
		t.Fatalf("read created operation %d %d %v", before, after, e)
	}
}
func TestRootManagerSearchUnpublishedAndStablePages(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootManagerSearchHandler(t, s)
	first := rootManagerSearchCreate(t, s, "first")
	second := rootManagerSearchCreate(t, s, "second")
	for flow, stamp := range map[string]string{first: "2026-01-01T00:00:00Z", second: "2026-01-02T00:00:00Z"} {
		if _, e := s.f.owner.Exec(context.Background(), "UPDATE applications.workflow_definitions SET updated_at=$2 WHERE id=$1", flow, stamp); e != nil {
			t.Fatal(e)
		}
	}
	one := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 1, "")), 200)
	token := one["queryVersion"].(string)
	item := rootManagerSearchItems(t, one)[0].(map[string]any)
	if item["id"] != second || item["currentVersion"] != float64(0) || !reflect.DeepEqual(item["triggerEvents"], []any{}) || one["total"] != float64(2) {
		t.Fatalf("unpublished %+v", one)
	}
	two := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(2, 1, token)), 200)
	if rootManagerSearchItems(t, two)[0].(map[string]any)["id"] != first || two["queryVersion"] != token {
		t.Fatalf("stable page %+v", two)
	}
	empty := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(99, 1, token)), 200)
	if len(rootManagerSearchItems(t, empty)) != 0 || empty["total"] != float64(2) {
		t.Fatalf("deep page %+v", empty)
	}
	rootManagerSearchCreate(t, s, "third")
	rootWorkflowHTTPError(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(2, 1, token)), 409, "COMMON_QUERY_CHANGED")
	fresh := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 1, "")), 200)
	if fresh["total"] != float64(3) {
		t.Fatalf("fresh %+v", fresh)
	}
}
func TestRootManagerSearchStartingCountsAndClosingRemainInFlight(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootManagerSearchHandler(t, s)
	flow := rootManagerSearchCreate(t, s, "closing")
	head := rootWorkflowHTTPConfirm(t, s, flow)
	enabled := data(t, rootWorkflowHTTPCall(t, s, flow, "POST", "enable", map[string]any{"operationId": uuid(t, s.f.owner), "expectedRevision": head.Revision}), 200)
	old := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, "")), 200)
	record := uuid(t, s.f.owner)
	physical := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.table, "-", "")}.Sanitize()
	if _, e := s.f.owner.Exec(context.Background(), "INSERT INTO "+physical+"(id,created_by) VALUES($1,$2)", record, s.f.actor); e != nil {
		t.Fatal(e)
	}
	s.tx(t, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ReserveInTx(context.Background(), tx, workflowcatalog.ReserveInput{AppID: s.f.app, FlowID: flow, InstanceID: uuid(t, s.f.owner), RecordID: record, ActorID: s.f.actor, ExpectedRevision: int64(enabled["revision"].(float64)), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1})
		return e
	})
	rootWorkflowHTTPError(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, old["queryVersion"].(string))), 409, "COMMON_QUERY_CHANGED")
	data(t, rootWorkflowHTTPCall(t, s, flow, "POST", "close", map[string]any{"operationId": uuid(t, s.f.owner), "expectedRevision": enabled["revision"]}), 200)
	out := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, "")), 200)
	item := rootManagerSearchItems(t, out)[0].(map[string]any)
	if item["state"] != "closing" || item["inFlightCount"] != float64(1) {
		t.Fatalf("accepted start omitted %+v", item)
	}
}
func TestRootManagerSearchSessionCSRFActorAndCurrentManager(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootManagerSearchHandler(t, s)
	rootManagerSearchCreate(t, s, "secret")
	path := rootManagerSearchPath(s)
	raw := `{"page":1,"pageSize":20}`
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, false, false, "", nil), 401, "AUTH_UNAUTHENTICATED")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, true, false, "", nil), 403, "COMMON_CSRF_REJECTED")
	other, _ := rootWorkflowHTTPCandidate(t, s, "all")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, true, true, other, nil), 403, "APPLICATION_FORBIDDEN")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, true, true, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", other) }), 409, "AUTH_SESSION_CHANGED")
	if _, e := s.f.owner.Exec(context.Background(), "UPDATE auth.users SET status='disabled' WHERE id=$1", s.f.actor); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, "invalid-token")), 401, "AUTH_UNAUTHENTICATED")
}
func TestRootManagerSearchRejectsMalformedReadRequests(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootManagerSearchHandler(t, s)
	path := rootManagerSearchPath(s)
	for _, raw := range []string{`{}`, `{"page":null,"pageSize":20}`, `{"page":1,"pageSize":101}`, `{"page":0,"pageSize":20}`, `{"page":1,"page":2,"pageSize":20}`, `{"page":1,"pageSize":20,"actorId":"x"}`, `{"page":1,"pageSize":20,"filter":null}`, `{"page":1.5,"pageSize":20}`, `{"page":"1","pageSize":20}`, `{"page":9007199254740991,"pageSize":100}`} {
		t.Run(raw, func(t *testing.T) {
			rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, true, true, "", nil), 400, "COMMON_INVALID_ARGUMENT")
		})
	}
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path+"?page=1", `{"page":1,"pageSize":20}`, true, true, "", nil), 400, "COMMON_INVALID_ARGUMENT")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "GET", path, "", true, false, "", nil), 404, "API_NOT_FOUND")
}
func TestRootManagerSearchTokenBindsSessionAndIgnoresOtherApplicationChanges(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootManagerSearchHandler(t, s)
	rootManagerSearchCreate(t, s, "original")
	out := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, "")), 200)
	token := out["queryVersion"].(string)
	other := rootWorkflowManagementSetup(t)
	rootManagerSearchCreate(t, other, "other application")
	same := data(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, token)), 200)
	if same["queryVersion"] != token || same["total"] != float64(1) {
		t.Fatalf("unrelated change invalidated current query %+v", same)
	}
	raw, _ := json.Marshal(rootManagerSearchBody(1, 20, token))
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", rootManagerSearchPath(s), string(raw), true, true, s.f.actor, nil), 409, "COMMON_QUERY_CONTEXT_EXPIRED")
	missing := strings.Replace(rootManagerSearchPath(s), "/forms/"+s.view+"/", "/forms/"+s.table+"/", 1)
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", missing, string(raw), true, true, "", nil), 404, "APPLICATION_NOT_FOUND")
	if _, e := s.f.owner.Exec(context.Background(), "UPDATE personnel.permission_catalog SET enabled=false WHERE app_id=$1 AND category='application'", s.f.app); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootManagerSearchCall(t, s, h, rootManagerSearchBody(1, 20, token)), 403, "APPLICATION_FORBIDDEN")
}
