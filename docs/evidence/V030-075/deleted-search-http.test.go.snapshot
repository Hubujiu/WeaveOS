package main

import (
	"encoding/json"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// Independent P19 oracle: retained records, current manager authority, no values or actors.
func rootDeletedPath(f *rootTaskHTTPFixture, view string) string {
	return "/api/v1/applications/" + f.app + "/forms/" + view + "/records/deleted/search"
}
func rootDeletedQuery(page, size int, token string) string {
	q := map[string]any{"page": page, "pageSize": size}
	if token != "" {
		q["queryVersion"] = token
	}
	b, _ := json.Marshal(q)
	return string(b)
}
func rootDeletedItems(t *testing.T, data map[string]json.RawMessage) []map[string]json.RawMessage {
	t.Helper()
	rootHTTPFieldSet(t, data, []string{"items", "total", "page", "pageSize", "queryVersion", "schemaVersion"})
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(data["items"], &items); e != nil || items == nil {
		t.Fatal("items must be an explicit array", e)
	}
	for _, item := range items {
		rootHTTPFieldSet(t, item, []string{"id", "recordVersion", "deletedAt"})
		if _, e := time.Parse(time.RFC3339Nano, rootHTTPString(t, item, "deletedAt")); e != nil {
			t.Fatal("invalid deletion time", e)
		}
	}
	return items
}
func rootDeletedCounts(t *testing.T, f *rootTaskHTTPFixture) [3]int64 {
	t.Helper()
	var out [3]int64
	for i, table := range []string{"operations", "record_lifecycle", "record_lifecycle_events"} {
		if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1", f.app).Scan(&out[i]); e != nil {
			t.Fatal(e)
		}
	}
	return out
}
func rootDeletedSession(t *testing.T, f *rootTaskHTTPFixture, actor, version string) func(*http.Request) {
	t.Helper()
	sid, csrf, e := f.store.Create(f.ctx, session.Record{UserID: actor, SessionRef: f.id(t), AuthVersion: version})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.store.Revoke(f.ctx, sid) })
	return func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("X-Expected-Actor-Id", actor)
	}
}
func rootDeletedDelete(t *testing.T, f *rootTaskHTTPFixture, id string) {
	t.Helper()
	rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/records/"+id+"/deletion", rootLifecycleBody(f.id(t), "1"), nil), 200)
}
func TestRootDeletedRecordsHostMinimalProjectionAndNoBusinessWrites(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := rootDeletedPath(f, f.view)
	empty := rootHTTPData(t, f.call(t, "POST", path, rootDeletedQuery(1, 20, ""), nil), 200)
	if len(rootDeletedItems(t, empty)) != 0 || string(empty["total"]) != "0" {
		t.Fatal("normal record exposed as deleted")
	}
	rootDeletedDelete(t, f, f.record)
	before := rootDeletedCounts(t, f)
	w := f.call(t, "POST", path, rootDeletedQuery(1, 20, ""), nil)
	data := rootHTTPData(t, w, 200)
	items := rootDeletedItems(t, data)
	if len(items) != 1 || rootHTTPString(t, items[0], "id") != f.record || string(items[0]["recordVersion"]) != "2" || string(data["schemaVersion"]) != "1" || string(data["total"]) != "1" {
		t.Fatal("wrong retained result", w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-Id") == "" {
		t.Fatal("private envelope missing")
	}
	if !reflect.DeepEqual(before, rootDeletedCounts(t, f)) {
		t.Fatal("search wrote business state")
	}
	rootHTTPError(t, f.call(t, "GET", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/records/"+f.record, "", nil), 404, "APPLICATION_NOT_FOUND")
}
func TestRootDeletedRecordsPagesAndExplicitRefreshAfterRestore(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	base := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records"
	path := rootDeletedPath(f, f.view)
	second := rootHTTPData(t, f.call(t, "POST", base, fmt.Sprintf(`{"operationId":%q,"expectedSchemaVersion":1,"values":{%q:"second private value"}}`, f.id(t), f.field), nil), 201)
	other := rootHTTPString(t, second, "id")
	rootDeletedDelete(t, f, f.record)
	rootDeletedDelete(t, f, other)
	// Fixture-controlled timestamps ensure an independent deterministic ordering oracle.
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.record_lifecycle SET changed_at=CASE WHEN record_id=$2 THEN '2026-10-12T00:00:00Z'::timestamptz ELSE '2026-10-13T00:00:00Z'::timestamptz END WHERE app_id=$1", f.app, f.record); e != nil {
		t.Fatal(e)
	}
	first := rootHTTPData(t, f.call(t, "POST", path, rootDeletedQuery(1, 1, ""), nil), 200)
	one := rootDeletedItems(t, first)
	token := rootHTTPString(t, first, "queryVersion")
	if len(one) != 1 || rootHTTPString(t, one[0], "id") != other || string(first["total"]) != "2" {
		t.Fatal("wrong descending first page")
	}
	next := rootHTTPData(t, f.call(t, "POST", path, rootDeletedQuery(2, 1, token), nil), 200)
	two := rootDeletedItems(t, next)
	if len(two) != 1 || rootHTTPString(t, two[0], "id") != f.record || rootHTTPString(t, next, "queryVersion") != token {
		t.Fatal("unstable next page")
	}
	// An unrelated application does not invalidate this projection.
	unrelated := rootHTTPResourceSetup(t)
	rootDeletedDelete(t, unrelated, unrelated.record)
	rootHTTPData(t, f.call(t, "POST", path, rootDeletedQuery(2, 1, token), nil), 200)
	rootHTTPData(t, f.call(t, "POST", base+"/"+other+"/restoration", rootLifecycleBody(f.id(t), "2"), nil), 200)
	rootHTTPError(t, f.call(t, "POST", path, rootDeletedQuery(1, 1, token), nil), 409, "APPLICATION_QUERY_CHANGED")
	fresh := rootHTTPData(t, f.call(t, "POST", path, rootDeletedQuery(1, 20, ""), nil), 200)
	if len(rootDeletedItems(t, fresh)) != 1 || string(fresh["total"]) != "1" {
		t.Fatal("explicit refresh did not remove restored row")
	}
	rootHTTPData(t, f.call(t, "GET", base+"/"+other, "", nil), 200)
}
func TestRootDeletedRecordsSharedViewsAndSessionContextIsolation(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	d := rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+f.app+"/forms", fmt.Sprintf(`{"operationId":%q,"name":"Other retained view","source":{"kind":"existing_table","tableId":%q},"directoryId":null,"position":0,"expectedStructureVersion":1}`, f.id(t), f.table), nil), 201)
	var view struct{ ID string }
	if json.Unmarshal(d["form"], &view) != nil || view.ID == "" {
		t.Fatal("second view missing")
	}
	rootDeletedDelete(t, f, f.record)
	first := rootHTTPData(t, f.call(t, "POST", rootDeletedPath(f, f.view), rootDeletedQuery(1, 20, ""), nil), 200)
	token := rootHTTPString(t, first, "queryVersion")
	shared := rootHTTPData(t, f.call(t, "POST", rootDeletedPath(f, view.ID), rootDeletedQuery(1, 20, ""), nil), 200)
	if !reflect.DeepEqual(rootDeletedItems(t, first), rootDeletedItems(t, shared)) {
		t.Fatal("shared table lifecycle differed")
	}
	rootHTTPError(t, f.call(t, "POST", rootDeletedPath(f, view.ID), rootDeletedQuery(1, 20, token), nil), 409, "APPLICATION_QUERY_CONTEXT_EXPIRED")
	rootHTTPError(t, f.call(t, "POST", rootDeletedPath(f, f.view), rootDeletedQuery(1, 20, token), rootDeletedSession(t, f, f.actor, "1")), 409, "APPLICATION_QUERY_CONTEXT_EXPIRED")
	// A normal records token cannot stand in for the independent deletion projection.
	normal := rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/records/search", `{"page":1,"pageSize":20,"filter":null,"sort":null}`, nil), 200)
	rootHTTPError(t, f.call(t, "POST", rootDeletedPath(f, f.view), rootDeletedQuery(1, 20, rootHTTPString(t, normal, "queryVersion")), nil), 409, "APPLICATION_QUERY_CONTEXT_EXPIRED")
}
func TestRootDeletedRecordsCurrentManagerNotOrdinaryEditOrStaleAuthority(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	rootDeletedDelete(t, f, f.record)
	path := rootDeletedPath(f, f.view)
	body := rootDeletedQuery(1, 20, "")
	rootPresetGrant(t, f, f.other, f.view)
	group := f.id(t)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.permission_groups(id,app_id,name) VALUES($1,$2,$3)", group, f.app, "deleted-search-"+group); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", f.app, group, f.other); e != nil {
		t.Fatal(e)
	}
	var grant string
	if e := f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'data.edit','all') RETURNING id::text", f.app, group, f.view).Scan(&grant); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, f.field, f.table); e != nil {
		t.Fatal(e)
	}
	rootHTTPError(t, f.call(t, "POST", path, body, rootDeletedSession(t, f, f.other, "1")), 403, "APPLICATION_FORBIDDEN")
	first := rootHTTPData(t, f.call(t, "POST", path, body, nil), 200)
	token := rootHTTPString(t, first, "queryVersion")
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.apps SET owner_user_id=$2 WHERE id=$1", f.app, f.other); e != nil {
		t.Fatal(e)
	}
	rootHTTPError(t, f.call(t, "POST", path, rootDeletedQuery(1, 20, token), nil), 403, "APPLICATION_FORBIDDEN")
	root := rootTemplateCurrentBootstrap(t, f, f.other)
	rootHTTPData(t, f.call(t, "POST", path, body, rootDeletedSession(t, f, root.UserID, root.Record.AuthVersion)), 200)
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor); e != nil {
		t.Fatal(e)
	}
	rootHTTPError(t, f.call(t, "POST", path, body, nil), 401, "AUTH_UNAUTHENTICATED")
}
func TestRootDeletedRecordsStrictHTTPBoundary(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := rootDeletedPath(f, f.view)
	body := rootDeletedQuery(1, 20, "")
	for _, bad := range []string{`{}`, `{"page":0,"pageSize":20}`, `{"page":1,"pageSize":101}`, `{"page":1,"page":2,"pageSize":20}`, `{"page":1,"pageSize":20,"queryVersion":null}`, `{"page":1,"pageSize":20,"queryVersion":""}`, `{"page":1,"pageSize":20,"deleted":false}`} {
		rootHTTPError(t, f.call(t, "POST", path, bad, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "POST", path+"?force=true", body, nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", path, body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	rootHTTPError(t, f.call(t, "POST", path, body, func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 403, "COMMON_CSRF_REJECTED")
	rootHTTPError(t, f.call(t, "POST", path, body, func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "POST", path, body, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409, "AUTH_SESSION_CHANGED")
	rootHTTPError(t, f.call(t, "GET", path, "", nil), 404, "API_NOT_FOUND")
}
