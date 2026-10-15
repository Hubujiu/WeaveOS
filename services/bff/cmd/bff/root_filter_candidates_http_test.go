package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func filterHTTPPath(f *rootTaskHTTPFixture) string {
	return "/api/v1/applications/" + f.app + "/forms/" + f.view + "/filter-candidates"
}
func TestRootFilterCandidatesHTTPActualRecordValuesAndClosedMinimalProjection(t *testing.T) {
	f := rootHTTPResourceSetupKind(t, workflowRuntimeConfig{}, "member")
	w := f.call(t, "GET", filterHTTPPath(f)+"?fieldId="+f.field, "", func(r *http.Request) { r.Header.Del("X-CSRF-Token") })
	data := rootHTTPData(t, w, 200)
	rootHTTPFieldSet(t, data, []string{"items"})
	var items []map[string]json.RawMessage
	if json.Unmarshal(data["items"], &items) != nil || len(items) != 1 {
		t.Fatal("expected only recorded member", w.Body.String())
	}
	rootHTTPFieldSet(t, items[0], []string{"id", "label", "deleted"})
	if rootHTTPString(t, items[0], "id") != f.other || string(items[0]["deleted"]) != "false" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("wrong display-only projection", w.Body.String())
	}
	var envelope struct {
		Meta struct {
			Pagination struct {
				NextPageToken *string
				HasMore       bool
			}
		}
	}
	if json.Unmarshal(w.Body.Bytes(), &envelope) != nil || envelope.Meta.Pagination.NextPageToken != nil || envelope.Meta.Pagination.HasMore {
		t.Fatal("unexpected continuation", w.Body.String())
	}
}
func TestRootFilterCandidatesHTTPStrictQueryAndCurrentIdentity(t *testing.T) {
	f := rootHTTPResourceSetupKind(t, workflowRuntimeConfig{}, "member")
	base := filterHTTPPath(f)
	for _, q := range []string{"", "?fieldId=bad", "?fieldId=" + f.field + "&fieldId=" + f.field, "?fieldId=" + f.field + "&action=edit", "?fieldId=" + f.field + "&recordId=" + f.record, "?fieldId=" + f.field + "&pageSize=0", "?fieldId=" + f.field + "&pageSize=51", "?fieldId=" + f.field + "&pageSize=01", "?fieldId=" + f.field + "&pageToken=", "?fieldId=" + f.field + "&q=%FF", "?fieldId=" + f.field + "&q=%00"} {
		t.Run(q, func(t *testing.T) {
			rootHTTPError(t, f.call(t, "GET", base+q, "", nil), 400, "COMMON_VALIDATION_FAILED")
		})
	}
	path := base + "?fieldId=" + f.field
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409, "AUTH_SESSION_CHANGED")
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", "invalid") }), 400, "COMMON_VALIDATION_FAILED")
}
