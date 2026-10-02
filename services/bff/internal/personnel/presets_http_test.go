package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

// Oracles: PRD FM-AC-04/08, ADR008 A1–A3/A7; real Session/PG18/Redis,
// actual Service dispatch, SET ROLE auth_app; no application/store mocks.
const presetBase = "/api/v1/personnel/table-presets"

func presetsWeb(t *testing.T) *webFixture {
	t.Helper()
	f := setupWeb(t)
	t.Cleanup(func() {
		var exists bool
		if e := f.owner.QueryRow(context.Background(), `SELECT to_regclass('personnel.table_presets') IS NOT NULL`).Scan(&exists); e != nil {
			t.Error(e)
		} else if exists {
			if _, e = f.owner.Exec(context.Background(), `DELETE FROM personnel.table_presets WHERE owner_id=$1`, f.actor.UserID); e != nil {
				t.Error(e)
			}
		}
	})
	return f
}
func presetBody(view, name, filter string, hidden []string) string {
	if filter == "" {
		filter = "null"
	}
	ids, _ := json.Marshal(hidden)
	if hidden == nil {
		ids = []byte("[]")
	}
	n, _ := json.Marshal(name)
	return fmt.Sprintf(`{"view":%q,"name":%s,"filter":%s,"hiddenColumnIds":%s,"schemaVersion":1}`, view, n, filter, ids)
}
func presetID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if w.Code != 201 {
		t.Fatalf("approved explicit preset save must return 201, got %d (%s)", w.Code, w.Body.String())
	}
	var id string
	data := envelopeData(t, w)
	if e := json.Unmarshal(data["id"], &id); e != nil || !validID(id) {
		t.Fatal("preset UUID missing")
	}
	return id
}
func presetUpdate(name string, version int) string {
	n, _ := json.Marshal(name)
	return fmt.Sprintf(`{"version":%d,"name":%s,"filter":null,"hiddenColumnIds":[],"schemaVersion":1}`, version, n)
}
func TestPresetLifecycleIsolationAndVersion(t *testing.T) {
	f := presetsWeb(t)
	other := presetsWeb(t)
	id := presetID(t, f.request("POST", presetBase, presetBody("members", "  命名😀  ", "", []string{"identities"}), true, true))
	w := f.request("GET", presetBase+"/"+id, "", true, false)
	if w.Code != 200 {
		t.Fatal("saved preset GET", w.Code)
	}
	d := envelopeData(t, w)
	if string(d["name"]) != `"命名😀"` || string(d["version"]) != "1" || string(d["view"]) != `"members"` || string(d["filter"]) != "null" {
		t.Fatal("trimmed persisted configuration and initial CAS metadata", w.Body.String())
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		path := presetBase + "/" + id
		if method == "DELETE" {
			path += "?version=1"
		}
		if w := other.request(method, path, presetUpdate("stolen", 1), true, true); w.Code != 404 {
			t.Fatalf("foreign %s must be indistinguishable from missing: %d", method, w.Code)
		}
	}
	for _, view := range []string{"events", "members"} {
		w := other.request("GET", presetBase+"?view="+view, "", true, false)
		if w.Code != 200 || string(envelopeData(t, w)["items"]) != "[]" {
			t.Fatal("owner/view list isolation", w.Code)
		}
	}
	if w := f.request("GET", presetBase+"?view=events", "", true, false); w.Code != 200 || string(envelopeData(t, w)["items"]) != "[]" {
		t.Fatal("view isolation")
	}
	if w := f.request("PUT", presetBase+"/"+id, presetUpdate("新保存", 1), true, true); w.Code != 200 || string(envelopeData(t, w)["version"]) != "2" {
		t.Fatal("CAS update", w.Code)
	}
	for _, method := range []string{"PUT", "DELETE"} {
		path := presetBase + "/" + id
		if method == "DELETE" {
			path += "?version=1"
		}
		if w := f.request(method, path, presetUpdate("stale", 1), true, true); w.Code != 409 || !strings.Contains(w.Body.String(), "PERSONNEL_PRESET_CONFLICT") {
			t.Fatal("stale mutation must reject", method, w.Code)
		}
	}
	if w := f.request("DELETE", presetBase+"/"+id+"?version=2", "", true, true); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("successful deletion must have no body", w.Code)
	}
}
func TestPresetSessionCSRFAndLiveRevocation(t *testing.T) {
	f := presetsWeb(t)
	body := presetBody("members", "security", "", nil)
	if w := f.request("GET", presetBase+"?view=members", "", false, false); w.Code != 401 {
		t.Fatal("anonymous list")
	}
	if w := f.request("POST", presetBase, body, true, false); w.Code != 403 {
		t.Fatal("CSRF required")
	}
	id := presetID(t, f.request("POST", presetBase, body, true, true))
	if _, e := f.owner.Exec(context.Background(), `DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'`, f.template); e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		path := presetBase + "/" + id
		if method == "DELETE" {
			path += "?version=1"
		}
		if w := f.request(method, path, presetUpdate("denied", 1), true, true); w.Code != 403 {
			t.Fatal("revoked manager denied", method, w.Code)
		}
	}
	if w := f.request("GET", presetBase+"?view=members", "", true, false); w.Code != 403 {
		t.Fatal("revoked list denied")
	}
}
func TestPresetNameCapacityAndCASConcurrency(t *testing.T) {
	f := presetsWeb(t)
	presetID(t, f.request("POST", presetBase, presetBody("members", "Same", "", nil), true, true))
	if w := f.request("POST", presetBase, presetBody("members", "  Same ", "", nil), true, true); w.Code != 409 || !strings.Contains(w.Body.String(), "PERSONNEL_PRESET_NAME_CONFLICT") {
		t.Fatal("trimmed exact-name conflict", w.Code)
	}
	presetID(t, f.request("POST", presetBase, presetBody("members", "same", "", nil), true, true))
	presetID(t, f.request("POST", presetBase, presetBody("events", "Same", "", nil), true, true))
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 40)
	for i := range 40 {
		go func(i int) {
			<-start
			results <- f.request("POST", presetBase, presetBody("members", fmt.Sprint("concurrent-", i), "", nil), true, true)
		}(i)
	}
	close(start)
	success, limit := 0, 0
	for range 40 {
		w := <-results
		if w.Code == 201 {
			success++
		} else if w.Code == 409 && strings.Contains(w.Body.String(), "PERSONNEL_PRESET_LIMIT_REACHED") {
			limit++
		} else {
			t.Fatal("hard slot quota must produce independent domain failure", w.Code, w.Body.String())
		}
	}
	if success != 18 || limit != 22 {
		t.Fatalf("20/owner/view capacity with two preexisting: %d %d", success, limit)
	}
	w := f.request("GET", presetBase+"?view=members", "", true, false)
	var items []struct {
		ID string `json:"id"`
	}
	if w.Code != 200 || json.Unmarshal(envelopeData(t, w)["items"], &items) != nil || len(items) != 20 {
		t.Fatal("full owner/view list")
	}
	id := items[0].ID
	start = make(chan struct{})
	results = make(chan *httptest.ResponseRecorder, 2)
	for _, name := range []string{"tab-a", "tab-b"} {
		go func(n string) {
			<-start
			results <- f.request("PUT", presetBase+"/"+id, presetUpdate(n, 1), true, true)
		}(name)
	}
	close(start)
	success, conflict := 0, 0
	for range 2 {
		w := <-results
		if w.Code == 200 {
			success++
		} else if w.Code == 409 && strings.Contains(w.Body.String(), "PERSONNEL_PRESET_CONFLICT") {
			conflict++
		} else {
			t.Fatal("CAS schedule", w.Code)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("one CAS winner", success, conflict)
	}
}
func TestPresetUnknownFieldsNamesAndDisplayWhitelist(t *testing.T) {
	f := presetsWeb(t)
	for _, body := range []string{
		presetBody("unknown", "x", "", nil), presetBody("members", " \t\n", "", nil), presetBody("members", strings.Repeat("😀", 101), "", nil),
		presetBody("members", "x", "", []string{"departmentIds"}), presetBody("events", "x", "", []string{"account"}), presetBody("members", "x", "", []string{"account", "account"}), presetBody("members", "x", "", []string{"account", "departments", "identities", "personnelManage"}),
		strings.Replace(presetBody("members", "x", "", nil), `"schemaVersion":1`, `"schemaVersion":2`, 1), strings.Replace(presetBody("members", "x", "", nil), `"hiddenColumnIds":[]`, `"hiddenColumnIds":null`, 1),
		strings.Replace(presetBody("members", "x", "", nil), `"schemaVersion":1`, `"schemaVersion":1,"ownerId":"x"`, 1), strings.Replace(presetBody("members", "x", "", nil), `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1),
	} {
		if w := f.request("POST", presetBase, body, true, true); w.Code != 400 {
			t.Fatalf("closed DTO/Unicode/column invariants must reject: %d %s", w.Code, body)
		}
	}
	presetID(t, f.request("POST", presetBase, presetBody("members", strings.Repeat("😀", 100), "", nil), true, true))
	for _, path := range []string{presetBase, presetBase + "?view=members&view=events", presetBase + "?view=members&ownerId=x", presetBase + "/bad-id"} {
		if w := f.request("GET", path, "", true, false); w.Code != 400 {
			t.Fatal("strict query", path, w.Code)
		}
	}
}
func TestPresetEditableASTAndIndependentUTF8RawBoundaries(t *testing.T) {
	f := presetsWeb(t)
	leaf := `{"field":"account","operator":"eq","value":"A"}`
	and := `{"operator":"and","children":[` + leaf + `]}`
	for i, filter := range []string{and, `{"operator":"or","children":[` + and + `,` + and + `]}`} {
		presetID(t, f.request("POST", presetBase, presetBody("members", fmt.Sprint("valid", i), filter, nil), true, true))
	}
	for _, filter := range []string{`{"operator":"or","children":[` + leaf + `]}`, `{"operator":"and","children":[]}`, `{"operator":"and","children":[` + and + `]}`, `{"operator":"and","children":[{"field":"identities","operator":"eq","value":"x"}]}`, `{"operator":"and","children":[{"field":"account","operator":"gt","value":"x"}]}`} {
		if w := f.request("POST", presetBase, presetBody("members", "invalid", filter, nil), true, true); w.Code != 400 {
			t.Fatal("uneditable/invalid AST must preserve query meaning by rejecting", w.Code)
		}
	}
	// Independent canonical byte oracle: overhead counted from a fixed literal,
	// multibyte values ensure this is UTF-8 byte counting, not rune/UTF16 length.
	base := `{"children":[{"field":"account","operator":"eq","value":""}],"operator":"and"}`
	value := strings.Repeat("界", (16384-len(base))/3) + strings.Repeat("a", (16384-len(base))%3)
	exact := strings.Replace(base, `"value":""`, `"value":"`+value+`"`, 1)
	if len(exact) != 16384 {
		t.Fatal("independent boundary fixture")
	}
	presetID(t, f.request("POST", presetBase, presetBody("members", "filter exact", exact, nil), true, true))
	over := strings.Replace(exact, `"value":"`, `"value":"a`, 1)
	if w := f.request("POST", presetBase, presetBody("members", "filter over", over, nil), true, true); w.Code != 400 {
		t.Fatal("canonical filter +1 byte must reject", w.Code)
	}
	body := presetBody("members", "raw exact", "", nil)
	padded := body + strings.Repeat(" ", 65536-len(body))
	presetID(t, f.request("POST", presetBase, padded, true, true))
	if w := f.request("POST", presetBase, padded+" ", true, true); w.Code != 400 {
		t.Fatal("raw +1 must reject even with tiny canonical configuration", w.Code)
	}
}
func TestPresetCRUDNoBusinessRevisionOrAudit(t *testing.T) {
	f := presetsWeb(t)
	ctx := context.Background()
	var before, after string
	const observation = `SELECT (SELECT string_agg(scope||':'||revision,',' ORDER BY scope) FROM personnel.query_revisions)||':'||(SELECT count(*)::text FROM auth.authentication_events)`
	if e := f.owner.QueryRow(ctx, observation).Scan(&before); e != nil {
		t.Fatal(e)
	}
	id := presetID(t, f.request("POST", presetBase, presetBody("members", "pure configuration", "", nil), true, true))
	if w := f.request("PUT", presetBase+"/"+id, presetUpdate("changed", 1), true, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := f.request("DELETE", presetBase+"/"+id+"?version=2", "", true, true); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if e := f.owner.QueryRow(ctx, observation).Scan(&after); e != nil || after != before {
		t.Fatal("preset CRUD must leave all business revision/audit data unchanged", before, after, e)
	}
}

func TestPresetDatabaseLeastPrivilegeAndHardConstraints(t *testing.T) {
	f := presetsWeb(t)
	ctx := context.Background()
	var present bool
	if e := f.owner.QueryRow(ctx, `SELECT to_regclass('personnel.table_presets') IS NOT NULL`).Scan(&present); e != nil || !present {
		t.Fatal("approved independent preset table must exist", e)
	}
	for _, priv := range []string{"SELECT", "INSERT", "DELETE"} {
		var allowed bool
		if e := f.owner.QueryRow(ctx, `SELECT has_table_privilege('auth_app','personnel.table_presets',$1)`, priv).Scan(&allowed); e != nil || !allowed {
			t.Fatal("preset runtime capability", priv, e)
		}
	}
	for _, col := range []string{"id", "owner_id", "view_key", "slot", "created_at"} {
		var allowed bool
		if e := f.owner.QueryRow(ctx, `SELECT has_column_privilege('auth_app','personnel.table_presets',$1,'UPDATE')`, col).Scan(&allowed); e != nil || allowed {
			t.Fatal("immutable preset ownership/context column", col, e)
		}
	}
	for _, col := range []string{"name", "filter_json", "hidden_column_ids", "schema_version", "version", "updated_at"} {
		var allowed bool
		if e := f.owner.QueryRow(ctx, `SELECT has_column_privilege('auth_app','personnel.table_presets',$1,'UPDATE')`, col).Scan(&allowed); e != nil || !allowed {
			t.Fatal("mutable preset content column", col, e)
		}
	}
	for _, sql := range []string{
		`INSERT INTO personnel.table_presets(owner_id,view_key,name,slot,hidden_column_ids) VALUES($1,'members','bad-slot',21,'[]')`,
		`INSERT INTO personnel.table_presets(owner_id,view_key,name,slot,hidden_column_ids) VALUES($1,'unknown','bad-view',1,'[]')`,
		`INSERT INTO personnel.table_presets(owner_id,view_key,name,slot,hidden_column_ids) VALUES($1,'members','bad-hidden',1,'{}')`,
	} {
		if _, e := f.owner.Exec(ctx, sql, f.actor.UserID); e == nil {
			t.Fatal("database hard constraints must independently reject malformed rows", sql)
		}
	}
	var backup bool
	if e := f.owner.QueryRow(ctx, `SELECT has_table_privilege('auth_backup','personnel.table_presets','SELECT')`).Scan(&backup); e != nil || !backup {
		t.Fatal("backup includes new table", e)
	}
}
