package personnel

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// OpenAPI format:uuid permits either hexadecimal letter case. Q25 relations
// are sets of UUID identities, not sets of case-sensitive spellings.
func TestUUIDContractCaseInsensitiveReferencesAndRelations(t *testing.T) {
	t.Run("detail path", func(t *testing.T) {
		f := setupWeb(t)
		if w := f.request("GET", "/api/v1/personnel/members/"+strings.ToUpper(f.actor.UserID), "", true, false); w.Code != 200 {
			t.Fatalf("valid uppercase UUID path got %d", w.Code)
		}
	})
	t.Run("template set", func(t *testing.T) {
		f := setupWeb(t)
		body := fmt.Sprintf(`{"name":"I1","description":"","version":1,"permissionCodes":["app.test.A"],"templateIds":["%s","%s"]}`, f.template, strings.ToUpper(f.template))
		w := f.request("PUT", "/api/v1/personnel/identities/"+f.i1, body, true, true)
		if w.Code != 200 {
			t.Fatalf("same UUID spelled in two letter cases must be one template reference, got %d", w.Code)
		}
		var ids []string
		_ = json.Unmarshal(envelopeData(t, w)["templateIds"], &ids)
		if len(ids) != 1 || ids[0] != f.template {
			t.Fatal("exactly one canonical template reference required")
		}
	})
	t.Run("identity set", func(t *testing.T) {
		f := setupWeb(t)
		body := fmt.Sprintf(`{"version":0,"identityIds":["%s","%s"]}`, f.i1, strings.ToUpper(f.i1))
		w := f.requestCurrentQuery(t, "PUT", "/api/v1/personnel/members/"+f.actor.UserID+"/identities", body, true, true)
		if w.Code != 200 {
			t.Fatalf("same UUID must be one member identity reference, got %d", w.Code)
		}
		var ids []string
		_ = json.Unmarshal(envelopeData(t, w)["identityIds"], &ids)
		if len(ids) != 1 || ids[0] != f.i1 {
			t.Fatal("exactly one canonical identity relation required")
		}
	})
	t.Run("explicit group removal", func(t *testing.T) {
		f := setupWeb(t)
		department := rootDepartment(t, f.fixture)
		path := "/api/v1/personnel/members/" + f.actor.UserID + "/groups"
		add := fmt.Sprintf(`{"operation":"add","departmentId":"%s","version":0}`, department)
		if w := f.requestCurrentQuery(t, "POST", path, add, true, true); w.Code != 200 {
			t.Fatal("lowercase fixture membership could not be created")
		}
		remove := fmt.Sprintf(`{"operation":"remove","departmentId":"%s","version":1}`, strings.ToUpper(department))
		w := f.requestCurrentQuery(t, "POST", path, remove, true, true)
		if w.Code != 200 {
			t.Fatalf("uppercase reference must remove the same department, got %d", w.Code)
		}
		var ids []string
		_ = json.Unmarshal(envelopeData(t, w)["departmentIds"], &ids)
		if len(ids) != 0 {
			t.Fatal("case spelling must not turn explicit removal into a no-op")
		}
	})
}
