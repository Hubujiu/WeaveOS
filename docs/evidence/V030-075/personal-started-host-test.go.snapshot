package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

const startedHTTPPath = "/api/v1/workflow-started/search"

func TestRootPersonalStartedHTTPSRealSessionAndConfirmedStateRefresh(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	data := rootHTTPData(t, f.call(t, "POST", startedHTTPPath, `{"page":1}`, nil), 200)
	rootHTTPFieldSet(t, data, []string{"items", "total", "page", "pageSize", "queryVersion"})
	var items []map[string]json.RawMessage
	if e := json.Unmarshal(data["items"], &items); e != nil || len(items) != 1 || string(data["total"]) != "1" {
		t.Fatal("missing current initiator row", e)
	}
	row := items[0]
	rootHTTPFieldSet(t, row, []string{"appId", "viewId", "recordId", "instance"})
	for k, v := range map[string]string{"appId": f.app, "viewId": f.view, "recordId": f.record} {
		if rootHTTPString(t, row, k) != v {
			t.Fatal("wrong locator", k)
		}
	}
	var instance map[string]json.RawMessage
	json.Unmarshal(row["instance"], &instance)
	rootHTTPFieldSet(t, instance, []string{"id", "flowId", "name", "initiatorId", "state", "createdAt", "updatedAt", "definitionVersion", "sequence"})
	if rootHTTPString(t, instance, "id") != f.instance || rootHTTPString(t, instance, "initiatorId") != f.actor || rootHTTPString(t, instance, "state") != "active" {
		t.Fatal("incorrect personal instance")
	}
	rootHTTPConfirmedEvent(t, f)
	rootHTTPError(t, f.call(t, "POST", startedHTTPPath, `{"page":1,"queryVersion":"`+rootHTTPString(t, data, "queryVersion")+`"}`, nil), 409, "APPLICATION_QUERY_CHANGED")
	fresh := rootHTTPData(t, f.call(t, "POST", startedHTTPPath, `{"page":1}`, nil), 200)
	json.Unmarshal(fresh["items"], &items)
	json.Unmarshal(items[0]["instance"], &instance)
	if rootHTTPString(t, instance, "state") != "completed" {
		t.Fatal("fresh summary still active")
	}
}
func TestRootPersonalStartedHTTPSRejectsActorInjectionAndAbsentIdentity(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	for _, body := range []string{`{"page":1,"actorId":"` + f.other + `"}`, `{"page":1,"appId":"` + f.app + `"}`, `{"page":1,"page":2}`, `{"page":null}`, `{"page":1,"pageSize":101}`} {
		rootHTTPError(t, f.call(t, "POST", startedHTTPPath, body, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "POST", startedHTTPPath, `{"page":1}`, func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "POST", startedHTTPPath, `{"page":1}`, func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 403, "COMMON_CSRF_REJECTED")
	rootHTTPError(t, f.call(t, "GET", startedHTTPPath, "", nil), 404, "API_NOT_FOUND")
}
