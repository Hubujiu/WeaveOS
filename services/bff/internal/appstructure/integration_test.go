package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	owner, runtime        *pgxpool.Pool
	service               *Service
	actor, app, sid, csrf string
	session               *session.Store
}

func setup(t *testing.T) *fixture {
	t.Helper()
	c := context.Background()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("real isolated PostgreSQL required")
	}
	p, e := pgxpool.New(c, dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	var version int
	if e = p.QueryRow(c, "SELECT current_setting('server_version_num')::int").Scan(&version); e != nil || version < 180000 {
		t.Fatal("connected PG18 required", e)
	}
	roles, e := os.ReadFile("../../../../infra/runtime/roles.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(c, string(roles)); e != nil {
		t.Fatal(e)
	}
	var actor string
	if e = p.QueryRow(c, "INSERT INTO auth.users(account) VALUES('v013-'||gen_random_uuid()) RETURNING id::text").Scan(&actor); e != nil {
		t.Fatal(e)
	}
	var app string
	if e = p.QueryRow(c, "INSERT INTO applications.apps(name,owner_user_id) VALUES('test',$1) RETURNING id::text", actor).Scan(&app); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(c, "INSERT INTO applications.menu_resources VALUES($1,'application',$1);", app); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(c, "SELECT applications.register_catalog_entry($1)", app); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.AfterConnect = func(c context.Context, x *pgx.Conn) error { _, e := x.Exec(c, "SET ROLE auth_app"); return e }
	r, e := pgxpool.NewWithConfig(c, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(r.Close)
	store := session.NewStore(os.Getenv("WEAVEOS_TEST_REDIS_URL"), "v013-"+strings.ReplaceAll(actor, "-", ""))
	sid, csrf, e := store.Create(c, session.Record{UserID: actor, SessionRef: uuid(t, p), AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Revoke(c, sid); store.Close() })
	f := &fixture{p, r, &Service{Application: &Application{Pool: r, ConfirmationKey: []byte("isolated-ephemeral-test-key-32bytes"), ConfirmationKeyID: "test", Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}, Dependencies: LocalRegistry{}}, Authenticator: session.Authenticator{Sessions: store, DB: r, Origin: "https://weaveos.test"}}, actor, app, sid, csrf, store}
	return f
}
func uuid(t *testing.T, p *pgxpool.Pool) string {
	var s string
	if e := p.QueryRow(context.Background(), "SELECT gen_random_uuid()::text").Scan(&s); e != nil {
		t.Fatal(e)
	}
	return s
}
func (f *fixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "https://weaveos.test/api/v1/applications/"+f.app+path, strings.NewReader(string(b)))
	r.Header.Set("Origin", "https://weaveos.test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", f.csrf)
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
	r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: f.csrf})
	w := httptest.NewRecorder()
	f.service.ServeHTTP(w, r)
	return w
}
func data(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("product contract status want %d got %d: %s", status, w.Code, w.Body.String())
	}
	var env struct {
		Code string
		Data map[string]any
	}
	if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil || env.Code != "OK" {
		t.Fatalf("envelope %s %v", w.Body.String(), e)
	}
	return env.Data
}
func TestRealPGStructureMigrationAndRestrictedDDL(t *testing.T) {
	f := setup(t)
	for _, n := range []string{"directories", "logical_tables", "fields", "form_views", "table_field_dependencies"} {
		var yes bool
		if e := f.owner.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", "applications."+n).Scan(&yes); e != nil {
			t.Fatal(e)
		}
		if !yes {
			t.Errorf("required real metadata relation %s absent", n)
		}
	}
	if _, e := f.runtime.Exec(context.Background(), "CREATE TABLE appdata.unapproved(id int)"); e == nil {
		t.Fatal("runtime arbitrary DDL must fail")
	}
}
func TestAtomicFormCreateAndGridSave(t *testing.T) {
	f := setup(t)
	create := map[string]any{"operationId": uuid(t, f.owner), "name": "表单", "source": map[string]any{"kind": "new_table"}, "directoryId": nil, "position": 0, "expectedStructureVersion": 0}
	d := data(t, f.call(t, "POST", "/forms", create), 201)
	form := d["form"].(map[string]any)
	table := d["table"].(map[string]any)
	if form["viewVersion"] != float64(0) || table["schemaVersion"] != float64(0) {
		t.Fatal("pending version zero")
	}
	fid := uuid(t, f.owner)
	field := map[string]any{"id": fid, "name": "金额", "kind": "money", "required": true, "default": "1.235", "config": map[string]any{}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}}
	save := map[string]any{"operationId": uuid(t, f.owner), "expectedSchemaVersion": 0, "expectedViewVersion": 0, "fields": []any{field}, "layout": []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": fid, "span": 6}}, "optionMappings": []any{}, "confirmationToken": nil}
	out := data(t, f.call(t, "PUT", "/forms/"+form["id"].(string)+"/definition", save), 200)
	def := out["definition"].(map[string]any)
	if def["table"].(map[string]any)["schemaVersion"] != float64(1) {
		t.Fatal("saved schema")
	}
	var typ string
	if e := f.owner.QueryRow(context.Background(), "SELECT format_type(a.atttypid,a.atttypmod) FROM pg_attribute a WHERE a.attrelid=to_regclass($1) AND a.attname=$2", "appdata.t_"+strings.ReplaceAll(table["id"].(string), "-", ""), "f_"+strings.ReplaceAll(fid, "-", "")).Scan(&typ); e != nil || typ != "numeric(38,2)" {
		t.Fatal("real precise physical column", typ, e)
	}
	data(t, f.call(t, "PUT", "/forms/"+form["id"].(string)+"/definition", save), 200)
	data(t, f.call(t, "GET", "/forms/"+form["id"].(string)+"/definition", nil), 200)
}
func TestOrdinaryRootMenuDoesNotGrantDefinition(t *testing.T) {
	f := setup(t)
	var other string
	f.owner.QueryRow(context.Background(), "INSERT INTO auth.users(account) VALUES('other-'||gen_random_uuid()) RETURNING id::text").Scan(&other)
	_, e := f.owner.Exec(context.Background(), "UPDATE applications.apps SET owner_user_id=$2 WHERE id=$1", f.app, other)
	if e == nil {
		t.Fatal("immutable owner regression")
	}
	foreign, e := (&applications.Application{Pool: f.runtime}).List(context.Background(), session.Principal{})
	if e == nil || foreign != nil {
		t.Fatal("untrusted session must fail")
	}
	w := f.call(t, "GET", "/forms/"+uuid(t, f.owner)+"/definition", nil)
	if w.Code != 404 {
		t.Fatalf("fake resource must fail closed: %d %s", w.Code, w.Body.String())
	}
}
