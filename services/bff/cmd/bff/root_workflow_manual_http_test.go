package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func rootHTTPManualBody(t *testing.T, f *rootTaskHTTPFixture, op string) string {
	t.Helper()
	var revision int64
	if e := f.owner.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.flow).Scan(&revision); e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"operationId": op, "flowId": f.flow, "expectedWorkflowRevision": revision, "expectedSchemaVersion": 1, "expectedRecordVersion": 1})
	return string(body)
}
func TestRootManualStartHTTPSCreatorAcceptsAndReplaysMinimum(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := f.root() + "/workflow-starts"
	op := f.id(t)
	body := rootHTTPManualBody(t, f, op)
	got := rootHTTPData(t, f.call(t, "POST", path, body, nil), 202)
	rootHTTPFieldSet(t, got, []string{"operationId", "flowId", "instanceId", "status", "ignored"})
	if rootHTTPString(t, got, "operationId") != op || rootHTTPString(t, got, "flowId") != f.flow || rootHTTPString(t, got, "status") != "accepted" || string(got["ignored"]) != "false" {
		t.Fatal("not a durable accepted intent")
	}
	replay := rootHTTPData(t, f.call(t, "POST", path, body, nil), 202)
	if !reflect.DeepEqual(replay, got) {
		t.Fatal("manual replay changed receipt")
	}
	ignored := rootHTTPData(t, f.call(t, "POST", path, rootHTTPManualBody(t, f, f.id(t)), nil), 202)
	if rootHTTPString(t, ignored, "instanceId") != rootHTTPString(t, got, "instanceId") || rootHTTPString(t, ignored, "status") != "ignored" || string(ignored["ignored"]) != "true" {
		t.Fatal("in-flight manual not ignored")
	}
	rootHTTPTriggerCount(t, f, f.record, 1)
	receipt := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-operations/"+op, "", nil), 200)
	if string(receipt["httpStatus"]) != "202" {
		t.Fatal("confirmed operation lost accepted result")
	}
}
func TestRootManualStartHTTPSPreservesIngressAndClosedBody(t *testing.T) {
	f := rootHTTPTriggerSetupEvents(t, "manual")
	path := f.root() + "/workflow-starts"
	body := rootHTTPManualBody(t, f, f.id(t))
	for _, c := range []struct {
		name   string
		status int
		code   string
		edit   func(*http.Request)
	}{
		{"session", 401, "AUTH_UNAUTHENTICATED", func(r *http.Request) { r.Header.Del("Cookie") }},
		{"csrf", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }},
		{"origin", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Set("Origin", "https://foreign.invalid") }},
		{"actor", 409, "AUTH_SESSION_CHANGED", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
	} {
		t.Run(c.name, func(t *testing.T) { rootHTTPError(t, f.call(t, "POST", path, body, c.edit), c.status, c.code) })
	}
	for _, raw := range []string{"null", body + "{}", strings.TrimSuffix(body, "}") + `,"actorId":"` + f.actor + `"}`, strings.TrimSuffix(body, "}") + `,"operationId":"` + f.id(t) + `"}`, strings.Replace(body, `"expectedRecordVersion":1`, `"expectedRecordVersion":0`, 1), strings.Replace(body, `"expectedRecordVersion":1`, `"expectedRecordVersion":1.5`, 1), strings.Repeat(" ", 4097) + body} {
		rootHTTPError(t, f.call(t, "POST", path, raw, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "POST", path+"?extra=1", body, nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", path, body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	rootHTTPError(t, f.call(t, "PATCH", path, body, nil), 404, "API_NOT_FOUND")
	rootHTTPTriggerCount(t, f, f.record, 0)
}
