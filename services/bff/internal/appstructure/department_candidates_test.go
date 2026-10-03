package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func departmentCall(t *testing.T, f *fixture, query string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://weaveos.test/api/v1/applications/"+f.app+"/department-candidates"+query, nil)
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
	w := httptest.NewRecorder()
	f.service.ServeHTTP(w, r)
	return w
}

func TestDepartmentCandidatesAreLiveMinimalAndCursorScoped(t *testing.T) {
	f := candidateFixture(t)
	prefix := "v013-dept-" + uuid(t, f.owner) + "-"
	var parent string
	if e := f.owner.QueryRow(context.Background(), "SELECT id::text FROM personnel.departments WHERE is_root LIMIT 1").Scan(&parent); e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for _, suffix := range []string{"a", "b", "c"} {
		var id string
		if e := f.owner.QueryRow(context.Background(), "INSERT INTO personnel.departments(name,parent_id) VALUES($1,$2) RETURNING id::text", prefix+suffix, parent).Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	w := departmentCall(t, f, "?q="+url.QueryEscape(" "+prefix+" ")+"&pageSize=2")
	out := data(t, w, 200)
	items := out["items"].([]any)
	if len(items) != 2 || len(items[0].(map[string]any)) != 4 || items[0].(map[string]any)["id"] != ids[0] || items[0].(map[string]any)["label"] != prefix+"a" || items[0].(map[string]any)["parentId"] != parent || items[0].(map[string]any)["status"] != "active" {
		t.Fatal("actual minimal department source", out)
	}
	var envelope struct {
		Meta struct{ Pagination Pagination }
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)
	if !envelope.Meta.Pagination.HasMore || envelope.Meta.Pagination.NextPageToken == nil {
		t.Fatal("standard cursor missing")
	}
	token := *envelope.Meta.Pagination.NextPageToken
	if w := candidateCall(t, f, "?q="+url.QueryEscape(prefix)+"&pageToken="+token); w.Code != 400 {
		t.Fatalf("department cursor cannot replay into member source %d %s", w.Code, w.Body.String())
	}
	out = data(t, departmentCall(t, f, "?q="+url.QueryEscape(prefix)+"&pageToken="+token), 200)
	if len(out["items"].([]any)) != 1 || out["items"].([]any)[0].(map[string]any)["id"] != ids[2] {
		t.Fatal("stable name/id keyset", out)
	}
	if _, e := f.owner.Exec(context.Background(), "DELETE FROM personnel.departments WHERE id=$1", ids[1]); e != nil {
		t.Fatal(e)
	}
	out = data(t, departmentCall(t, f, "?q="+url.QueryEscape(prefix)), 200)
	if len(out["items"].([]any)) != 2 {
		t.Fatal("deleted department must disappear", out)
	}
	view, _ := newForm(t, f)
	f.service.Application.References = CurrentSources{}
	removed := field(t, f, "department", ids[1], map[string]any{})
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, removed), 400, "APPLICATION_RESOURCE_INVALID")
}

func TestDepartmentCandidatesUseSharedDispatchAuthorizationAndActorGuard(t *testing.T) {
	f := candidateFixture(t)
	path := "/api/v1/applications/" + f.app + "/department-candidates"
	data(t, rootCall(t, f, "GET", path, nil, f.actor), 200)
	for _, query := range []string{"?q=" + url.QueryEscape(string(make([]byte, 1))), "?q=" + url.QueryEscape(strings.Repeat("界", 101)), "?pageSize=0", "?pageSize=51", "?pageSize=1&pageSize=2", "?pageToken=garbage"} {
		if w := rootCall(t, f, "GET", path+query, nil, f.actor); w.Code != 400 {
			t.Fatalf("strict department query %d %s", w.Code, w.Body.String())
		}
	}
	if w := rootCall(t, f, "GET", path, nil, uuid(t, f.owner)); w.Code != 409 {
		t.Fatalf("same actor guard on shared route %d %s", w.Code, w.Body.String())
	}
	foreign := setup(t)
	if w := rootCall(t, foreign, "GET", path, nil, foreign.actor); w.Code != 403 {
		t.Fatalf("foreign app department candidate read %d %s", w.Code, w.Body.String())
	}
}
