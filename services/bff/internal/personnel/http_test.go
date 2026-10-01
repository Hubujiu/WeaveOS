package personnel

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type webFixture struct {
	*fixture
	handler   http.Handler
	store     *session.Store
	sid, csrf string
}

func setupWeb(t *testing.T) *webFixture {
	t.Helper()
	f := setup(t)
	store := session.NewStore(os.Getenv("WEAVEOS_TEST_REDIS_URL"), "personnel-http-"+strings.ReplaceAll(f.actor.UserID, "-", ""))
	ctx := context.Background()
	sid, csrf, err := store.Create(ctx, session.Record{UserID: f.actor.UserID, SessionRef: "11111111-1111-4111-8111-111111111111", AuthVersion: "1"})
	if err != nil {
		t.Fatal("isolated real Redis session required")
	}
	t.Cleanup(func() { _, _ = store.Revoke(ctx, sid); _ = store.Close() })
	queryOptions, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	queryRedis := redis.NewClient(queryOptions)
	t.Cleanup(func() { _ = queryRedis.Close() })
	f.app.Queries = NewQueryContextStore(queryRedis, "personnel-http-"+strings.ReplaceAll(f.actor.UserID, "-", ""))
	s := &Service{Application: f.app, Authenticator: session.Authenticator{Sessions: store, DB: f.app.Pool, Origin: "https://weaveos.test"}}
	return &webFixture{f, httpserver.NewHandler(func(context.Context) error { return nil }, s), store, sid, csrf}
}
func (f *webFixture) request(method, path, body string, login, csrf bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://weaveos.test"+path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.77:4321"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://weaveos.test")
	if login {
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: f.csrf})
	}
	if csrf {
		r.Header.Set("X-CSRF-Token", f.csrf)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func envelopeData(t *testing.T, w *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal("expected standard JSON envelope")
	}
	for _, field := range []string{"code", "message", "data", "meta"} {
		if body[field] == nil {
			t.Fatalf("missing envelope %s", field)
		}
	}
	if len(body) != 4 {
		t.Fatal("exact envelope required")
	}
	var data map[string]json.RawMessage
	_ = json.Unmarshal(body["data"], &data)
	return data
}
func TestHTTPRealSessionOwnAccessAndRevocationQ25(t *testing.T) {
	f := setupWeb(t)
	unauthorized := f.request("GET", "/api/v1/me/access", "", false, false)
	if unauthorized.Code != 401 || unauthorized.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("anonymous own access 401 challenge, got %d", unauthorized.Code)
	}
	allowed := f.request("GET", "/api/v1/me/access", "", true, false)
	if allowed.Code != 200 {
		t.Fatalf("own access real session, got %d", allowed.Code)
	}
	data := envelopeData(t, allowed)
	if string(data["personnelManage"]) != "true" {
		t.Fatal("real template-based manager")
	}
	if _, err := f.owner.Exec(context.Background(), "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template); err != nil {
		t.Fatal(err)
	}
	denied := f.request("GET", "/api/v1/personnel/members", "", true, false)
	if denied.Code != 403 {
		t.Fatalf("live revoke must return 403 without relogin, got %d", denied.Code)
	}
	if _, err := f.store.Revoke(context.Background(), f.sid); err != nil {
		t.Fatal(err)
	}
	if w := f.request("GET", "/api/v1/me/access", "", true, false); w.Code != 401 {
		t.Fatal("real revoked Session must deny")
	}
}
func TestHTTPDefinitionStatusesCSRFAndVersionQ25(t *testing.T) {
	f := setupWeb(t)
	input := `{"name":"HTTP身份","description":"","templateIds":[],"permissionCodes":[]}`
	if w := f.request("POST", "/api/v1/personnel/identities", input, true, false); w.Code != 403 {
		t.Fatalf("write needs bound CSRF, got %d", w.Code)
	}
	created := f.request("POST", "/api/v1/personnel/identities", input, true, true)
	if created.Code != 201 || created.Header().Get("Location") == "" {
		t.Fatalf("create 201 Location, got %d", created.Code)
	}
	data := envelopeData(t, created)
	var id string
	_ = json.Unmarshal(data["id"], &id)
	cleanupDefinition(t, f.fixture, Identity, id)
	update := `{"name":"HTTP修改","description":"","templateIds":[],"permissionCodes":[],"version":1}`
	if w := f.request("PUT", "/api/v1/personnel/identities/"+id, update, true, true); w.Code != 200 {
		t.Fatalf("versioned update: %d", w.Code)
	}
	if w := f.request("PUT", "/api/v1/personnel/identities/"+id, update, true, true); w.Code != 409 || !strings.Contains(w.Body.String(), "PERSONNEL_CONFLICT") {
		t.Fatal("stale update returns specific 409")
	}
	deleted := f.request("DELETE", "/api/v1/personnel/identities/"+id+"?version=2", "", true, true)
	if deleted.Code != 204 || deleted.Body.Len() != 0 {
		t.Fatalf("delete 204 no body, got %d", deleted.Code)
	}
}
func TestHTTPStrictBodyAndQueryQ25(t *testing.T) {
	f := setupWeb(t)
	for _, body := range []string{`{"name":"x","description":"","templateIds":[],"permissionCodes":[],"bootstrapAdmin":true}`, `{"name":"x","description":"","templateIds":null,"permissionCodes":[]}`, `{"name":"x","name":"y","description":"","templateIds":[],"permissionCodes":[]}`, `{"name":"x","description":"","permissionCodes":[]}`} {
		if w := f.request("POST", "/api/v1/personnel/identities", body, true, true); w.Code != 400 {
			t.Fatalf("unknown, null, duplicate and absent fields reject, got %d", w.Code)
		}
	}
	if w := f.request("POST", "/api/v1/personnel/identities", `{"name":"x","description":"","templateIds":[],"permissionCodes":[],"version":0}`, true, true); w.Code != 400 {
		t.Fatalf("create must reject edit-only version even zero, got %d", w.Code)
	}
	root := rootDepartment(t, f.fixture)
	if w := f.request("POST", "/api/v1/personnel/members/"+f.actor.UserID+"/groups", `{"operation":"add","departmentId":"`+root+`","sourceDepartmentId":null,"version":0}`, true, true); w.Code != 400 {
		t.Fatalf("optional UUID may be absent but not null, got %d", w.Code)
	}
	for _, query := range []string{"?page=0", "?pageSize=101", "?page=1&page=2", "?bootstrapAdmin=true"} {
		if w := f.request("GET", "/api/v1/personnel/members"+query, "", true, false); w.Code != 400 {
			t.Fatalf("strict query %s: %d", query, w.Code)
		}
	}
}
func TestHTTPMissingAndDependencyFailureQ25(t *testing.T) {
	f := setupWeb(t)
	if w := f.request("GET", "/api/v1/personnel/members/00000000-0000-4000-8000-000000000099", "", true, false); w.Code != 404 {
		t.Fatalf("missing member 404, got %d", w.Code)
	}
	f.app.Pool.Close()
	if w := f.request("GET", "/api/v1/me/access", "", true, false); w.Code != 503 {
		t.Fatalf("cannot verify current identity must fail 503, got %d", w.Code)
	}
}

