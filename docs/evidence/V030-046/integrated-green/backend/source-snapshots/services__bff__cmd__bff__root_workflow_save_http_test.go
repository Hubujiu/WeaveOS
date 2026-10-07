package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func rootHTTPSaveBody(op, token, field, value string) string {
	raw, _ := json.Marshal(map[string]any{"operationId": op, "basisToken": token, "changes": map[string]any{field: value}})
	return string(raw)
}
func TestRootWorkflowSaveHTTPRealHostSaveAndGenericReceipt(t *testing.T) {
	f := rootHTTPTaskSetupEditable(t, true)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	var editable []string
	if json.Unmarshal(preview["editableFieldIds"], &editable) != nil || len(editable) != 1 || editable[0] != f.field {
		t.Fatal("preview did not expose exact writable intersection")
	}
	op := f.id(t)
	body := rootHTTPSaveBody(op, token, f.field, "saved by node")
	saved := rootHTTPData(t, f.call(t, "PATCH", f.taskPath()+"/record", body, nil), 200)
	if len(saved) != 6 || rootHTTPString(t, saved, "id") != f.record || rootHTTPString(t, saved, "operationId") != op || string(saved["recordVersion"]) != "2" {
		t.Fatal("save response not original record result")
	}
	repeated := rootHTTPData(t, f.call(t, "PATCH", f.taskPath()+"/record", body, nil), 200)
	if string(repeated["recordVersion"]) != "2" {
		t.Fatal("duplicate HTTP save wrote twice")
	}
	result := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-operations/"+op, "", nil), 200)
	if string(result["httpStatus"]) != "200" {
		t.Fatal("generic operation did not retain confirmed local save")
	}
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1", f.app).Scan(&n); e != nil || n != 1 {
		t.Fatal("HTTP Save advanced engine", e)
	}
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", token), nil), 409, "WORKFLOW_BASIS_CHANGED")
}
func TestRootWorkflowSaveHTTPPreservesSessionCSRFAndActorBoundaries(t *testing.T) {
	f := rootHTTPTaskSetupEditable(t, true)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	for _, c := range []struct {
		name   string
		status int
		code   string
		edit   func(*http.Request)
	}{
		{"session", 401, "AUTH_UNAUTHENTICATED", func(r *http.Request) { r.Header.Del("Cookie") }},
		{"csrf", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Set("X-CSRF-Token", "wrong") }},
		{"origin", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Set("Origin", "https://foreign.invalid") }},
		{"actor", 409, "AUTH_SESSION_CHANGED", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			rootHTTPError(t, f.call(t, "PATCH", f.taskPath()+"/record", rootHTTPSaveBody(f.id(t), token, f.field, "no"), c.edit), c.status, c.code)
		})
	}
}
func TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges(t *testing.T) {
	f := rootHTTPTaskSetupEditable(t, true)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	op := f.id(t)
	valid := rootHTTPSaveBody(op, token, f.field, "no")
	for _, c := range []struct{ name, body string }{
		{"unknown", strings.TrimSuffix(valid, "}") + `,"actorId":"` + f.actor + `"}`},
		{"duplicate-operation", strings.TrimSuffix(valid, "}") + `,"operationId":"` + op + `"}`},
		{"empty", `{"operationId":"` + op + `","basisToken":"` + token + `","changes":{}}`},
		{"null", `{"operationId":"` + op + `","basisToken":"` + token + `","changes":null}`},
		{"number", `{"operationId":"` + op + `","basisToken":"` + token + `","changes":{"` + f.field + `":123}}`},
		{"duplicate-field", `{"operationId":"` + op + `","basisToken":"` + token + `","changes":{"` + f.field + `":"a","` + f.field + `":"b"}}`},
		{"extra-query", valid},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := f.taskPath() + "/record"
			if c.name == "extra-query" {
				path += "?unused=1"
			}
			rootHTTPError(t, f.call(t, "PATCH", path, c.body, nil), 400, "COMMON_VALIDATION_FAILED")
		})
	}
}
func TestRootWorkflowSaveHTTPReadonlyApprovalNodeDenied(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	rootHTTPError(t, f.call(t, "PATCH", f.taskPath()+"/record", rootHTTPSaveBody(f.id(t), token, f.field, "no"), nil), 403, "APPLICATION_FORBIDDEN")
}
