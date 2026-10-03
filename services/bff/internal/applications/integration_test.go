package applications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type webFixture struct {
	owner, runtime   *pgxpool.Pool
	service          *Service
	store            *session.Store
	actor, sid, csrf string
	ops              []string
}

func fixture(t *testing.T, root bool) *webFixture {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	owner, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	// Each package must work against a freshly migrated isolated database,
	// without depending on the later audit package to install runtime roles.
	// Use the exact reviewed policy, not a permissive fixture-only GRANT.
	roles, err := os.ReadFile("../../../../infra/runtime/roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, string(roles)); err != nil {
		t.Fatal("initialize reviewed isolated runtime roles", err)
	}
	var actor string
	if root {
		err = owner.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE is_bootstrap_admin").Scan(&actor)
	}
	if !root || err == pgx.ErrNoRows {
		err = owner.QueryRow(ctx, "INSERT INTO auth.users(account,is_bootstrap_admin) VALUES('b5-'||gen_random_uuid(),$1) RETURNING id::text", root).Scan(&actor)
	}
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, e := c.Exec(ctx, "SET ROLE auth_app"); return e }
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	store := session.NewStore(os.Getenv("WEAVEOS_TEST_REDIS_URL"), "b5-"+strings.ReplaceAll(actor, "-", ""))
	var version string
	if err := owner.QueryRow(ctx, "SELECT auth_version::text FROM auth.users WHERE id=$1", actor).Scan(&version); err != nil {
		t.Fatal(err)
	}
	var ref string
	if err := owner.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&ref); err != nil {
		t.Fatal(err)
	}
	sid, csrf, err := store.Create(ctx, session.Record{UserID: actor, SessionRef: ref, AuthVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = store.Revoke(ctx, sid); _ = store.Close() })
	f := &webFixture{owner: owner, runtime: runtime, store: store, actor: actor, sid: sid, csrf: csrf}
	f.service = &Service{Application: &Application{Pool: runtime}, Authenticator: session.Authenticator{Sessions: store, DB: runtime, Origin: "https://weaveos.test"}}
	return f
}
func (f *webFixture) uuid(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.owner.QueryRow(context.Background(), "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func (f *webFixture) operation(t *testing.T) string {
	id := f.uuid(t)
	f.ops = append(f.ops, id)
	return id
}
func (f *webFixture) request(method, path, body string, login, csrf bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://weaveos.test"+path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.19:4321"
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
	f.service.ServeHTTP(w, r)
	return w
}
func (f *webFixture) call(method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	return f.request(method, path, string(b), true, true)
}
func data(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]json.RawMessage {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want status %d got %d: %s", status, w.Code, w.Body.String())
	}
	var env struct {
		Code string
		Data map[string]json.RawMessage
		Meta map[string]string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != "OK" || env.Meta["requestId"] == "" {
		t.Fatal("standard trusted envelope required")
	}
	return env.Data
}
func value(t *testing.T, d map[string]json.RawMessage, k string) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(d[k], &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func revision(t *testing.T, d map[string]json.RawMessage) int64 {
	t.Helper()
	var n int64
	if err := json.Unmarshal(d["policyRevision"], &n); err != nil {
		t.Fatal(err)
	}
	return n
}
func (f *webFixture) create(t *testing.T) (string, string) {
	op := f.operation(t)
	w := f.call("POST", "/api/v1/applications", map[string]any{"name": "  应用  ", "operationId": op})
	d := data(t, w, 201)
	id := value(t, d, "id")
	if value(t, d, "name") != "应用" || value(t, d, "ownerUserId") != f.actor || revision(t, d) != 1 || w.Header().Get("Location") != "/api/v1/applications/"+id {
		t.Fatal("canonical app result/fixed owner/initial revision/Location")
	}
	return id, op
}

func TestB5FrozenSchemaAndCapability(t *testing.T) {
	f := fixture(t, false)
	ctx := context.Background()
	for _, name := range []string{"apps", "permission_groups", "group_members", "grants", "grant_fields", "menu_resources", "operations"} {
		var yes bool
		if err := f.owner.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "applications."+name).Scan(&yes); err != nil {
			t.Fatal(err)
		}
		if !yes {
			t.Errorf("B5a.2 required persistent relation applications.%s missing", name)
		}
	}
	var n int
	if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.permission_catalog WHERE code='applications.create' AND name='应用管理' AND category='system' AND app_id IS NULL AND enabled").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("B5a.1 exact create capability must be registered")
	}
}

