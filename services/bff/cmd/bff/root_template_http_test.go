package main

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rootTemplateHTTPBody(t *testing.T, m apptemplates.Manifest, b []apptemplates.Binding) string {
	t.Helper()
	raw, e := json.Marshal(map[string]any{"manifest": m, "bindings": b})
	if e != nil {
		t.Fatal(e)
	}
	return string(raw)
}
func TestRootTemplateHTTPExportAndReadOnlyPreflight(t *testing.T) {
	f, m, b, _ := rootTemplatePreflightSource(t)
	rootTemplateGrantCreate(t, f, f.actor)
	path := "/api/v1/applications/" + f.app + "/structure-template"
	w := f.call(t, "GET", path, "", nil)
	data := rootHTTPData(t, w, 200)
	if len(data) != 8 || string(data["format"]) != `"weaveos.structure-template"` || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-Id") == "" {
		t.Fatal("export contract changed")
	}
	raw, _ := json.Marshal(data)
	if _, e := apptemplates.DecodeManifest(raw); e != nil {
		t.Fatal("invalid public manifest", e)
	}
	if strings.Contains(string(raw), f.record) || strings.Contains(string(raw), f.instance) {
		t.Fatal("runtime identity leak")
	}
	head := f.call(t, "HEAD", path, "", nil)
	if head.Code != 200 || head.Body.Len() != 0 {
		t.Fatal("HEAD returned body or wrong status", head.Code)
	}
	before := rootTemplateReadCounts(t, f)
	pre := f.call(t, "POST", "/api/v1/application-templates/preflight", rootTemplateHTTPBody(t, m, b), nil)
	result := rootHTTPData(t, pre, 200)
	if len(result) != 2 || string(result["valid"]) != "true" || pre.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preflight response changed")
	}
	after := rootTemplateReadCounts(t, f)
	for i, n := range before {
		if n != after[i] {
			t.Fatal("preflight HTTP mutated business state")
		}
	}
}
func TestRootTemplateHTTPAuthAndFailureBoundaries(t *testing.T) {
	f, m, b, _ := rootTemplatePreflightSource(t)
	body := rootTemplateHTTPBody(t, m, b)
	export := "/api/v1/applications/" + f.app + "/structure-template"
	preflight := "/api/v1/application-templates/preflight"
	for _, tc := range []struct {
		name, method, path, code string
		status                   int
		edit                     func(*http.Request)
	}{
		{"unauthenticated", "GET", export, "AUTH_UNAUTHENTICATED", 401, func(r *http.Request) { r.Header.Del("Cookie") }},
		{"csrf", "POST", preflight, "COMMON_CSRF_REJECTED", 403, func(r *http.Request) { r.Header.Set("X-CSRF-Token", "wrong") }},
		{"actor", "POST", preflight, "AUTH_SESSION_CHANGED", 409, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
		{"owner-not-creator", "POST", preflight, "APPLICATION_FORBIDDEN", 403, nil},
		{"query", "GET", export + "?records=true", "COMMON_INVALID_ARGUMENT", 400, nil},
		{"media", "POST", preflight, "COMMON_UNSUPPORTED_MEDIA_TYPE", 415, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }},
		{"encoding", "POST", preflight, "COMMON_UNSUPPORTED_MEDIA_TYPE", 415, func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call(t, tc.method, tc.path, body, tc.edit)
			rootTemplateHTTPError(t, w, tc.status)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("wrong safe error", w.Body.String())
			}
		})
	}
	sid, csrf, e := f.store.Create(f.ctx, session.Record{UserID: f.other, SessionRef: f.id(t), AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	defer f.store.Revoke(f.ctx, sid)
	rootTemplateHTTPError(t, f.call(t, "GET", export, "", func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
	}), 403)
	for _, tc := range []struct{ method, path, allow string }{{"POST", export, "GET, HEAD"}, {"GET", preflight, "POST"}} {
		w := f.call(t, tc.method, tc.path, body, nil)
		rootTemplateHTTPError(t, w, 405)
		if w.Header().Get("Allow") != tc.allow {
			t.Fatal("missing method allow")
		}
	}
	rootTemplateHTTPError(t, f.call(t, "POST", "/api/v1/application-templates/unknown", body, nil), 404)
}
func TestRootTemplateHTTPClosedPayloadAndSize(t *testing.T) {
	f, m, b, _ := rootTemplatePreflightSource(t)
	rootTemplateGrantCreate(t, f, f.actor)
	body := rootTemplateHTTPBody(t, m, b)
	var nullObject map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &nullObject) != nil {
		t.Fatal("bad fixture")
	}
	nullObject["bindings"] = json.RawMessage("null")
	nullBody, _ := json.Marshal(nullObject)
	cases := map[string]string{
		"unknown":         strings.TrimSuffix(body, "}") + `,"records":[]}`,
		"duplicate":       `{"bindings":[],` + body[1:],
		"case":            strings.Replace(body, `"manifest":`, `"Manifest":`, 1),
		"null-bindings":   string(nullBody),
		"unknown-binding": strings.Replace(body, `"sourceId":`, `"extra":true,"sourceId":`, 1),
		"trailing":        body + `{}`,
		"utf8":            string([]byte{255}) + body,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			w := f.call(t, "POST", "/api/v1/application-templates/preflight", raw, nil)
			rootTemplateHTTPError(t, w, 400)
			if !strings.Contains(w.Body.String(), "COMMON_INVALID_ARGUMENT") {
				t.Fatal(w.Body.String())
			}
		})
	}
	t.Run("whole-limit", func(t *testing.T) {
		rootTemplateHTTPError(t, f.call(t, "POST", "/api/v1/application-templates/preflight", strings.Repeat(" ", 4*1024*1024+1), nil), 413)
	})
	t.Run("manifest-limit", func(t *testing.T) {
		raw, _ := json.Marshal(m)
		bindings, _ := json.Marshal(b)
		body := `{"manifest":` + strings.TrimSuffix(string(raw), "}") + strings.Repeat(" ", 1048576) + `},"bindings":` + string(bindings) + `}`
		rootTemplateHTTPError(t, f.call(t, "POST", "/api/v1/application-templates/preflight", body, nil), 413)
	})
}

func rootTemplateHTTPError(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	var env map[string]json.RawMessage
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &env) != nil || len(env) != 4 || string(env["code"]) == `"OK"` || string(env["data"]) != "null" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-Id") == "" {
		t.Fatalf("want safe error %d actual %d %s", status, w.Code, w.Body.String())
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(env["meta"], &meta) != nil || len(meta) != 1 || len(meta["requestId"]) < 3 {
		t.Fatal("missing request correlation")
	}
}
