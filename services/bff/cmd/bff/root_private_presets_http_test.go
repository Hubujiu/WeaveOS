package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func rootPresetHTTPPath(f *rootTaskHTTPFixture) string {
	return "/api/v1/applications/" + f.app + "/forms/" + f.view + "/table-presets"
}
func rootPresetHTTPBody(t *testing.T, f *rootTaskHTTPFixture, op string) string {
	t.Helper()
	r := rootPresetRequest(t, f, "HTTP方案")
	raw, e := json.Marshal(r.State)
	if e != nil {
		t.Fatal(e)
	}
	return `{"operationId":"` + op + `","expectedSchemaVersion":1,` + string(raw[1:])
}
func TestRootPrivatePresetsHTTPClosedCRUDAndBodylessDiscard(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := rootPresetHTTPPath(f)
	op := f.id(t)
	body := rootPresetHTTPBody(t, f, op)
	response := f.call(t, "POST", path, body, nil)
	created := rootHTTPData(t, response, 201)
	rootPresetContractCapture(t, "POST", path, response.Code, response.Body.Bytes(), response.Header().Get("X-Request-Id"))
	id := rootHTTPString(t, created, "id")
	if len(created) != 3 || rootHTTPString(t, created, "operationId") != op || response.Header().Get("Location") != path+"/"+id {
		t.Fatal("create receipt not minimum", response.Body.String())
	}
	raw := f.call(t, "GET", path+"/"+id, "", nil)
	got := rootHTTPData(t, raw, 200)
	rootPresetContractCapture(t, "GET", path+"/"+id, raw.Code, raw.Body.Bytes(), raw.Header().Get("X-Request-Id"))
	if len(got) != 13 || rootHTTPString(t, got, "name") != "HTTP方案" || string(got["invalid"]) != "false" {
		t.Fatal("incomplete persistent configuration", raw.Body.String())
	}
	items := rootHTTPData(t, f.call(t, "GET", path, "", nil), 200)
	var list []any
	if json.Unmarshal(items["items"], &list) != nil || len(list) != 1 {
		t.Fatal("private list")
	}
	update := strings.Replace(body, op, f.id(t), 1)
	update = strings.Replace(update, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":1,"expectedVersion":1`, 1)
	rootHTTPData(t, f.call(t, "PUT", path+"/"+id, update, nil), 200)
	deleteOp := f.id(t)
	deleted := f.call(t, "DELETE", path+"/"+id+"?operationId="+deleteOp+"&expectedVersion=2", "", nil)
	if deleted.Code != 204 || deleted.Body.Len() != 0 || deleted.Header().Get("Location") != "" {
		t.Fatal("discard not bodyless confirmed", deleted.Code, deleted.Body.String())
	}
	again := f.call(t, "DELETE", path+"/"+id+"?operationId="+deleteOp+"&expectedVersion=2", "", nil)
	if again.Code != 204 || again.Body.Len() != 0 {
		t.Fatal("discard replay not bodyless", again.Code, again.Body.String())
	}
}
func TestRootPrivatePresetsHTTPInputAndLiveIdentityGuards(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := rootPresetHTTPPath(f)
	base := rootPresetHTTPBody(t, f, f.id(t))
	tests := []struct {
		name, body, suffix string
		edit               func(*http.Request)
		status             int
		code               string
	}{
		{"unknown_owner", strings.Replace(base, `"name":`, `"ownerId":"x","name":`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"duplicate", strings.Replace(base, `"name":`, `"name":"first","name":`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"surrogate", strings.Replace(base, "HTTP方案", `\ud800`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"raw_body_limit", strings.Repeat(" ", 65537) + base, "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"unknown_query", base, "?owner=other", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"wrong_media", base, "", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE"},
		{"encoding", base, "", func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE"},
		{"cross_actor", base, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }, 409, "AUTH_SESSION_CHANGED"},
		{"missing_session", base, "", func(r *http.Request) { r.Header.Del("Cookie") }, 401, "AUTH_UNAUTHENTICATED"},
		{"csrf", base, "", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, 403, "COMMON_CSRF_REJECTED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call(t, "POST", path+tc.suffix, tc.body, tc.edit)
			var envelope struct{ Code string }
			json.Unmarshal(w.Body.Bytes(), &envelope)
			if w.Code != tc.status || envelope.Code != tc.code {
				t.Fatalf("%d %s want %d %s", w.Code, w.Body.String(), tc.status, tc.code)
			}
		})
	}
}

func rootPresetContractCapture(t *testing.T, method, path string, status int, body []byte, requestID string) {
	t.Helper()
	raw, e := json.Marshal(map[string]any{"method": method, "path": path, "status": status, "body": json.RawMessage(body), "requestId": requestID})
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("V071_CONTRACT_RESPONSE %s", raw)
}