func TestHTTPAllApprovedReadFamiliesQ25(t *testing.T) {
	for _, path := range []string{"personnel/departments", "personnel/members", "personnel/identities", "personnel/templates", "personnel/permissions", "personnel/events"} {
		t.Run(path, func(t *testing.T) {
			f := setupWeb(t)
			w := f.request("GET", "/api/v1/"+path, "", true, false)
			if w.Code != 200 {
				t.Fatalf("approved collection read: %d", w.Code)
			}
			data := envelopeData(t, w)
			if data["items"] == nil || string(data["items"]) == "null" {
				t.Fatal("collection is an explicit items array")
			}
		})
	}
	for _, kind := range []string{"members", "identities", "templates"} {
		t.Run(kind+" detail", func(t *testing.T) {
			f := setupWeb(t)
			id := f.actor.UserID
			if kind == "identities" {
				id = f.i1
			}
			if kind == "templates" {
				id = f.template
			}
			w := f.request("GET", "/api/v1/personnel/"+kind+"/"+id, "", true, false)
			if w.Code != 200 {
				t.Fatalf("approved detail read: %d", w.Code)
			}
			_ = envelopeData(t, w)
		})
	}
}

func TestHTTPMemberAndDepartmentWritesQ25(t *testing.T) {
	f := setupWeb(t)
	target := newTarget(t, f.fixture)
	root := rootDepartment(t, f.fixture)
	identities := `{"identityIds":["` + f.i1 + `","` + f.i2 + `"],"version":0}`
	if w := f.request("PUT", "/api/v1/personnel/members/"+target+"/identities", identities, true, true); w.Code != 200 {
		t.Fatalf("member identity write: %d", w.Code)
	}
	groups := `{"operation":"add","departmentId":"` + root + `","version":1}`
	if w := f.request("POST", "/api/v1/personnel/members/"+target+"/groups", groups, true, true); w.Code != 200 {
		t.Fatalf("explicit group write: %d", w.Code)
	}
	created := f.request("POST", "/api/v1/personnel/departments", `{"name":"HTTP部门","parentId":"`+root+`"}`, true, true)
	if created.Code != 201 {
		t.Fatalf("department creation: %d", created.Code)
	}
	data := envelopeData(t, created)
	var id string
	_ = json.Unmarshal(data["id"], &id)
	cleanDepartment(t, f.fixture, id)
	if w := f.request("PUT", "/api/v1/personnel/departments/"+id, `{"name":"HTTP重命名","version":1}`, true, true); w.Code != 200 {
		t.Fatalf("department rename: %d", w.Code)
	}
	if w := f.request("DELETE", "/api/v1/personnel/departments/"+id+"?version=2", "", true, true); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("department delete empty 204")
	}
}

func TestHTTPOptionalUUIDMustNotBeNullQ25(t *testing.T) {
	f := setupWeb(t)
	root := rootDepartment(t, f.fixture)
	w := f.request("POST", "/api/v1/personnel/members/"+f.actor.UserID+"/groups", `{"operation":"add","departmentId":"`+root+`","sourceDepartmentId":null,"version":0}`, true, true)
	if w.Code != 400 {
		t.Fatalf("optional UUID may be absent but not null, got %d", w.Code)
	}
}
