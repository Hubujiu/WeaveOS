package main

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"net/http"
	"strings"
	"testing"
)

func rootCopyHTTPBody(t *testing.T, v any) string {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(raw)
}
func rootCopyExecuteBody(t *testing.T, op string, req apptemplates.CopyPreflightInput) string {
	t.Helper()
	var m map[string]any
	if json.Unmarshal([]byte(rootCopyHTTPBody(t, req)), &m) != nil {
		t.Fatal("invalid test request")
	}
	m["operationId"] = op
	return rootCopyHTTPBody(t, m)
}
func TestRootStructureCopyHTTPPlanPreflightExecuteAndRecovery(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	in := rootCopyPlanInput(f)
	base := "/api/v1/structure-copies/"
	w := f.call(t, "POST", base+"plan", rootCopyHTTPBody(t, in), nil)
	data := rootHTTPData(t, w, 200)
	if len(data) != 5 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("plan envelope changed")
	}
	raw, _ := json.Marshal(data)
	var plan apptemplates.CopyPlan
	if json.Unmarshal(raw, &plan) != nil {
		t.Fatal("invalid plan")
	}
	req := rootCopyPreflightInput(plan, in)
	pre := f.call(t, "POST", base+"preflight", rootCopyHTTPBody(t, req), nil)
	result := rootHTTPData(t, pre, 200)
	if len(result) != 3 || string(result["valid"]) != "true" {
		t.Fatal("preflight shape wrong")
	}
	op := f.id(t)
	body := rootCopyExecuteBody(t, op, req)
	executed := f.call(t, "POST", base+"execute", body, nil)
	receipt := rootHTTPData(t, executed, 201)
	if len(receipt) != 7 || string(receipt["status"]) != `"copied"` || executed.Header().Get("Location") == "" {
		t.Fatal("execute receipt wrong")
	}
	again := rootHTTPData(t, f.call(t, "POST", base+"execute", body, nil), 201)
	a, _ := json.Marshal(receipt)
	b, _ := json.Marshal(again)
	if !rootTemplateJSONEqual(a, b) {
		t.Fatal("HTTP replay changed")
	}
	recovered := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-operations/"+op, "", nil), 200)
	if !rootTemplateJSONEqual(recovered["result"], a) {
		t.Fatal("recovery result changed")
	}
}
func TestRootStructureCopyHTTPBoundary(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	in := rootCopyPlanInput(f)
	body := rootCopyHTTPBody(t, in)
	path := "/api/v1/structure-copies/plan"
	for _, tc := range []struct {
		name, method, path, body, code string
		status                         int
		edit                           func(*http.Request)
	}{
		{"session", "POST", path, body, "AUTH_UNAUTHENTICATED", 401, func(r *http.Request) { r.Header.Del("Cookie") }},
		{"csrf", "POST", path, body, "COMMON_CSRF_REJECTED", 403, func(r *http.Request) { r.Header.Set("X-CSRF-Token", "wrong") }},
		{"actor", "POST", path, body, "AUTH_SESSION_CHANGED", 409, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
		{"bare-query", "POST", path + "?", body, "COMMON_INVALID_ARGUMENT", 400, nil},
		{"query", "POST", path + "?a=1", body, "COMMON_INVALID_ARGUMENT", 400, nil},
		{"method", "GET", path, "", "COMMON_METHOD_NOT_ALLOWED", 405, nil},
		{"unknown", "POST", "/api/v1/structure-copies/unknown", body, "API_NOT_FOUND", 404, nil},
		{"media", "POST", path, body, "COMMON_UNSUPPORTED_MEDIA_TYPE", 415, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }},
		{"gzip", "POST", path, body, "COMMON_UNSUPPORTED_MEDIA_TYPE", 415, func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }},
		{"extra", "POST", path, strings.TrimSuffix(body, "}") + `,"records":[]}`, "COMMON_INVALID_ARGUMENT", 400, nil},
		{"case", "POST", path, strings.Replace(body, `"source":`, `"Source":`, 1), "COMMON_INVALID_ARGUMENT", 400, nil},
		{"duplicate", "POST", path, `{"source":null,` + body[1:], "COMMON_INVALID_ARGUMENT", 400, nil},
		{"nested-extra", "POST", path, strings.Replace(body, `"kind":`, `"extra":true,"kind":`, 1), "COMMON_INVALID_ARGUMENT", 400, nil},
		{"trailing", "POST", path, body + `{}`, "COMMON_INVALID_ARGUMENT", 400, nil},
		{"utf8", "POST", path, string([]byte{255}) + body, "COMMON_INVALID_ARGUMENT", 400, nil},
		{"large", "POST", path, strings.Repeat(" ", 4*1024*1024+1), "APPLICATION_TEMPLATE_TOO_LARGE", 413, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call(t, tc.method, tc.path, tc.body, tc.edit)
			rootTemplateHTTPError(t, w, tc.status)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("wrong boundary", w.Body.String())
			}
		})
	}
}
