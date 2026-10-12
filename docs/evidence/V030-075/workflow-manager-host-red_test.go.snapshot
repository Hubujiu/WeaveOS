package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Real buildHandler + Session + restricted storage; deployment itself is a fixture.
func TestRootWorkflowManagerSearchHostComposition(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/workflows/search"
	r := f.call(t, "POST", path, `{"page":1,"pageSize":20}`, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.actor) })
	out := rootHTTPData(t, r, 200)
	rootHTTPFieldSet(t, out, []string{"items", "total", "page", "pageSize", "queryVersion"})
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(out["items"], &items); e != nil || len(items) != 1 {
		t.Fatalf("items %s %v", out["items"], e)
	}
	rootHTTPFieldSet(t, items[0], []string{"id", "name", "revision", "currentVersion", "candidateVersion", "state", "triggerEvents", "inFlightCount", "updatedAt"})
	if rootHTTPString(t, items[0], "id") != f.flow || string(items[0]["triggerEvents"]) != `["manual"]` || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("host summary mismatch")
	}
	token := rootHTTPString(t, out, "queryVersion")
	raw, _ := json.Marshal(map[string]any{"page": 2, "pageSize": 20, "queryVersion": token})
	second := rootHTTPData(t, f.call(t, "POST", path, string(raw), nil), 200)
	if string(second["items"]) != "[]" || string(second["total"]) != "1" || rootHTTPString(t, second, "queryVersion") != token {
		t.Fatal("host query store not wired")
	}
	rootHTTPError(t, f.call(t, "POST", path, `{"page":1,"pageSize":20}`, func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 403, "COMMON_CSRF_REJECTED")
}
