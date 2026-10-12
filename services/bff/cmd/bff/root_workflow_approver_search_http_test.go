package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// P16 real composition: discovery authority is not a grant of runtime record rights.
func TestRootWorkflowApproverSearchHostComposition(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/workflows/approvers/search"
	r := f.call(t, "POST", path, `{"page":1,"pageSize":100}`, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.actor) })
	out := rootHTTPData(t, r, 200)
	rootHTTPFieldSet(t, out, []string{"items", "total", "page", "pageSize", "queryVersion", "schemaVersion", "policyRevision"})
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(out["items"], &items); e != nil || items == nil {
		t.Fatalf("items %s %v", out["items"], e)
	}
	found := false
	for _, item := range items {
		rootHTTPFieldSet(t, item, []string{"id", "label", "status"})
		if rootHTTPString(t, item, "id") == f.actor {
			found = true
		}
		if rootHTTPString(t, item, "status") != "active" {
			t.Fatal("inactive candidate")
		}
	}
	if !found || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner candidate or no-store missing")
	}
	raw, _ := json.Marshal(map[string]any{"page": 100, "pageSize": 100, "queryVersion": rootHTTPString(t, out, "queryVersion")})
	later := rootHTTPData(t, f.call(t, "POST", path, string(raw), nil), 200)
	if string(later["items"]) != "[]" || string(later["total"]) != string(out["total"]) {
		t.Fatal("host query context lost")
	}
	rootHTTPError(t, f.call(t, "POST", path, `{"page":1,"pageSize":20}`, func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 403, "COMMON_CSRF_REJECTED")
}
