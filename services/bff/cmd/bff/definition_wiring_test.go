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
	json.Unmarshal(w.Body.Bytes(), &out)
	var saveID string
	p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&saveID)
	var fieldID string
	p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&fieldID)
	var referenceID string
	p.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&referenceID)
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
		id := mutation.Data.ID
		if id == "" || mutation.Data.RecordVersion != 1 || mutation.Data.SchemaVersion != 1 || !strings.HasSuffix(w.Header().Get("Location"), "/records/"+id) {
			t.Fatalf("exact create response %s", w.Body.String())
		}
		w = request("GET", root+"/records/"+id, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"alpha"`) || !strings.Contains(w.Body.String(), `"appId":"`+app+`"`) {
			t.Fatalf("same RR record read %d %s", w.Code, w.Body.String())
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
		if w.Code != 201 || strings.Contains(w.Body.String(), `"alpha"`) || strings.Contains(w.Body.String(), `"beta"`) {
			t.Fatalf("minimum confirmed replay %d %s", w.Code, w.Body.String())
		}
		w = request("GET", root+"/records/"+id+"/history", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"alpha"`) || !strings.Contains(w.Body.String(), `"beta"`) {
			t.Fatalf("real same-Tx history %d %s", w.Code, w.Body.String())
		}
		var count int
		if e := p.QueryRow(c, "SELECT count(*) FROM applications.record_change_events WHERE app_id=$1 AND record_id=$2", app, id).Scan(&count); e != nil || count != 2 {
			t.Fatalf("actual history events %d %v", count, e)
		}
		w = request("PATCH", root+"/records/"+id, `{"operationId":"`+newID()+`","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{}}`)
		if w.Code != 409 || !strings.Contains(w.Body.String(), `"APPLICATION_RECORD_CONFLICT"`) {
			t.Fatalf("stale record CAS %d %s", w.Code, w.Body.String())
		}
		w = request("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"queryVersion":"`+page.Data.QueryVersion+`"}`)
		if w.Code != 409 || !strings.Contains(w.Body.String(), `"APPLICATION_QUERY_CHANGED"`) {
			t.Fatalf("changed saved projection %d %s", w.Code, w.Body.String())
		}
		w = request("POST", root+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"queryVersion":"not-a-token"}`)
		if w.Code != 409 || !strings.Contains(w.Body.String(), `"APPLICATION_QUERY_CONTEXT_EXPIRED"`) {
			t.Fatalf("expired invalid context %d %s", w.Code, w.Body.String())
		}
		for _, body := range []string{`{"operationId":"` + newID() + `","expectedSchemaVersion":1,"values":{},"extra":true}`, `{"operationId":"` + newID() + `","expectedSchemaVersion":1,"values":{"` + fieldID + `":123}}`, `{"operationId":"` + newID() + `","expectedSchemaVersion":1,"values":{},"values":{}}`} {
			w = request("POST", root+"/records", body)
			if w.Code != 400 || !strings.Contains(w.Body.String(), `"COMMON_VALIDATION_FAILED"`) {
				t.Fatalf("strict record body %d %s", w.Code, w.Body.String())
			}
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
			if w.Code != bad.status || !strings.Contains(w.Body.String(), bad.code) {
				t.Fatalf("shared guard %s %d %s", bad.header, w.Code, w.Body.String())
			}
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
		var minimum struct{ Data map[string]any }
		if json.Unmarshal(w.Body.Bytes(), &minimum) != nil || len(minimum.Data) != 3 || minimum.Data["operationId"] != draftOperation || strings.Contains(w.Body.String(), "unfinished") {
			t.Fatalf("ADR14 normal draft write must exact minimum %s", w.Body.String())
		}
		w = request("POST", root+"/drafts", draftBody)
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &minimum) != nil || len(minimum.Data) != 3 || minimum.Data["operationId"] != draftOperation || minimum.Data["id"] != draft.Data.ID || strings.Contains(w.Body.String(), "unfinished") {
			t.Fatalf("ADR14 replay must same minimum %s", w.Body.String())
		}
		path := root + "/drafts/" + draft.Data.ID
		w = request("GET", path, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"unfinished"`) {
			t.Fatalf("draft read %d %s", w.Code, w.Body.String())
		}
		patchOperation := newID()
		patchBody := `{"operationId":"` + patchOperation + `","expectedDraftVersion":1,"changes":{},"removeFieldIds":["` + fieldID + `"]}`
		w = request("PATCH", path, patchBody)
		if w.Code != 200 || strings.Contains(w.Body.String(), `"unfinished"`) {
			t.Fatalf("draft sparse remove %d %s", w.Code, w.Body.String())
		}
		if json.Unmarshal(w.Body.Bytes(), &minimum) != nil || len(minimum.Data) != 3 || minimum.Data["operationId"] != patchOperation || minimum.Data["draftVersion"] != float64(2) {
			t.Fatalf("ADR14 patch minimum %s", w.Body.String())
		}
		w = request("PATCH", path, patchBody)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &minimum) != nil || len(minimum.Data) != 3 || minimum.Data["draftVersion"] != float64(2) {
			t.Fatalf("ADR14 patch confirmed replay %d %s", w.Code, w.Body.String())
		}
		w = request("GET", root+"/drafts?pageSize=1", "")
		if w.Code != 200 || !strings.HasPrefix(strings.TrimSpace(w.Body.String()), `{"code":"OK"`) || !strings.Contains(w.Body.String(), `"pagination"`) {
			t.Fatalf("draft list envelope %d %s", w.Code, w.Body.String())
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
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"read":"own"`) || strings.Contains(w.Body.String(), `"create":true`) {
			t.Fatalf("ordinary runtime actual grant projection %d %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"history":"none"`) {
			t.Fatalf("ADR14 runtime lacks read/history intersection %s", w.Body.String())
		}
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
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"total":0`) {
			t.Fatalf("immutable createdBy own predicate before COUNT %d %s", w.Code, w.Body.String())
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
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"createdBy":"`+member+`"`) {
			t.Fatalf("ordinary own read %d %s", w.Code, w.Body.String())
		}
		w = memberRequest("GET", root+"/runtime", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"create":true`) {
			t.Fatalf("defaults-only runtime capability %d %s", w.Code, w.Body.String())
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
