package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func rootDefinitionEnvelope(t *testing.T, w *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if w.Code != status || body["code"] != code || !reflect.DeepEqual(keys, []string{"code", "data", "message", "meta"}) {
		t.Fatalf("expected status %d/code %s/closed envelope; got %d %s", status, code, w.Code, w.Body.String())
	}
	if body["data"] == nil {
		return nil
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data: %s", w.Body.String())
	}
	return data
}
func rootDefinitionFieldSet(t *testing.T, data map[string]any, want []string) {
	t.Helper()
	got := make([]string, 0, len(data))
	for key := range data {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response fields got %v want %v", got, want)
	}
}

func TestDefinitionConfigRequiresDedicatedExplicitKeyAndLimits(t *testing.T) {
	values := map[string]string{"WEAVEOS_DEFINITION_HMAC_KEY": base64.StdEncoding.EncodeToString([]byte("isolated-definition-test-32bytes!")), "WEAVEOS_DEFINITION_KEY_ID": "test", "WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS": "1000", "WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS": "5000"}
	cfg, e := readConfig(func(k string) string { return values[k] })
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(cfg.DefinitionKey, []byte("isolated-definition-test-32bytes!")) || cfg.DefinitionKeyID != "test" || cfg.SchemaLimits.LockTimeout != time.Second || cfg.SchemaLimits.StatementTimeout != 5*time.Second {
		t.Fatal("dedicated key bytes, key identity and exact lock/statement budgets must match explicit configuration")
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
	roles, e := os.ReadFile("../../../../infra/runtime/roles.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(c, string(roles)); e != nil {
		t.Fatal(e)
	}
	// Compose the service using the actual restricted runtime role, while the
	// separate fixture connection owns isolated setup and source application.
	runtimeURL, e := url.Parse(cfg.DatabaseURL)
	if e != nil {
		t.Fatal(e)
	}
	query := runtimeURL.Query()
	query.Set("options", "-c role=auth_app")
	runtimeURL.RawQuery = query.Encode()
	cfg.DatabaseURL = runtimeURL.String()
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
	server := httptest.NewUnstartedServer(nil)
	cfg.Origin = "https://" + server.Listener.Addr().String()
	h, close, e := buildHandler(c, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer close()
	server.Config.Handler = h
	server.StartTLS()
	defer server.Close()
	// The client trusts the isolated server certificate; HTTPS transport,
	// configured origin, Session and CSRF are real rather than an in-memory call.
	client := server.Client()
	serve := func(r *http.Request) *httptest.ResponseRecorder {
		r.RequestURI = ""
		response, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		if response.TLS == nil {
			t.Fatal("real HTTPS transport required")
		}
		body, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		w := httptest.NewRecorder()
		for k, v := range response.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(body)
		return w
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, cfg.Origin+"/api/v1/applications/"+app+path, strings.NewReader(body))
		r.Header.Set("Origin", cfg.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
		w := httptest.NewRecorder()
		*w = *serve(r)
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
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	var saveID string
	if e := p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&saveID); e != nil {
		t.Fatal(e)
	}
	var fieldID string
	if e := p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&fieldID); e != nil {
		t.Fatal(e)
	}
	var referenceID string
	if e := p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&referenceID); e != nil {
		t.Fatal(e)
	}
	w = request("PUT", "/forms/"+out.Data.Form.ID+"/definition", `{"operationId":"`+saveID+`","expectedSchemaVersion":0,"expectedViewVersion":0,"fields":[{"id":"`+fieldID+`","name":"标题","kind":"text","required":false,"default":null,"config":{"maxLength":null},"presentation":{"helpText":null,"displayTimeZone":null}},{"id":"`+referenceID+`","name":"成员引用","kind":"member","required":false,"default":null,"config":{},"presentation":{"helpText":null,"displayTimeZone":null}}],"layout":[],"optionMappings":[],"confirmationToken":null}`)
	if w.Code != 200 {
		t.Fatalf("composed actual Save %d %s", w.Code, w.Body.String())
	}
	newID := func() string {
		var id string
		if e := p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&id); e != nil {
			t.Fatal(e)
		}
		return id
	}
	root := "/forms/" + out.Data.Form.ID
	t.Run("draft list is the frozen items object", func(t *testing.T) {
		w := request("GET", root+"/drafts", "")
		var env struct{ Data struct{ Items []any } }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &env) != nil || env.Data.Items == nil {
			t.Fatalf("frozen data.items array %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("record HTTP and atomic history", func(t *testing.T) {
		createID := newID()
		body := `{"operationId":"` + createID + `","expectedSchemaVersion":1,"values":{"` + fieldID + `":"alpha"}}`
		w := request("POST", root+"/records", body)
		if w.Code != 201 {
			t.Fatalf("published record create want201 got%d %s", w.Code, w.Body.String())
		}
		var mutation struct {
			Data struct {
				ID                           string
				RecordVersion, SchemaVersion int64
			}
		}
		if e := json.Unmarshal(w.Body.Bytes(), &mutation); e != nil {
			t.Fatal(e)
		}
		confirmedCreate := rootDefinitionEnvelope(t, w, 201, "OK")
		rootDefinitionFieldSet(t, confirmedCreate, []string{"operationId", "id", "recordVersion", "schemaVersion", "createdAt", "updatedAt"})
		if confirmedCreate["operationId"] != createID {
			t.Fatal("create receipt lost the actual operation identity")
		}
		id := mutation.Data.ID
		if id == "" || mutation.Data.RecordVersion != 1 || mutation.Data.SchemaVersion != 1 || !strings.HasSuffix(w.Header().Get("Location"), "/records/"+id) {
			t.Fatalf("exact create response %s", w.Body.String())
		}
		w = request("GET", root+"/records/"+id, "")
		record := rootDefinitionEnvelope(t, w, 200, "OK")
		if record["id"] != id || record["appId"] != app || record["viewId"] != out.Data.Form.ID || record["recordVersion"] != float64(1) || !reflect.DeepEqual(record["values"], map[string]any{fieldID: "alpha", referenceID: nil}) {
			t.Fatalf("exact record identity/version/values: %+v", record)
		}
		w = request("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null}`)
		if w.Code != 200 {
			t.Fatalf("actual search %d %s", w.Code, w.Body.String())
		}
		var page struct {
			Data struct {
				Total        int64
				QueryVersion string
			}
		}
		if json.Unmarshal(w.Body.Bytes(), &page) != nil || page.Data.Total != 1 || page.Data.QueryVersion == "" {
			t.Fatalf("complete query page %s", w.Body.String())
		}
		w = request("PATCH", root+"/records/"+id, `{"operationId":"`+newID()+`","expectedSchemaVersion":1,"expectedRecordVersion":1,"queryVersion":"`+page.Data.QueryVersion+`","changes":{"`+fieldID+`":"beta"}}`)
		if w.Code != 200 {
			t.Fatalf("actual edit %d %s", w.Code, w.Body.String())
		}
		w = request("POST", root+"/records", body)
		if replay := rootDefinitionEnvelope(t, w, 201, "OK"); !reflect.DeepEqual(replay, confirmedCreate) {
			t.Fatalf("confirmed replay must equal the original independently validated receipt: %+v", replay)
		}
		w = request("GET", root+"/records/"+id+"/history", "")
		history := rootDefinitionEnvelope(t, w, 200, "OK")
		items, ok := history["items"].([]any)
		if !ok || len(items) != 2 {
			t.Fatalf("expected both history transitions: %+v", history)
		}
		gotChanges := map[float64][]any{}
		for _, raw := range items {
			event := raw.(map[string]any)
			version := event["recordVersionAfter"].(float64)
			if _, duplicate := gotChanges[version]; duplicate {
				t.Fatal("duplicate history version")
			}
			changes := event["changes"].([]any)
			matches := 0
			for _, raw := range changes {
				change := raw.(map[string]any)
				if change["fieldId"] == fieldID {
					matches++
					gotChanges[version] = []any{event["recordVersionBefore"], change["before"], change["after"]}
				}
			}
			if matches != 1 {
				t.Fatal("history must contain exactly one selected field transition per event")
			}
		}
		wantChanges := map[float64][]any{1: {float64(0), nil, "alpha"}, 2: {float64(1), "alpha", "beta"}}
		if !reflect.DeepEqual(gotChanges, wantChanges) {
			t.Fatalf("history transitions got %+v want %+v", gotChanges, wantChanges)
		}
		var count int
		if e := p.QueryRow(c, "SELECT count(*) FROM applications.record_change_events WHERE app_id=$1 AND record_id=$2", app, id).Scan(&count); e != nil || count != 2 {
			t.Fatalf("actual history events %d %v", count, e)
		}
		w = request("PATCH", root+"/records/"+id, `{"operationId":"`+newID()+`","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{}}`)
		rootDefinitionEnvelope(t, w, 409, "APPLICATION_RECORD_CONFLICT")
		w = request("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"queryVersion":"`+page.Data.QueryVersion+`"}`)
		rootDefinitionEnvelope(t, w, 409, "APPLICATION_QUERY_CHANGED")
		w = request("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"queryVersion":"not-a-token"}`)
		rootDefinitionEnvelope(t, w, 409, "APPLICATION_QUERY_CONTEXT_EXPIRED")
		for _, body := range []string{`{"operationId":"` + newID() + `","expectedSchemaVersion":1,"values":{},"extra":true}`, `{"operationId":"` + newID() + `","expectedSchemaVersion":1,"values":{"` + fieldID + `":123}}`, `{"operationId":"` + newID() + `","expectedSchemaVersion":1,"values":{},"values":{}}`} {
			w = request("POST", root+"/records", body)
			rootDefinitionEnvelope(t, w, 400, "COMMON_VALIDATION_FAILED")
		}
		for _, bad := range []struct {
			header, value string
			status        int
			code          string
		}{{"X-CSRF-Token", "invalid", 403, "COMMON_CSRF_REJECTED"}, {"X-Expected-Actor-Id", newID(), 409, "AUTH_SESSION_CHANGED"}} {
			r := httptest.NewRequest("POST", cfg.Origin+"/api/v1/applications/"+app+root+"/records", strings.NewReader(`{"operationId":"`+newID()+`","expectedSchemaVersion":1,"values":{}}`))
			r.Header.Set("Origin", cfg.Origin)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-CSRF-Token", csrf)
			r.Header.Set(bad.header, bad.value)
			r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
			r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
			w := httptest.NewRecorder()
			*w = *serve(r)
			rootDefinitionEnvelope(t, w, bad.status, bad.code)
		}
		if e := p.QueryRow(c, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_kind='record.create'", app).Scan(&count); e != nil || count != 1 {
			t.Fatalf("rejected input/CSRF/actor must have no operation effects %d %v", count, e)
		}
	})
	t.Run("draft HTTP remains separate from records", func(t *testing.T) {
		draftOperation := newID()
		draftBody := `{"operationId":"` + draftOperation + `","targetRecordId":null,"schemaVersion":1,"baseRecordVersion":null,"values":{"` + fieldID + `":"unfinished"}}`
		w := request("POST", root+"/drafts", draftBody)
		if w.Code != 201 {
			t.Fatalf("actual draft create %d %s", w.Code, w.Body.String())
		}
		var draft struct {
			Data struct {
				ID           string
				DraftVersion int64
			}
		}
		if json.Unmarshal(w.Body.Bytes(), &draft) != nil || draft.Data.ID == "" || draft.Data.DraftVersion != 1 {
			t.Fatalf("draft response %s", w.Body.String())
		}
		expectedDraft := map[string]any{"operationId": draftOperation, "id": draft.Data.ID, "draftVersion": float64(1)}
		if got := rootDefinitionEnvelope(t, w, 201, "OK"); !reflect.DeepEqual(got, expectedDraft) {
			t.Fatalf("exact initial draft receipt %+v", got)
		}
		w = request("POST", root+"/drafts", draftBody)
		if got := rootDefinitionEnvelope(t, w, 201, "OK"); !reflect.DeepEqual(got, expectedDraft) {
			t.Fatalf("exact draft replay %+v", got)
		}
		path := root + "/drafts/" + draft.Data.ID
		w = request("GET", path, "")
		draftRead := rootDefinitionEnvelope(t, w, 200, "OK")
		if !reflect.DeepEqual(draftRead["values"], map[string]any{fieldID: "unfinished"}) {
			t.Fatalf("exact incomplete draft input %+v", draftRead)
		}
		patchOperation := newID()
		patchBody := `{"operationId":"` + patchOperation + `","expectedDraftVersion":1,"changes":{},"removeFieldIds":["` + fieldID + `"]}`
		w = request("PATCH", path, patchBody)
		expectedPatch := map[string]any{"operationId": patchOperation, "id": draft.Data.ID, "draftVersion": float64(2)}
		if got := rootDefinitionEnvelope(t, w, 200, "OK"); !reflect.DeepEqual(got, expectedPatch) {
			t.Fatalf("exact draft patch receipt %+v", got)
		}
		w = request("PATCH", path, patchBody)
		if got := rootDefinitionEnvelope(t, w, 200, "OK"); !reflect.DeepEqual(got, expectedPatch) {
			t.Fatalf("exact draft patch replay %+v", got)
		}
		w = request("GET", path, "")
		if got := rootDefinitionEnvelope(t, w, 200, "OK"); !reflect.DeepEqual(got["values"], map[string]any{}) {
			t.Fatalf("sparse removal must persist an empty value object: %+v", got)
		}
		w = request("GET", root+"/drafts?pageSize=1", "")
		list := rootDefinitionEnvelope(t, w, 200, "OK")
		drafts, ok := list["items"].([]any)
		if !ok || len(drafts) != 1 || drafts[0].(map[string]any)["id"] != draft.Data.ID {
			t.Fatalf("exact draft list %+v", list)
		}
		query := path + "?operationId=" + newID() + "&expectedDraftVersion=2"
		w = request("DELETE", query, "")
		if w.Code != 204 || w.Body.Len() != 0 {
			t.Fatalf("discard must be204 bodyless %d %s", w.Code, w.Body.String())
		}
		w = request("DELETE", query, "")
		if w.Code != 204 || w.Body.Len() != 0 {
			t.Fatalf("discard replay204 %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("ordinary runtime uses registered form tuple", func(t *testing.T) {
		var member, group, grant, table string
		if e := p.QueryRow(c, "INSERT INTO auth.users(account) VALUES('ordinary-composition-'||gen_random_uuid()) RETURNING id::text").Scan(&member); e != nil {
			t.Fatal(e)
		}
		if e := p.QueryRow(c, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'ordinary') RETURNING id::text", app).Scan(&group); e != nil {
			t.Fatal(e)
		}
		if _, e := p.Exec(c, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", app, group, member); e != nil {
			t.Fatal(e)
		}
		if _, e := p.Exec(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'menu.enter','all')", app, group, out.Data.Form.ID); e != nil {
			t.Fatal(e)
		}
		if e := p.QueryRow(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'data.read','own') RETURNING id::text", app, group, out.Data.Form.ID).Scan(&grant); e != nil {
			t.Fatal(e)
		}
		if e := p.QueryRow(c, "SELECT table_id::text FROM applications.form_views WHERE id=$1", out.Data.Form.ID).Scan(&table); e != nil {
			t.Fatal(e)
		}
		if _, e := p.Exec(c, "INSERT INTO applications.grant_fields(app_id,grant_id,table_id,field_id) VALUES($1,$2,$3,$4)", app, grant, table, fieldID); e != nil {
			t.Fatal(e)
		}
		memberSID, memberCSRF, e := store.Create(c, session.Record{UserID: member, SessionRef: newID(), AuthVersion: "1"})
		if e != nil {
			t.Fatal(e)
		}
		defer store.Revoke(c, memberSID)
		r := httptest.NewRequest("GET", cfg.Origin+"/api/v1/applications/"+app+root+"/runtime", nil)
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: memberSID})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: memberCSRF})
		w := httptest.NewRecorder()
		*w = *serve(r)
		runtimeData := rootDefinitionEnvelope(t, w, 200, "OK")
		capabilities, ok := runtimeData["capabilities"].(map[string]any)
		if !ok || capabilities["read"] != "own" || capabilities["create"] != false || capabilities["history"] != "none" {
			t.Fatalf("exact ordinary capability paths: %+v", runtimeData)
		}
		rootDefinitionFieldSet(t, capabilities, []string{"create", "read", "edit", "search", "draftCreate", "draftEdit", "history"})
		memberRequest := func(method, path, body string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(method, cfg.Origin+"/api/v1/applications/"+app+path, strings.NewReader(body))
			r.Header.Set("Origin", cfg.Origin)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-CSRF-Token", memberCSRF)
			r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: memberSID})
			r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: memberCSRF})
			w := httptest.NewRecorder()
			*w = *serve(r)
			return w
		}
		w = memberRequest("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null}`)
		ownPage := rootDefinitionEnvelope(t, w, 200, "OK")
		if ownPage["total"] != float64(0) || !reflect.DeepEqual(ownPage["items"], []any{}) {
			t.Fatalf("own predicate must return the exact empty page: %+v", ownPage)
		}
		if _, e := p.Exec(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'data.create','all')", app, group, out.Data.Form.ID); e != nil {
			t.Fatal(e)
		}
		w = memberRequest("POST", root+"/records", `{"operationId":"`+newID()+`","expectedSchemaVersion":1,"values":{"`+fieldID+`":"unauthorized"}}`)
		if w.Code != 403 {
			t.Fatalf("empty create mask forbids explicit field %d %s", w.Code, w.Body.String())
		}
		w = memberRequest("POST", root+"/records", `{"operationId":"`+newID()+`","expectedSchemaVersion":1,"values":{}}`)
		if w.Code != 201 {
			t.Fatalf("ordinary defaults-only create %d %s", w.Code, w.Body.String())
		}
		var mine struct{ Data struct{ ID string } }
		if json.Unmarshal(w.Body.Bytes(), &mine) != nil || mine.Data.ID == "" {
			t.Fatalf("ordinary created ID %s", w.Body.String())
		}
		w = memberRequest("GET", root+"/records/"+mine.Data.ID, "")
		own := rootDefinitionEnvelope(t, w, 200, "OK")
		if own["id"] != mine.Data.ID || own["createdBy"] != member {
			t.Fatalf("ordinary own record binding: %+v", own)
		}
		w = memberRequest("GET", root+"/runtime", "")
		runtimeData = rootDefinitionEnvelope(t, w, 200, "OK")
		capabilities, ok = runtimeData["capabilities"].(map[string]any)
		if !ok || capabilities["create"] != true {
			t.Fatalf("defaults-only create capability: %+v", runtimeData)
		}
		t.Run("field scoped ordinary reference candidates", func(t *testing.T) {
			var createGrant, editGrant string
			var referenceGroup string
			if e := p.QueryRow(c, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'reference-selector') RETURNING id::text", app).Scan(&referenceGroup); e != nil {
				t.Fatal(e)
			}
			if _, e := p.Exec(c, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", app, referenceGroup, member); e != nil {
				t.Fatal(e)
			}
			for _, entry := range []struct {
				action, scope string
				dest          *string
			}{{"data.create", "all", &createGrant}, {"data.edit", "own", &editGrant}} {
				if e := p.QueryRow(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,$4,$5) RETURNING id::text", app, referenceGroup, out.Data.Form.ID, entry.action, entry.scope).Scan(entry.dest); e != nil {
					t.Fatal(e)
				}
				if _, e := p.Exec(c, "INSERT INTO applications.grant_fields(app_id,grant_id,table_id,field_id) VALUES($1,$2,$3,$4)", app, *entry.dest, table, referenceID); e != nil {
					t.Fatal(e)
				}
			}
			prefix := "ordinary-candidate-" + newID() + "-"
			for _, ending := range []string{"a", "b", "c"} {
				if _, e := p.Exec(c, "INSERT INTO auth.users(account) VALUES($1)", prefix+ending); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := p.Exec(c, "INSERT INTO auth.users(account,status) VALUES($1,'disabled')", prefix+"inactive"); e != nil {
				t.Fatal(e)
			}
			path := root + "/reference-candidates?fieldId=" + referenceID + "&action=create&q=" + prefix + "&pageSize=2"
			w := memberRequest("GET", path, "")
			if w.Code != 200 {
				t.Fatalf("field/action authorized real candidates %d %s", w.Code, w.Body.String())
			}
			var page struct {
				Data struct{ Items []map[string]any }
				Meta struct {
					Pagination struct {
						NextPageToken *string
						HasMore       bool
					}
				}
			}
			if json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Data.Items) != 2 || len(page.Data.Items[0]) != 3 || page.Data.Items[0]["label"] != prefix+"a" || !page.Meta.Pagination.HasMore || page.Meta.Pagination.NextPageToken == nil {
				t.Fatalf("minimal stable active candidates %s", w.Body.String())
			}
			token := *page.Meta.Pagination.NextPageToken
			w = memberRequest("GET", path+"&pageToken="+token, "")
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Data.Items) != 1 || page.Data.Items[0]["label"] != prefix+"c" {
				t.Fatalf("opaque second page %d %s", w.Code, w.Body.String())
			}
			w = memberRequest("GET", root+"/reference-candidates?fieldId="+referenceID+"&action=edit&recordId="+mine.Data.ID+"&q="+prefix+"&pageToken="+token, "")
			if w.Code != 400 {
				t.Fatalf("cursor must bind action/record %d %s", w.Code, w.Body.String())
			}
			w = memberRequest("GET", root+"/reference-candidates?fieldId="+referenceID+"&action=edit&recordId="+mine.Data.ID+"&q="+prefix, "")
			if w.Code != 200 {
				t.Fatalf("own field edit candidates %d %s", w.Code, w.Body.String())
			}
			ownerResponse := request("POST", root+"/records", `{"operationId":"`+newID()+`","expectedSchemaVersion":1,"values":{}}`)
			var theirs struct{ Data struct{ ID string } }
			if ownerResponse.Code != 201 || json.Unmarshal(ownerResponse.Body.Bytes(), &theirs) != nil {
				t.Fatal("owned fixture create", ownerResponse.Body.String())
			}
			w = memberRequest("GET", root+"/reference-candidates?fieldId="+referenceID+"&action=edit&recordId="+theirs.Data.ID+"&q="+prefix, "")
			if w.Code != 403 {
				t.Fatalf("own edit candidates must reject others row %d %s", w.Code, w.Body.String())
			}
			if _, e := p.Exec(c, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", app); e != nil {
				t.Fatal(e)
			}
			w = memberRequest("GET", path+"&pageToken="+token, "")
			if w.Code != 400 {
				t.Fatalf("candidate cursor must bind current policy %d %s", w.Code, w.Body.String())
			}
			// Remove actual create mask while keeping empty-mask create authority.
			if _, e := p.Exec(c, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id=$2", app, createGrant); e != nil {
				t.Fatal(e)
			}
			w = memberRequest("GET", path+"&pageToken="+token, "")
			if w.Code != 403 {
				t.Fatalf("revoked field candidate permission %d %s", w.Code, w.Body.String())
			}
		})
		// Revoke the real form entry while leaving root menu and data tuples.
		if _, e := p.Exec(c, "DELETE FROM applications.grants WHERE app_id=$1 AND group_id=$2 AND action='menu.enter'", app, group); e != nil {
			t.Fatal(e)
		}
		if _, e := p.Exec(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'application',$1,'menu.enter','all')", app, group); e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{root + "/runtime", root + "/records/" + mine.Data.ID} {
			w = memberRequest("GET", path, "")
			if w.Code != 403 {
				t.Fatalf("root menu must not inherit child authority %d %s", w.Code, w.Body.String())
			}
		}
		w = memberRequest("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"queryVersion":"expired"}`)
		if w.Code != 403 {
			t.Fatalf("actual authorization precedes expired context %d %s", w.Code, w.Body.String())
		}
	})
}
