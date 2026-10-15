package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func rootHTTPCandidateSetup(t *testing.T) *rootTaskHTTPFixture {
	t.Helper()
	return rootHTTPTaskFromResource(t, rootHTTPResourceSetupKind(t, workflowRuntimeConfig{}, "member"), true)
}
func TestRootWorkflowCandidatesHTTPRealMemberPageAndNoStore(t *testing.T) {
	f := rootHTTPCandidateSetup(t)
	w := f.call(t, "GET", f.taskPath()+"/reference-candidates?fieldId="+f.field+"&pageSize=1", "", nil)
	data := rootHTTPData(t, w, 200)
	rootHTTPFieldSet(t, data, []string{"items"})
	var items []map[string]json.RawMessage
	if json.Unmarshal(data["items"], &items) != nil || len(items) != 1 {
		t.Fatal("bounded active candidate page missing")
	}
	rootHTTPFieldSet(t, items[0], []string{"id", "label", "status"})
	if rootHTTPString(t, items[0], "status") != "active" {
		t.Fatal("non-active selectable member")
	}
	var envelope struct {
		Meta struct {
			Pagination struct {
				NextPageToken *string
				HasMore       bool
			}
		}
	}
	if json.Unmarshal(w.Body.Bytes(), &envelope) != nil || !envelope.Meta.Pagination.HasMore || envelope.Meta.Pagination.NextPageToken == nil {
		t.Fatal("standard pagination metadata missing")
	}
	next := f.taskPath() + "/reference-candidates?fieldId=" + f.field + "&pageSize=1&pageToken=" + *envelope.Meta.Pagination.NextPageToken
	rootHTTPData(t, f.call(t, "GET", next, "", nil), 200)
}
func TestRootWorkflowCandidatesHTTPStrictClosedQuery(t *testing.T) {
	f := rootHTTPCandidateSetup(t)
	base := f.taskPath() + "/reference-candidates"
	for _, query := range []string{"", "?fieldId=bad", "?fieldId=" + f.field + "&fieldId=" + f.field, "?fieldId=" + f.field + "&action=edit", "?fieldId=" + f.field + "&actorId=" + f.actor, "?fieldId=" + f.field + "&pageSize=0", "?fieldId=" + f.field + "&pageSize=51", "?fieldId=" + f.field + "&pageSize=01", "?fieldId=" + f.field + "&pageToken="} {
		t.Run(query, func(t *testing.T) {
			rootHTTPError(t, f.call(t, "GET", base+query, "", nil), 400, "COMMON_VALIDATION_FAILED")
		})
	}
}
func TestRootWorkflowCandidatesHTTPRequiresSessionAndExpectedActor(t *testing.T) {
	f := rootHTTPCandidateSetup(t)
	path := f.taskPath() + "/reference-candidates?fieldId=" + f.field
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409, "AUTH_SESSION_CHANGED")
	rootHTTPError(t, f.call(t, "POST", path, `{}`, nil), 404, "API_NOT_FOUND")
}
