package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const approvedHTTPPath = "/api/v1/workflow-approvals/search"

func TestRootPersonalApprovedHTTPSRealConfirmedEventAndHistoryLink(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	eventPath := rootHTTPConfirmedEvent(t, f)
	eventID := eventPath[strings.LastIndex(eventPath, "/")+1:]
	data := rootHTTPData(t, f.call(t, "POST", approvedHTTPPath, `{"page":1}`, nil), 200)
	rootHTTPFieldSet(t, data, []string{"items", "total", "page", "pageSize", "queryVersion"})
	if string(data["total"]) != "1" || string(data["pageSize"]) != "20" {
		t.Fatal("wrong approved count", string(data["total"]))
	}
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(data["items"], &items); e != nil || len(items) != 1 {
		t.Fatal("missing confirmed event", e)
	}
	item := items[0]
	rootHTTPFieldSet(t, item, []string{"appId", "viewId", "recordId", "event"})
	for key, want := range map[string]string{"appId": f.app, "viewId": f.view, "recordId": f.record} {
		if rootHTTPString(t, item, key) != want {
			t.Fatal("wrong resource locator", key)
		}
	}
	var event map[string]json.RawMessage
	if e := json.Unmarshal(item["event"], &event); e != nil {
		t.Fatal(e)
	}
	rootHTTPFieldSet(t, event, []string{"flowName", "flowNameSource", "id", "instanceId", "flowId", "nodeId", "targetNodeId", "actorId", "action", "outcome", "sequence", "schemaVersion", "recordVersion", "occurredAt"})
	for key, want := range map[string]string{"id": eventID, "actorId": f.actor, "nodeId": f.node, "action": "agree", "outcome": "success", "flowName": "Approval", "flowNameSource": "captured"} {
		if rootHTTPString(t, event, key) != want {
			t.Fatal("wrong approved journal", key)
		}
	}
	detail := rootHTTPData(t, f.call(t, "GET", eventPath, "", nil), 200)
	var confirmed map[string]json.RawMessage
	json.Unmarshal(detail["event"], &confirmed)
	if rootHTTPString(t, confirmed, "id") != eventID {
		t.Fatal("list cannot reach original controlled history")
	}
	next := rootHTTPData(t, f.call(t, "POST", approvedHTTPPath, `{"page":2,"pageSize":1,"queryVersion":"`+rootHTTPString(t, data, "queryVersion")+`"}`, nil), 200)
	if string(next["items"]) != "[]" || string(next["total"]) != "1" {
		t.Fatal("beyond-total history wrong")
	}
}
func TestRootPersonalApprovedHTTPSClosedBodyAndIdentity(t *testing.T) {
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
		t.Run(c.name, func(t *testing.T) {
			rootHTTPError(t, f.call(t, "POST", approvedHTTPPath, body, c.edit), c.status, c.code)
		})
	}
	for _, raw := range []string{"null", "[]", "{}", body + "{}", `{"page":1,"page":2}`, `{"page":1,"actorId":"` + f.actor + `"}`, `{"page":1,"filter":{}}`, `{"page":null}`, `{"page":0}`, `{"page":1.5}`, `{"page":1,"pageSize":null}`, `{"page":1,"pageSize":0}`, `{"page":1,"pageSize":101}`, `{"page":9007199254740991,"pageSize":100}`, `{"page":1,"queryVersion":null}`, strings.Repeat(" ", 4097) + body} {
		rootHTTPError(t, f.call(t, "POST", approvedHTTPPath, raw, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "POST", approvedHTTPPath+"?x=1", body, nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", approvedHTTPPath, body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		rootHTTPError(t, f.call(t, method, approvedHTTPPath, body, nil), 404, "API_NOT_FOUND")
	}
	rootHTTPError(t, f.call(t, "POST", approvedHTTPPath+"/", body, nil), 404, "API_NOT_FOUND")
}
