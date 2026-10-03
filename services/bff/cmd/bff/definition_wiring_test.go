package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDefinitionConfigRequiresDedicatedExplicitKeyAndLimits(t *testing.T) {
	values := map[string]string{"WEAVEOS_DEFINITION_HMAC_KEY": base64.StdEncoding.EncodeToString([]byte("isolated-definition-test-32bytes!")), "WEAVEOS_DEFINITION_KEY_ID": "test", "WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS": "1000", "WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS": "5000"}
	cfg, e := readConfig(func(k string) string { return values[k] })
	if e != nil {
		t.Fatal(e)
	}
	key := reflect.ValueOf(cfg).FieldByName("DefinitionKey")
	if !key.IsValid() || key.Len() < 32 {
		t.Fatal("dedicated injected definition key was not loaded")
	}
	values["WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS"] = "0"
	if _, e = readConfig(func(k string) string { return values[k] }); e == nil {
		t.Fatal("explicit zero lock budget accepted")
	}
	delete(values, "WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS")
	if _, e = readConfig(func(k string) string { return values[k] }); e == nil {
		t.Fatal("partial definition configuration accepted")
	}
}
func TestBFFCompositionExposesDefinitionWithSameSession(t *testing.T) {
	c := context.Background()
	values := map[string]string{"WEAVEOS_DATABASE_URL": os.Getenv("WEAVEOS_TEST_DATABASE_URL"), "WEAVEOS_REDIS_URL": os.Getenv("WEAVEOS_TEST_REDIS_URL"), "WEAVEOS_PUBLIC_ORIGIN": "https://weaveos.test", "WEAVEOS_SESSION_GENERATION": "v013-composition", "WEAVEOS_AUDIT_KEY_ID": "test", "WEAVEOS_AUDIT_HMAC_KEY": base64.StdEncoding.EncodeToString([]byte("synthetic-audit-key-32-bytes-only!")), "WEAVEOS_DEFINITION_HMAC_KEY": base64.StdEncoding.EncodeToString([]byte("isolated-definition-test-32bytes!")), "WEAVEOS_DEFINITION_KEY_ID": "test", "WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS": "1000", "WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS": "5000"}
	cfg, e := readConfig(func(k string) string { return values[k] })
	if e != nil {
		t.Fatal(e)
	}
	p, e := pgxpool.New(c, cfg.DatabaseURL)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	var actor, app, operation string
	e = p.QueryRow(c, "INSERT INTO auth.users(account) VALUES('definition-composition-'||gen_random_uuid()) RETURNING id::text,gen_random_uuid()::text").Scan(&actor, &operation)
	if e != nil {
		t.Fatal(e)
	}
	e = p.QueryRow(c, "INSERT INTO applications.apps(name,owner_user_id) VALUES('composition',$1) RETURNING id::text", actor).Scan(&app)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(c, "INSERT INTO applications.menu_resources VALUES($1,'application',$1)", app); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(c, "SELECT applications.register_catalog_entry($1)", app); e != nil {
		t.Fatal(e)
	}
	store := session.NewStore(cfg.RedisURL, cfg.Generation)
	defer store.Close()
	sid, csrf, e := store.Create(c, session.Record{UserID: actor, SessionRef: operation, AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	defer store.Revoke(c, sid)
	h, close, e := buildHandler(c, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer close()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, cfg.Origin+"/api/v1/applications/"+app+path, strings.NewReader(body))
		r.Header.Set("Origin", cfg.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/structure", "")
	if w.Code != 200 {
		t.Fatalf("composed definition read %d %s", w.Code, w.Body.String())
	}
	w = request("POST", "/forms", `{"operationId":"`+operation+`","name":"表单","source":{"kind":"new_table"},"directoryId":null,"position":0,"expectedStructureVersion":0}`)
	if w.Code != 201 {
		t.Fatalf("composed definition create %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data struct{ Form struct{ ID string } }
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	var saveID string
	p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&saveID)
	w = request("PUT", "/forms/"+out.Data.Form.ID+"/definition", `{"operationId":"`+saveID+`","expectedSchemaVersion":0,"expectedViewVersion":0,"fields":[],"layout":[],"optionMappings":[],"confirmationToken":null}`)
	if w.Code != 200 {
		t.Fatalf("composed actual Save %d %s", w.Code, w.Body.String())
	}
}
