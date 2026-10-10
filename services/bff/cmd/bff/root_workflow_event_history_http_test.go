package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRootWorkflowEventHistoryHTTPSPages(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	detail := rootHTTPConfirmedEvent(t, f)
	path := detail[:strings.LastIndex(detail, "/")]
	seen := map[string]bool{}
	token := ""
	for n := 0; n < 2; n++ {
		q := "?pageSize=1"
		if token != "" {
			q += "&pageToken=" + token
		}
		w := f.call(t, "GET", path+q, "", func(r *http.Request) { r.Header.Del("X-CSRF-Token") })
		data := rootHTTPData(t, w, 200)
		rootHTTPFieldSet(t, data, []string{"items"})
		var items []map[string]json.RawMessage
		if json.Unmarshal(data["items"], &items) != nil || len(items) != 1 {
			t.Fatal("wrong list")
		}
		rootHTTPFieldSet(t, items[0], []string{"flowName", "flowNameSource", "id", "instanceId", "flowId", "nodeId", "targetNodeId", "actorId", "action", "outcome", "sequence", "schemaVersion", "recordVersion", "occurredAt"})
		id := rootHTTPString(t, items[0], "id")
		if seen[id] {
			t.Fatal("duplicate cursor event")
		}
		seen[id] = true
		if n == 0 && (path+"/"+id != detail || rootHTTPString(t, items[0], "action") != "agree") {
			t.Fatal("latest confirmed missing")
		}
		if n == 1 && rootHTTPString(t, items[0], "action") != "start" {
			t.Fatal("old start missing")
		}
		var env struct {
			Meta struct {
				Pagination struct {
					HasMore       bool
					NextPageToken *string
				}
			}
		}
		if json.Unmarshal(w.Body.Bytes(), &env) != nil {
			t.Fatal("meta")
		}
		p := env.Meta.Pagination
		if p.HasMore != (n == 0) || (p.NextPageToken != nil) != (n == 0) {
			t.Fatal("wrong pagination", p)
		}
		if p.NextPageToken != nil {
			token = *p.NextPageToken
		}
	}
}
func TestRootWorkflowEventHistoryHTTPSClosedInput(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	detail := rootHTTPConfirmedEvent(t, f)
	path := detail[:strings.LastIndex(detail, "/")]
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409, "AUTH_SESSION_CHANGED")
	for _, q := range []string{"?x=1", "?pageSize=0", "?pageSize=101", "?pageSize=01", "?pageSize=", "?pageSize=1&pageSize=2", "?pageToken=", "?pageToken=a&pageToken=b", "?"} {
		rootHTTPError(t, f.call(t, "GET", path+q, "", nil), 400, "COMMON_VALIDATION_FAILED")
	}
	for _, body := range []string{"{}", "null", " "} {
		rootHTTPError(t, f.call(t, "GET", path, body, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	rootHTTPError(t, f.call(t, "GET", path+"?pageToken=bad", "", nil), 409, "APPLICATION_QUERY_CONTEXT_EXPIRED")
	for _, method := range []string{"POST", "DELETE", "PATCH"} {
		rootHTTPError(t, f.call(t, method, path, "", nil), 404, "API_NOT_FOUND")
	}
}
