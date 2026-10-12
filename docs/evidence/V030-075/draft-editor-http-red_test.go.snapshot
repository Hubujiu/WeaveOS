package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func rootDraftEditorHTTP(t *testing.T) (*rootTaskHTTPFixture, string, string) {
	t.Helper()
	f := rootHTTPResourceSetup(t)
	base := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/drafts"
	body := `{"operationId":"` + f.id(t) + `","targetRecordId":"` + f.record + `","schemaVersion":1,"baseRecordVersion":1,"values":{"` + f.field + `":"unfinished"}}`
	d := rootHTTPData(t, f.call(t, "POST", base, body, nil), 201)
	draft := rootHTTPString(t, d, "id")
	return f, base + "/" + draft + "/editor-context", draft
}
func TestRootDraftEditorHTTPClosedProjectionAndNoStore(t *testing.T) {
	f, path, id := rootDraftEditorHTTP(t)
	data := rootHTTPData(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.actor) }), 200)
	rootHTTPFieldSet(t, data, []string{"draft", "appId", "schemaVersion", "viewVersion", "policyRevision", "recordVersion", "editableFieldIds", "canCommit"})
	if rootHTTPString(t, data, "appId") != f.app {
		t.Fatal("wrong app")
	}
	var draft map[string]json.RawMessage
	if json.Unmarshal(data["draft"], &draft) != nil || rootHTTPString(t, draft, "id") != id {
		t.Fatal("wrong draft")
	}
	rootHTTPFieldSet(t, draft, []string{"id", "viewId", "tableId", "targetRecordId", "schemaVersion", "baseRecordVersion", "draftVersion", "createdAt", "updatedAt", "hasConflicts", "values", "conflicts"})
	var fields []string
	if json.Unmarshal(data["editableFieldIds"], &fields) != nil || len(fields) != 1 || fields[0] != f.field {
		t.Fatal("current mask missing")
	}
	var commit bool
	if json.Unmarshal(data["canCommit"], &commit) != nil || !commit {
		t.Fatal("safe commit missing")
	}
}
func TestRootDraftEditorHTTPRejectsQueryAndMutatingMethods(t *testing.T) {
	f, path, _ := rootDraftEditorHTTP(t)
	for _, query := range []string{"?recordId=" + f.record, "?actorId=" + f.actor, "?fieldId=" + f.field, "?pageSize=20", "?x=1&x=2"} {
		t.Run(query, func(t *testing.T) {
			rootHTTPError(t, f.call(t, "GET", path+query, "", nil), 400, "COMMON_VALIDATION_FAILED")
		})
	}
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) { rootHTTPError(t, f.call(t, method, path, `{}`, nil), 404, "API_NOT_FOUND") })
	}
}
func TestRootDraftEditorHTTPRequiresLiveSessionAndExpectedActor(t *testing.T) {
	f, path, _ := rootDraftEditorHTTP(t)
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409, "AUTH_SESSION_CHANGED")
}
