package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func rootWorkflowReadPath(f *rootTaskHTTPFixture) string {
	return fmt.Sprintf("/api/v1/applications/%s/forms/%s/records/%s/workflow-instances/search", f.app, f.view, f.record)
}
func TestRootWorkflowReadHTTPSCurrentSummaryAndClosedProjection(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	data := rootHTTPData(t, f.call(t, "POST", rootWorkflowReadPath(f), `{"page":1}`, nil), 200)
	if len(data) != 5 {
		t.Fatalf("page keys %v", data)
	}
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(data["items"], &items); e != nil || len(items) != 1 {
		t.Fatalf("items %s %v", data["items"], e)
	}
	if len(items[0]) != 9 {
		t.Fatalf("private or missing projection keys %v", items[0])
	}
	if rootHTTPString(t, items[0], "id") != f.instance || rootHTTPString(t, items[0], "state") != "active" || string(data["pageSize"]) != "20" || rootHTTPString(t, data, "queryVersion") == "" {
		t.Fatalf("wrong summary %v", data)
	}
}
func TestRootWorkflowReadHTTPSClosedBody(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootWorkflowReadPath(f)
	rootHTTPData(t, f.call(t, "POST", path, `{"page":1,"pageSize":20}`, nil), 200)
	for _, body := range []string{`{}`, `{"page":0}`, `{"page":1,"pageSize":101}`, `{"page":1,"pageSize":null}`, `{"page":1,"queryVersion":null}`, `{"page":1,"sql":"select secret"}`, `{"page":1,"page":2}`, `{"page":"1"}`, `{"page":1} {}`} {
		r := f.call(t, "POST", path, body, nil)
		if r.Code != 400 {
			t.Fatalf("body %s got%d %s", body, r.Code, r.Body.String())
		}
	}
	if r := f.call(t, "POST", path+"?page=1", `{"page":1}`, nil); r.Code != 400 {
		t.Fatalf("query accepted %d", r.Code)
	}
}
func TestRootWorkflowReadHTTPSIdentityAndCSRFAreRequired(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootWorkflowReadPath(f)
	rootHTTPData(t, f.call(t, "POST", path, `{"page":1}`, nil), 200)
	for _, c := range []struct {
		status int
		edit   func(*http.Request)
	}{{401, func(r *http.Request) { r.Header.Del("Cookie") }}, {403, func(r *http.Request) { r.Header.Set("X-CSRF-Token", "invalid") }}, {409, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }}} {
		if r := f.call(t, "POST", path, `{"page":1}`, c.edit); r.Code != c.status {
			t.Fatalf("want%d got%d", c.status, r.Code)
		}
	}
}
func TestRootWorkflowReadHTTPSChangedSummaryRequiresRefresh(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootWorkflowReadPath(f)
	a := rootHTTPData(t, f.call(t, "POST", path, `{"page":1}`, nil), 200)
	token := rootHTTPString(t, a, "queryVersion")
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET sequence=sequence+1 WHERE id=$1", f.instance); e != nil {
		t.Fatal(e)
	}
	r := f.call(t, "POST", path, fmt.Sprintf(`{"page":1,"queryVersion":%q}`, token), nil)
	if r.Code != 409 {
		t.Fatalf("stale summary not rejected %d %s", r.Code, r.Body.String())
	}
	rootHTTPData(t, f.call(t, "POST", path, `{"page":1}`, nil), 200)
}
