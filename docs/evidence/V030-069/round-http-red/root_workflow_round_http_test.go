package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func rootHTTPRoundSetup(t *testing.T, state string) (*rootTaskHTTPFixture, string) {
	t.Helper()
	f := rootHTTPTriggerSetupEvents(t, "manual")
	got := rootHTTPData(t, f.call(t, "POST", f.root()+"/workflow-starts", rootHTTPManualBody(t, f, f.id(t)), nil), 202)
	id := rootHTTPString(t, got, "instanceId")
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state=$2 WHERE id=$1", id, state); e != nil {
		t.Fatal(e)
	}
	return f, id
}
func rootHTTPRoundPath(f *rootTaskHTTPFixture, id string) string {
	return f.root() + "/workflows/" + id
}
func rootHTTPRoundBody(t *testing.T, f *rootTaskHTTPFixture, op, kind string) string {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal([]byte(rootHTTPManualBody(t, f, op)), &v); e != nil {
		t.Fatal(e)
	}
	delete(v, "flowId")
	v["kind"] = kind
	b, _ := json.Marshal(v)
	return string(b)
}
func TestRootRoundHTTPSPreviewReworkAndResubmit(t *testing.T) {
	f, id := rootHTTPRoundSetup(t, "rejected")
	path := rootHTTPRoundPath(f, id)
	got := rootHTTPData(t, f.call(t, "GET", path+"/round-actions", "", nil), 200)
	rootHTTPFieldSet(t, got, []string{"instanceId", "flowId", "roundNumber", "latestInstanceId", "state", "definitionVersion", "workflowRevision", "schemaVersion", "recordVersion", "canRework", "canResubmit", "canReview", "editableFieldIds"})
	if string(got["canRework"]) != "true" || string(got["canResubmit"]) != "true" || string(got["canReview"]) != "true" {
		t.Fatal("qualified latest original/configured approver denied")
	}
	rework := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{"` + f.field + `":"reworked"}}`
	saved := rootHTTPData(t, f.call(t, "POST", path+"/rework", rework, nil), 200)
	if string(saved["recordVersion"]) != "2" {
		t.Fatal("rework was not saved")
	}
	replay := rootHTTPData(t, f.call(t, "POST", path+"/rework", rework, nil), 200)
	if !reflect.DeepEqual(saved, replay) {
		t.Fatal("save replay changed")
	}
	op := f.id(t)
	body := strings.Replace(rootHTTPRoundBody(t, f, op, "resubmit"), `"expectedRecordVersion":1`, `"expectedRecordVersion":2`, 1)
	next := rootHTTPData(t, f.call(t, "POST", path+"/round-actions", body, nil), 202)
	rootHTTPFieldSet(t, next, []string{"operationId", "flowId", "previousInstanceId", "instanceId", "roundKind", "roundNumber", "status"})
	if rootHTTPString(t, next, "previousInstanceId") != id || string(next["roundNumber"]) != "2" || rootHTTPString(t, next, "status") != "accepted" {
		t.Fatal("wrong successor receipt")
	}
	if again := rootHTTPData(t, f.call(t, "POST", path+"/round-actions", body, nil), 202); !reflect.DeepEqual(next, again) {
		t.Fatal("start replay changed")
	}
	old := rootHTTPData(t, f.call(t, "GET", path+"/round-actions", "", nil), 200)
	if string(old["canRework"]) != "false" || string(old["canResubmit"]) != "false" || string(old["canReview"]) != "false" {
		t.Fatal("old round writable")
	}
	rootHTTPTriggerCount(t, f, f.record, 2)
}
func TestRootRoundHTTPSReviewByConfiguredUnassignedApprover(t *testing.T) {
	f, id := rootHTTPRoundSetup(t, "completed")
	path := rootHTTPRoundPath(f, id) + "/round-actions"
	got := rootHTTPData(t, f.call(t, "POST", path, rootHTTPRoundBody(t, f, f.id(t), "review"), nil), 202)
	if rootHTTPString(t, got, "roundKind") != "review" || rootHTTPString(t, got, "previousInstanceId") != id {
		t.Fatal("wrong review")
	}
	rootHTTPTriggerCount(t, f, f.record, 2)
}
func TestRootRoundHTTPSIngressAndClosedInputs(t *testing.T) {
	f, id := rootHTTPRoundSetup(t, "rejected")
	path := rootHTTPRoundPath(f, id) + "/round-actions"
	body := rootHTTPRoundBody(t, f, f.id(t), "resubmit")
	for _, c := range []struct {
		status int
		code   string
		edit   func(*http.Request)
	}{
		{401, "AUTH_UNAUTHENTICATED", func(r *http.Request) { r.Header.Del("Cookie") }},
		{403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }},
		{409, "AUTH_SESSION_CHANGED", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
	} {
		rootHTTPError(t, f.call(t, "POST", path, body, c.edit), c.status, c.code)
	}
	for _, bad := range []string{"null", body + "{}", strings.TrimSuffix(body, "}") + `,"actorId":"` + f.actor + `"}`, strings.Replace(body, `"resubmit"`, `"agree"`, 1), strings.Replace(body, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":0`, 1), strings.Repeat(" ", 4097) + body} {
		rootHTTPError(t, f.call(t, "POST", path, bad, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "GET", path+"?bad=1", "", nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", path, body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	rootHTTPError(t, f.call(t, "PATCH", path, body, nil), 404, "API_NOT_FOUND")
	rootHTTPTriggerCount(t, f, f.record, 1)
}
