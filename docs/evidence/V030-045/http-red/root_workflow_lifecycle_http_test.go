package main

import (
	"fmt"
	"net/http"
	"testing"
)

func rootLifecycleHTTPPath(f *rootTaskHTTPFixture) string {
	return fmt.Sprintf("/api/v1/applications/%s/forms/%s/records/%s/workflow-instances/%s", f.app, f.view, f.record, f.instance)
}
func TestRootLifecycleHTTPSPreviewAcceptAndOperation(t *testing.T) {
	for _, action := range []string{"withdraw", "return"} {
		t.Run(action, func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			path := rootLifecycleHTTPPath(f)
			preview := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
			token := rootHTTPString(t, preview, "basisToken")
			op := f.id(t)
			target := ""
			if action == "return" {
				target = fmt.Sprintf(`,"targetNodeId":%q`, f.node)
			}
			body := fmt.Sprintf(`{"operationId":%q,"action":%q,"basisToken":%q%s}`, op, action, token, target)
			r := f.call(t, "POST", path+"/actions", body, nil)
			a := rootHTTPData(t, r, 202)
			if rootHTTPString(t, a, "status") != "pending" || r.Header().Get("Location") != "/api/v1/application-workflow-operations/"+op {
				t.Fatalf("false final response %s", r.Body.String())
			}
			b := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-workflow-operations/"+op, "", nil), 200)
			if rootHTTPString(t, b, "commandId") != rootHTTPString(t, a, "commandId") {
				t.Fatal("operation identity changed")
			}
			again := rootHTTPData(t, f.call(t, "POST", path+"/actions", body, nil), 202)
			if rootHTTPString(t, again, "commandId") != rootHTTPString(t, a, "commandId") {
				t.Fatal("HTTP replay created command")
			}
		})
	}
}
func TestRootLifecycleHTTPSRejectsClosedBodyAndQuery(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootLifecycleHTTPPath(f)
	a := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
	token := rootHTTPString(t, a, "basisToken")
	op := f.id(t)
	for _, body := range []string{`{}`, fmt.Sprintf(`{"operationId":%q,"action":"withdraw","basisToken":%q,"targetNodeId":%q}`, op, token, f.node), fmt.Sprintf(`{"operationId":%q,"action":"return","basisToken":%q}`, op, token), fmt.Sprintf(`{"operationId":%q,"action":"withdraw","basisToken":%q,"actorId":%q}`, op, token, f.actor), fmt.Sprintf(`{"operationId":%q,"action":"withdraw","action":"return","basisToken":%q}`, op, token)} {
		if r := f.call(t, "POST", path+"/actions", body, nil); r.Code != 400 {
			t.Fatalf("bad body got%d %s", r.Code, r.Body.String())
		}
	}
	if r := f.call(t, "GET", path+"/lifecycle?actor=x", "", nil); r.Code != 400 {
		t.Fatalf("query %d", r.Code)
	}
}
func TestRootLifecycleHTTPSIdentityAndCSRF(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootLifecycleHTTPPath(f)
	a := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
	body := fmt.Sprintf(`{"operationId":%q,"action":"withdraw","basisToken":%q}`, f.id(t), rootHTTPString(t, a, "basisToken"))
	for _, v := range []struct {
		status int
		edit   func(*http.Request)
	}{{401, func(r *http.Request) { r.Header.Del("Cookie") }}, {403, func(r *http.Request) { r.Header.Set("X-CSRF-Token", "invalid") }}, {409, func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }}} {
		if r := f.call(t, "POST", path+"/actions", body, v.edit); r.Code != v.status {
			t.Fatalf("identity want%d got%d", v.status, r.Code)
		}
	}
}
func TestRootLifecycleHTTPSOldPreviewCannotApproveChangedInstance(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootLifecycleHTTPPath(f)
	a := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET sequence=sequence+1 WHERE id=$1", f.instance); e != nil {
		t.Fatal(e)
	}
	body := fmt.Sprintf(`{"operationId":%q,"action":"withdraw","basisToken":%q}`, f.id(t), rootHTTPString(t, a, "basisToken"))
	if r := f.call(t, "POST", path+"/actions", body, nil); r.Code != 409 {
		t.Fatalf("stale accepted%d %s", r.Code, r.Body.String())
	}
}
