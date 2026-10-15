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
		rootHTTPFieldSet(t, item, []string{"id", "label", "status", "departmentNames"})
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

func TestRootWorkflowApproverTextSearchThroughRealSessionHost(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/workflows/approvers/search"
	var account string
	if e := f.owner.QueryRow(f.ctx, "SELECT account FROM auth.users WHERE id=$1", f.actor).Scan(&account); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"page": 1, "pageSize": 100, "q": account})
	out := rootHTTPData(t, f.call(t, "POST", path, string(raw), nil), 200)
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(out["items"], &items); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, item := range items {
		rootHTTPFieldSet(t, item, []string{"id", "label", "status", "departmentNames"})
		if rootHTTPString(t, item, "id") == f.actor {
			found = true
		}
	}
	if !found {
		t.Fatal("host did not expose eligible matching owner")
	}
	rootHTTPError(t, f.call(t, "POST", path, string(raw), func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 403, "COMMON_CSRF_REJECTED")
}

func TestRootWorkflowApproverDepartmentNamesThroughRealSession(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	var dept string
	name := "部门显示-" + f.actor
	if e := f.owner.QueryRow(f.ctx, "INSERT INTO personnel.departments(parent_id,name) SELECT id,$1 FROM personnel.departments WHERE is_root RETURNING id::text", name).Scan(&dept); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.department_members WHERE department_id=$1", dept); e != nil {
			t.Error(e)
		}
		if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.departments WHERE id=$1", dept); e != nil {
			t.Error(e)
		}
	})
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO personnel.department_members(department_id,user_id) VALUES($1,$2)", dept, f.actor); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"page": 1, "pageSize": 20, "q": name})
	out := rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/workflows/approvers/search", string(raw), nil), 200)
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(out["items"], &items); e != nil || len(items) != 1 {
		t.Fatalf("items %s %v", out["items"], e)
	}
	rootHTTPFieldSet(t, items[0], []string{"id", "label", "status", "departmentNames"})
	var names []string
	if e := json.Unmarshal(items[0]["departmentNames"], &names); e != nil || len(names) != 1 || names[0] != name || rootHTTPString(t, items[0], "id") != f.actor {
		t.Fatalf("actual department projection %+v %v", names, e)
	}
}
