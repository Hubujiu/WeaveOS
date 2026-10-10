package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func rootLifecycleBody(op string, version string) string {
	return `{"operationId":"` + op + `","expectedSchemaVersion":1,"expectedRecordVersion":` + version + `}`
}
func TestRootRecordLifecycleHTTPDeleteRestore(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record
	op := f.id(t)
	body := rootLifecycleBody(op, "1")
	w := f.call(t, "POST", path+"/deletion", body, nil)
	got := rootHTTPData(t, w, 200)
	if len(got) != 5 || string(got["deleted"]) != "true" || string(got["recordVersion"]) != "2" || rootHTTPString(t, got, "operationId") != op {
		t.Fatal("incorrect closed delete receipt", w.Body.String())
	}
	rootHTTPData(t, f.call(t, "POST", path+"/deletion", body, nil), 200)
	rootHTTPData(t, f.call(t, "GET", path, "", nil), 404)
	state := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
	if len(state) != 4 || string(state["deleted"]) != "true" {
		t.Fatal("incorrect bounded state", state)
	}
	restored := rootHTTPData(t, f.call(t, "POST", path+"/restoration", rootLifecycleBody(f.id(t), "2"), nil), 200)
	if string(restored["deleted"]) != "false" || string(restored["recordVersion"]) != "3" {
		t.Fatal("incorrect restore", restored)
	}
	rootHTTPData(t, f.call(t, "GET", path, "", nil), 200)
}
func TestRootRecordLifecycleHTTPClosedInputAndIdentity(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record + "/deletion"
	base := rootLifecycleBody(f.id(t), "1")
	for _, tc := range []struct {
		name, body, suffix string
		edit               func(*http.Request)
		status             int
		code               string
	}{
		{"unknown", strings.Replace(base, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":1,"deleted":false`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"duplicate", strings.Replace(base, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":0,"expectedSchemaVersion":1`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"missing", strings.Replace(base, `,"expectedRecordVersion":1`, "", 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"null", strings.Replace(base, `"expectedRecordVersion":1`, `"expectedRecordVersion":null`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"query", base, "?force=true", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"media", base, "", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE"},
		{"actor", base, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }, 409, "AUTH_SESSION_CHANGED"},
		{"session", base, "", func(r *http.Request) { r.Header.Del("Cookie") }, 401, "AUTH_UNAUTHENTICATED"},
		{"csrf", base, "", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, 403, "COMMON_CSRF_REJECTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call(t, "POST", path+tc.suffix, tc.body, tc.edit)
			var env struct{ Code string }
			json.Unmarshal(w.Body.Bytes(), &env)
			if w.Code != tc.status || env.Code != tc.code {
				t.Fatalf("got%d %s want%d %s", w.Code, w.Body.String(), tc.status, tc.code)
			}
		})
	}
}
