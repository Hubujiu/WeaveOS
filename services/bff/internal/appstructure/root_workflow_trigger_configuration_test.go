package appstructure

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func triggerTLSCall(t *testing.T, s rootWorkflowManagementFixture, server *httptest.Server, flow, method string, body any, login, csrf bool) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, server.URL+rootWorkflowHTTPPath(s, flow, "definition"), strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "weaveos.test"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://weaveos.test")
	if login {
		req.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: s.f.sid})
		req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: s.f.csrf})
	}
	if csrf {
		req.Header.Set("X-CSRF-Token", s.f.csrf)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.TLS == nil {
		t.Fatal("fixture must use real TLS transport")
	}
	w := httptest.NewRecorder()
	for k, v := range response.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(response.StatusCode)
	if _, err = io.Copy(w, response.Body); err != nil {
		t.Fatal(err)
	}
	return w
}
func TestRootWorkflowTriggerConfigurationHTTPSRoundTrip(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	server := httptest.NewTLSServer(rootWorkflowHTTPHandler(s))
	defer server.Close()
	flow := uuid(t, s.f.owner)
	body := rootWorkflowHTTPBody(t, s, s.f.actor)
	configs := []any{map[string]any{"event": "record.created", "condition": map[string]any{"operator": "and", "children": []any{map[string]any{"fieldId": s.field, "operator": "eq", "value": "alpha"}}}}}
	body["triggers"] = configs
	rootWorkflowHTTPError(t, triggerTLSCall(t, s, server, flow, "PUT", body, false, true), 401, "AUTH_UNAUTHENTICATED")
	rootWorkflowHTTPError(t, triggerTLSCall(t, s, server, flow, "PUT", body, true, false), 403, "COMMON_CSRF_REJECTED")
	saved := data(t, triggerTLSCall(t, s, server, flow, "PUT", body, true, true), 201)
	read := data(t, triggerTLSCall(t, s, server, flow, "GET", nil, true, true), 200)
	if !reflect.DeepEqual(read["triggers"], configs) {
		t.Fatalf("trigger config not returned: %+v", read["triggers"])
	}
	// Existing clients omit the additive setting; candidate configuration survives.
	delete(body, "triggers")
	body["operationId"] = uuid(t, s.f.owner)
	body["expectedRevision"] = saved["revision"]
	saved = data(t, triggerTLSCall(t, s, server, flow, "PUT", body, true, true), 200)
	read = data(t, triggerTLSCall(t, s, server, flow, "GET", nil, true, true), 200)
	if !reflect.DeepEqual(read["triggers"], configs) {
		t.Fatal("old client omission cleared configuration")
	}
	body["triggers"] = []any{}
	body["operationId"] = uuid(t, s.f.owner)
	body["expectedRevision"] = saved["revision"]
	data(t, triggerTLSCall(t, s, server, flow, "PUT", body, true, true), 200)
	read = data(t, triggerTLSCall(t, s, server, flow, "GET", nil, true, true), 200)
	got, ok := read["triggers"].([]any)
	if !ok || len(got) != 0 {
		t.Fatalf("explicit empty did not clear configuration: %+v", read)
	}
}
func TestRootWorkflowTriggerConfigurationHTTPSLegacyAndMalformed(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	server := httptest.NewTLSServer(rootWorkflowHTTPHandler(s))
	defer server.Close()
	flow := uuid(t, s.f.owner)
	body := rootWorkflowHTTPBody(t, s, s.f.actor)
	data(t, triggerTLSCall(t, s, server, flow, "PUT", body, true, true), 201)
	read := data(t, triggerTLSCall(t, s, server, flow, "GET", nil, true, true), 200)
	if got, ok := read["triggers"].([]any); !ok || len(got) != 0 {
		t.Fatalf("legacy config must be [], got %+v", read["triggers"])
	}
	for name, value := range map[string]any{
		"null": nil, "object": map[string]any{},
		"missing_condition": []any{map[string]any{"event": "manual"}},
		"unknown_event":     []any{map[string]any{"event": "timer", "condition": nil}},
		"unknown_key":       []any{map[string]any{"event": "manual", "condition": nil, "script": "x"}},
		"duplicate_event":   []any{map[string]any{"event": "manual", "condition": nil}, map[string]any{"event": "manual", "condition": nil}},
	} {
		t.Run(name, func(t *testing.T) {
			bad := rootWorkflowHTTPBody(t, s, s.f.actor)
			bad["triggers"] = value
			rootWorkflowHTTPError(t, triggerTLSCall(t, s, server, uuid(t, s.f.owner), "PUT", bad, true, true), 400, "COMMON_INVALID_ARGUMENT")
		})
	}
}