func TestB5CreateAtomicAndActorScopedOperation(t *testing.T) {
	f := fixture(t, true)
	id, op := f.create(t)
	ctx := context.Background()
	for _, q := range []string{"SELECT count(*) FROM applications.apps WHERE id=$1", "SELECT count(*) FROM applications.menu_resources WHERE app_id=$1 AND resource_id=$1 AND resource_kind='application'", "SELECT count(*) FROM personnel.permission_catalog WHERE app_id=$1::text AND code='app.'||$1::text||'.access'", "SELECT count(*) FROM auth.authentication_events WHERE object_id=$1 AND event_type='application_changed' AND reason_code='APPLICATION_CREATED'"} {
		var n int
		if err := f.owner.QueryRow(ctx, q, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("app/menu/catalog/audit must commit exactly once")
		}
	}
	w := f.call("GET", "/api/v1/application-operations/"+op, nil)
	d := data(t, w, 200)
	if value(t, d, "operationId") != op {
		t.Fatal("operation lookup must bind original operation")
	}
	other := fixture(t, false)
	if w := other.call("GET", "/api/v1/application-operations/"+op, nil); w.Code != 404 {
		t.Fatalf("other actor operation lookup must not leak result: %d", w.Code)
	}
}

func TestB5OwnerMembersMenuAndRevoke(t *testing.T) {
	f := fixture(t, true)
	id, _ := f.create(t)
	member := fixture(t, false)
	base := "/api/v1/applications/" + id
	group := data(t, f.call("POST", base+"/permission-groups", map[string]any{"name": "读者", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201)
	gid := value(t, group, "id")
	if revision(t, group) != 2 {
		t.Fatal("group create increments exactly once")
	}
	gp := base + "/permission-groups/" + gid
	data(t, f.call("PUT", gp+"/members", map[string]any{"memberIds": []string{member.actor}, "operationId": f.operation(t), "expectedPolicyRevision": 2}), 200)
	if w := member.call("GET", base+"/access", nil); w.Code != 403 {
		t.Fatalf("membership alone must not grant entry: %d", w.Code)
	}
	grant := map[string]any{"resourceKind": "application", "resourceId": id, "action": "menu.enter", "rowScope": "all", "fields": []string{}}
	d := data(t, f.call("PUT", gp+"/grants", map[string]any{"grants": []any{grant}, "operationId": f.operation(t), "expectedPolicyRevision": 3}), 200)
	if revision(t, d) != 4 {
		t.Fatal("grant replace revision")
	}
	access := data(t, member.call("GET", base+"/access", nil), 200)
	var can bool
	_ = json.Unmarshal(access["canEnter"], &can)
	if !can {
		t.Fatal("member+menu enters without global identity/template entry grant")
	}
	list := data(t, member.call("GET", "/api/v1/applications", nil), 200)
	if !strings.Contains(string(list["items"]), id) {
		t.Fatal("authorized app must be discoverable")
	}
	if w := member.call("GET", gp+"/members", nil); w.Code != 403 {
		t.Fatal("menu entry must not delegate policy configuration")
	}
	data(t, f.call("PUT", gp+"/members", map[string]any{"memberIds": []string{}, "operationId": f.operation(t), "expectedPolicyRevision": 4}), 200)
	if w := member.call("GET", base+"/access", nil); w.Code != 403 {
		t.Fatal("next request must observe persisted revoke without relogin")
	}
}

func TestB5EveryWriteReplayAndConflict(t *testing.T) {
	f := fixture(t, true)
	id, op := f.create(t)
	body := map[string]any{"name": "应用", "operationId": op}
	data(t, f.call("POST", "/api/v1/applications", body), 201)
	if w := f.call("POST", "/api/v1/applications", map[string]any{"name": "different", "operationId": op}); w.Code != 409 || !strings.Contains(w.Body.String(), "APPLICATION_OPERATION_CONFLICT") {
		t.Fatal("same actor/key different payload is 409")
	}
	op = f.operation(t)
	route := "/api/v1/applications/" + id + "/permission-groups"
	body = map[string]any{"name": "组", "operationId": op, "expectedPolicyRevision": 1}
	first := data(t, f.call("POST", route, body), 201)
	again := data(t, f.call("POST", route, body), 201)
	if string(first["id"]) != string(again["id"]) || revision(t, again) != 2 {
		t.Fatal("same group operation must stable replay before stale CAS")
	}
	gid := value(t, first, "id")
	gp := route + "/" + gid
	for _, item := range []struct {
		path string
		body map[string]any
	}{{gp, map[string]any{"name": "changed", "enabled": true}}, {gp + "/members", map[string]any{"memberIds": []string{}}}, {gp + "/grants", map[string]any{"grants": []any{}}}} {
		op = f.operation(t)
		item.body["operationId"] = op
		item.body["expectedPolicyRevision"] = revision(t, again)
		again = data(t, f.call("PUT", item.path, item.body), 200)
		replay := data(t, f.call("PUT", item.path, item.body), 200)
		if revision(t, replay) != revision(t, again) {
			t.Fatal("replay must not add second policy revision")
		}
		if w := f.call("PUT", gp+"/members", item.body); item.path != gp+"/members" && w.Code != 400 && w.Code != 409 {
			t.Fatal("different methods/resources cannot silently replay")
		}
	}
}

func TestB5SessionCSRFAndClosedPayload(t *testing.T) {
	f := fixture(t, true)
	body := `{"name":"x","operationId":"` + f.operation(t) + `"}`
	if w := f.request("POST", "/api/v1/applications", body, false, false); w.Code != 401 {
		t.Fatal("anonymous must challenge")
	}
	if w := f.request("POST", "/api/v1/applications", body, true, false); w.Code != 403 {
		t.Fatal("bound CSRF required")
	}
	for _, input := range []string{`{"name":"x","name":"y","operationId":"` + f.operation(t) + `"}`, `{"name":"x","ownerUserId":"` + f.actor + `","operationId":"` + f.operation(t) + `"}`, `{"name":"x"}`, `{"name":null,"operationId":"` + f.operation(t) + `"}`} {
		if w := f.request("POST", "/api/v1/applications", input, true, true); w.Code != 400 {
			t.Fatalf("strict write DTO must reject duplicate/unknown/missing/null: %d", w.Code)
		}
	}
}

func TestB5CreateCapabilityDoesNotGrantOtherApp(t *testing.T) {
	f := fixture(t, false)
	ctx := context.Background()
	var registered bool
	if err := f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='applications.create')").Scan(&registered); err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("frozen create capability migration not implemented")
	}
	var identity string
	if err := f.owner.QueryRow(ctx, "INSERT INTO personnel.identities(name) VALUES('b5-create-only') RETURNING id::text").Scan(&identity); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{{"INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", []any{f.actor, identity}}, {"INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'applications.create')", []any{identity}}} {
		if _, err := f.owner.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	id, _ := f.create(t)
	data(t, f.call("GET", "/api/v1/applications/"+id+"/access", nil), 200)
	other := fixture(t, true)
	foreign, _ := other.create(t)
	if w := f.call("GET", "/api/v1/applications/"+foreign+"/permission-groups", nil); w.Code != 403 {
		t.Fatal("create-only is not management of another app")
	}
	if w := f.call("POST", "/api/v1/applications/"+foreign+"/permission-groups", map[string]any{"name": "bad", "operationId": f.operation(t), "expectedPolicyRevision": 1}); w.Code != 403 {
		t.Fatal("create-only cannot configure foreign app")
	}
}
