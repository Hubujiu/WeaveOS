package applications

import (
	"strings"
	"testing"
)

func TestB5SafeRevisionRequestBoundary(t *testing.T) {
	f := fixture(t, true)
	app, _ := f.create(t)
	path := "/api/v1/applications/" + app + "/permission-groups"
	w := f.call("POST", path, map[string]any{"name": "revision", "operationId": f.operation(t), "expectedPolicyRevision": int64(9007199254740991)})
	if w.Code != 409 || !strings.Contains(w.Body.String(), "APPLICATION_POLICY_CONFLICT") {
		t.Fatalf("contract-valid safe integer must reach current CAS, not input rejection: %d %s", w.Code, w.Body.String())
	}
	if w := f.call("POST", path, map[string]any{"name": "revision", "operationId": f.operation(t), "expectedPolicyRevision": int64(9007199254740992)}); w.Code != 400 {
		t.Fatal("beyond safe-integer revision remains invalid")
	}
}
func TestB5NULNameIsInvalidAndLeavesNoOperation(t *testing.T) {
	f := fixture(t, true)
	op := f.operation(t)
	w := f.call("POST", "/api/v1/applications", map[string]any{"name": "x\x00y", "operationId": op})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "COMMON_INVALID_ARGUMENT") {
		t.Fatalf("PostgreSQL-unrepresentable NUL name must be invalid input: %d %s", w.Code, w.Body.String())
	}
	if w := f.call("GET", "/api/v1/application-operations/"+op, nil); w.Code != 404 {
		t.Fatal("invalid input must leave no operation result")
	}
}
