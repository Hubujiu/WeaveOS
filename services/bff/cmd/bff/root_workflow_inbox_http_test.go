package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const inboxHTTPPath = "/api/v1/workflow-tasks/search"

func TestRootWorkflowInboxHTTPSRealAuthenticatedPage(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	data := rootHTTPData(t, f.call(t, "POST", inboxHTTPPath, `{"page":1}`, nil), 200)
	rootHTTPFieldSet(t, data, []string{"items", "total", "page", "pageSize", "queryVersion"})
	if string(data["total"]) != "1" || string(data["pageSize"]) != "20" {
		t.Fatal("bad page counters")
	}
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(data["items"], &items); e != nil || len(items) != 1 {
		t.Fatal("missing actual assigned task", e)
	}
	rootHTTPFieldSet(t, items[0], []string{"id", "appId", "viewId", "recordId", "instanceId", "flowId", "nodeId", "flowName", "createdAt", "definitionVersion", "activationEpoch", "sequence"})
	for key, want := range map[string]string{"id": f.task, "appId": f.app, "viewId": f.view, "recordId": f.record, "instanceId": f.instance, "flowId": f.flow, "nodeId": f.node, "flowName": "Approval"} {
		if rootHTTPString(t, items[0], key) != want {
			t.Fatal("incorrect identity", key)
		}
	}
	token := rootHTTPString(t, data, "queryVersion")
	if token == "" {
		t.Fatal("missing context")
	}
	body := `{"page":2,"pageSize":1,"queryVersion":"` + token + `"}`
	next := rootHTTPData(t, f.call(t, "POST", inboxHTTPPath, body, nil), 200)
	if string(next["items"]) != "[]" || string(next["total"]) != "1" {
		t.Fatal("beyond-total query not empty")
	}
	// Use the real existing task preview route, proving these locators connect.
	rootHTTPData(t, f.call(t, "GET", f.taskPath()+"/preview", "", nil), 200)
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_definitions SET name='Changed' WHERE app_id=$1 AND id=$2", f.app, f.flow); e != nil {
		t.Fatal(e)
	}
	rootHTTPError(t, f.call(t, "POST", inboxHTTPPath, body, nil), 409, "APPLICATION_QUERY_CHANGED")
}
func TestRootWorkflowInboxHTTPSIngressAndClosedJSON(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	body := `{"page":1}`
	for _, c := range []struct {
		name   string
		status int
		code   string
		edit   func(*http.Request)
	}{
		{"session", 401, "AUTH_UNAUTHENTICATED", func(r *http.Request) { r.Header.Del("Cookie") }},
		{"csrf", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }},
		{"origin", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Set("Origin", "https://foreign.invalid") }},
		{"actor", 409, "AUTH_SESSION_CHANGED", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
	} {
		t.Run(c.name, func(t *testing.T) { rootHTTPError(t, f.call(t, "POST", inboxHTTPPath, body, c.edit), c.status, c.code) })
	}
	for _, raw := range []string{"null", "[]", "{}", body + "{}", `{"page":1,"page":2}`, `{"page":1,"actorId":"` + f.actor + `"}`, `{"page":1,"appId":"` + f.app + `"}`, `{"page":null}`, `{"page":0}`, `{"page":1.5}`, `{"page":1,"pageSize":null}`, `{"page":1,"pageSize":0}`, `{"page":1,"pageSize":101}`, `{"page":9007199254740991,"pageSize":100}`, `{"page":1,"queryVersion":null}`, strings.Repeat(" ", 4097) + body} {
		rootHTTPError(t, f.call(t, "POST", inboxHTTPPath, raw, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "POST", inboxHTTPPath+"?x=1", body, nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", inboxHTTPPath, body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		rootHTTPError(t, f.call(t, method, inboxHTTPPath, body, nil), 404, "API_NOT_FOUND")
	}
	rootHTTPError(t, f.call(t, "POST", inboxHTTPPath+"/", body, nil), 404, "API_NOT_FOUND")
}
