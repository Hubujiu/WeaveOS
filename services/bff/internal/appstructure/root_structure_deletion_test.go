package appstructure

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Independent expectations: V030-073 PRD/ADR/Data, frozen before this test.
func rootDeletionInput(t *testing.T, f *fixture, structure, resource int64) map[string]any {
	return map[string]any{"operationId": uuid(t, f.owner), "expectedStructureVersion": structure, "expectedResourceVersion": resource}
}
func rootDeletionError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	var env struct {
		Code string
		Data map[string]any
	}
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &env) != nil || env.Code != code {
		t.Fatalf("want %d/%s, got %d %s", status, code, w.Code, w.Body.String())
	}
	return env.Data
}
func rootDeletionReceipt(t *testing.T, w *httptest.ResponseRecorder, op, app, kind, id string, version int64) map[string]any {
	t.Helper()
	d := data(t, w, 200)
	expected := map[string]any{"operationId": op, "appId": app, "resourceKind": kind, "id": id, "structureVersion": float64(version), "deleted": true}
	if !reflect.DeepEqual(d, expected) || w.Header().Get("Location") != "" {
		t.Fatalf("closed six-key minimum receipt: %#v", d)
	}
	return d
}
func rootDeletionDirectory(t *testing.T, f *fixture, parent any, version int64) string {
	t.Helper()
	d := data(t, f.call(t, "POST", "/directories", map[string]any{"operationId": uuid(t, f.owner), "name": "directory", "parentId": parent, "position": 0, "expectedStructureVersion": version}), 201)
	return d["directory"].(map[string]any)["id"].(string)
}
func rootDeletionForm(t *testing.T, f *fixture, source map[string]any, version int64) (string, string) {
	t.Helper()
	d := data(t, f.call(t, "POST", "/forms", map[string]any{"operationId": uuid(t, f.owner), "name": "view", "source": source, "directoryId": nil, "position": 0, "expectedStructureVersion": version}), 201)
	return d["form"].(map[string]any)["id"].(string), d["table"].(map[string]any)["id"].(string)
}
func TestRootStructureDeletionEmptyApplication(t *testing.T) {
	f := setup(t)
	in := rootDeletionInput(t, f, 0, 1)
	rootDeletionReceipt(t, f.call(t, "POST", "/deletion", in), in["operationId"].(string), f.app, "application", f.app, 1)
	rootDeletionReceipt(t, f.call(t, "POST", "/deletion", in), in["operationId"].(string), f.app, "application", f.app, 1)
	rootDeletionError(t, f.call(t, "GET", "/structure", nil), 404, "APPLICATION_NOT_FOUND")
	rootDeletionError(t, f.call(t, "POST", "/deletion", rootDeletionInput(t, f, 1, 2)), 404, "APPLICATION_NOT_FOUND")
	var retained, disabled bool
	var sv, pv, events int64
	e := f.owner.QueryRow(context.Background(), `SELECT a.deleted_at IS NOT NULL,a.structure_version,a.policy_revision,NOT pc.enabled,(SELECT count(*) FROM applications.structure_deletions WHERE app_id=a.id) FROM applications.apps a JOIN personnel.permission_catalog pc ON pc.code='app.'||a.id::text||'.access' WHERE a.id=$1`, f.app).Scan(&retained, &sv, &pv, &disabled, &events)
	if e != nil || !retained || !disabled || sv != 1 || pv != 2 || events != 1 {
		t.Fatalf("retained identity/atomic versions/event %v %v %d %d %d %v", retained, disabled, sv, pv, events, e)
	}
}
func TestRootStructureDeletionNonCascadeDirectory(t *testing.T) {
	f := setup(t)
	parent := rootDeletionDirectory(t, f, nil, 0)
	child := rootDeletionDirectory(t, f, parent, 1)
	d := rootDeletionError(t, f.call(t, "POST", "/directories/"+parent+"/deletion", rootDeletionInput(t, f, 2, 0)), 409, "APPLICATION_STRUCTURE_NOT_EMPTY")
	if !reflect.DeepEqual(d, map[string]any{"dependencies": []any{"directories"}}) {
		t.Fatalf("closed dependencies: %#v", d)
	}
	data(t, f.call(t, "GET", "/directories/"+child, nil), 200)
	in := rootDeletionInput(t, f, 2, 0)
	rootDeletionReceipt(t, f.call(t, "POST", "/directories/"+child+"/deletion", in), in["operationId"].(string), f.app, "directory", child, 3)
	in = rootDeletionInput(t, f, 3, 0)
	rootDeletionReceipt(t, f.call(t, "POST", "/directories/"+parent+"/deletion", in), in["operationId"].(string), f.app, "directory", parent, 4)
	rootDeletionError(t, f.call(t, "GET", "/directories/"+child, nil), 404, "APPLICATION_NOT_FOUND")
}
func TestRootStructureDeletionSharedViewIdentity(t *testing.T) {
	f := setup(t)
	first, table := rootDeletionForm(t, f, map[string]any{"kind": "new_table"}, 0)
	second, _ := rootDeletionForm(t, f, map[string]any{"kind": "existing_table", "tableId": table}, 1)
	rootDeletionError(t, f.call(t, "POST", "/tables/"+table+"/deletion", rootDeletionInput(t, f, 2, 0)), 409, "APPLICATION_STRUCTURE_NOT_EMPTY")
	in := rootDeletionInput(t, f, 2, 0)
	rootDeletionReceipt(t, f.call(t, "POST", "/forms/"+first+"/deletion", in), in["operationId"].(string), f.app, "form", first, 3)
	rootDeletionError(t, f.call(t, "GET", "/forms/"+first+"/definition", nil), 404, "APPLICATION_NOT_FOUND")
	data(t, f.call(t, "GET", "/forms/"+second+"/definition", nil), 200)
	var active, retained bool
	var version int64
	e := f.owner.QueryRow(context.Background(), `SELECT t.deleted_at IS NULL,f.deleted_at IS NOT NULL,t.schema_version FROM applications.logical_tables t JOIN applications.form_views f ON f.table_id=t.id WHERE f.id=$1`, first).Scan(&active, &retained, &version)
	if e != nil || !active || !retained || version != 0 {
		t.Fatalf("view deletion must retain untouched shared table %v %v %d %v", active, retained, version, e)
	}
}
func TestRootStructureDeletionApplicationRejectsCustomGroup(t *testing.T) {
	f := setup(t)
	var id string
	e := f.owner.QueryRow(context.Background(), `INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'retained-group') RETURNING id::text`, f.app).Scan(&id)
	if e != nil {
		t.Fatal(e)
	}
	d := rootDeletionError(t, f.call(t, "POST", "/deletion", rootDeletionInput(t, f, 0, 1)), 409, "APPLICATION_STRUCTURE_NOT_EMPTY")
	if !reflect.DeepEqual(d, map[string]any{"dependencies": []any{"permission_groups"}}) {
		t.Fatalf("group not implicitly removed %#v", d)
	}
}
