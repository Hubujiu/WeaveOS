package appstructure

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestRootWorkflowHTTPInvalidAssigneeIdentityIsClientErrorAndAtomic(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	handler := rootWorkflowHTTPHandler(s)
	count := func() (int, int, int) {
		t.Helper()
		var defs, ops, audit int
		if e := s.f.owner.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM applications.workflow_definitions),(SELECT count(*) FROM applications.operations),(SELECT count(*) FROM auth.authentication_events)").Scan(&defs, &ops, &audit); e != nil {
			t.Fatal(e)
		}
		return defs, ops, audit
	}
	for _, id := range []string{"not-a-uuid", "", "00000000-0000-0000-0000-000000000000", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "11111111-1111-4111-8111-111111111111 "} {
		t.Run(id, func(t *testing.T) {
			d0, o0, a0 := count()
			body := rootWorkflowHTTPBody(t, s, id)
			raw, e := json.Marshal(body)
			if e != nil {
				t.Fatal(e)
			}
			w := rootWorkflowHTTPRequest(t, s, handler, http.MethodPut, rootWorkflowHTTPPath(s, uuid(t, s.f.owner), "definition"), string(raw), true, true, "", nil)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("malformed assignee identity must be a client error, got %d %s", w.Code, w.Body.String())
			}
			var envelope struct {
				Code string `json:"code"`
			}
			if e = json.Unmarshal(w.Body.Bytes(), &envelope); e != nil {
				t.Fatal(e)
			}
			if envelope.Code != "COMMON_INVALID_ARGUMENT" {
				t.Fatalf("unexpected client error %q", envelope.Code)
			}
			d1, o1, a1 := count()
			if d1 != d0 || o1 != o0 || a1 != a0 {
				t.Fatalf("invalid graph changed definitions/operations/audit before=%d/%d/%d after=%d/%d/%d", d0, o0, a0, d1, o1, a1)
			}
		})
	}
}
