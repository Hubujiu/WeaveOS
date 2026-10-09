package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestRootManualOptionsHTTPSDiscoverThenStart(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := f.root() + "/workflow-start-options/search"
	response := f.call(t, "POST", path, `{"page":1}`, nil)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("options cached")
	}
	got := rootHTTPData(t, response, 200)
	rootHTTPFieldSet(t, got, []string{"items", "total", "page", "pageSize", "queryVersion"})
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(got["items"], &items); e != nil || len(items) != 1 {
		t.Fatalf("options %s %v", got["items"], e)
	}
	item := items[0]
	rootHTTPFieldSet(t, item, []string{"flowId", "name", "workflowRevision", "definitionVersion", "schemaVersion", "recordVersion"})
	if rootHTTPString(t, item, "flowId") != f.flow || string(got["total"]) != "1" || string(got["pageSize"]) != "20" {
		t.Fatal("wrong option")
	}
	body := fmt.Sprintf(`{"operationId":%q,"flowId":%s,"expectedWorkflowRevision":%s,"expectedSchemaVersion":%s,"expectedRecordVersion":%s}`, f.id(t), item["flowId"], item["workflowRevision"], item["schemaVersion"], item["recordVersion"])
	rootHTTPData(t, f.call(t, "POST", f.root()+"/workflow-starts", body, nil), 202)
	old := fmt.Sprintf(`{"page":1,"queryVersion":%q}`, rootHTTPString(t, got, "queryVersion"))
	rootHTTPError(t, f.call(t, "POST", path, old, nil), 409, "APPLICATION_QUERY_CHANGED")
	fresh := rootHTTPData(t, f.call(t, "POST", path, `{"page":1}`, nil), 200)
	if string(fresh["items"]) != "[]" || string(fresh["total"]) != "0" {
		t.Fatal("started flow still first-start option")
	}
}
func TestRootManualOptionsHTTPSClosedBodyAndIngress(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := f.root() + "/workflow-start-options/search"
	rootHTTPData(t, f.call(t, "POST", path, `{"page":1}`, nil), 200)
	for _, raw := range []string{`{}`, `null`, `{"page":0}`, `{"page":1.5}`, `{"page":1,"pageSize":101}`, `{"page":1,"pageSize":0}`, `{"page":1,"pageSize":null}`, `{"page":1,"queryVersion":null}`, `{"page":1,"page":2}`, `{"page":1,"actorId":"x"}`, `{"page":1,"flowId":"x"}`, `{"page":1} {}`, strings.Repeat(" ", 4097) + `{"page":1}`} {
		rootHTTPError(t, f.call(t, "POST", path, raw, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "POST", path+"?page=1", `{"page":1}`, nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", path, `{"page":1}`, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	rootHTTPError(t, f.call(t, "GET", path, "", nil), 404, "API_NOT_FOUND")
	for _, c := range []struct {
		code int
		name string
		edit func(*http.Request)
	}{
		{401, "AUTH_UNAUTHENTICATED", func(r *http.Request) { r.Header.Del("Cookie") }},
		{403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }},
		{403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Set("Origin", "https://foreign.invalid") }},
		{409, "AUTH_SESSION_CHANGED", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
	} {
		rootHTTPError(t, f.call(t, "POST", path, `{"page":1}`, c.edit), c.code, c.name)
	}
	rootHTTPTriggerCount(t, f, f.record, 0)
}
