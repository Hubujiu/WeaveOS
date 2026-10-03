package applications

import (
	"context"
	"testing"
)

func TestB5RejectsCaseFoldedGrantFields(t *testing.T) {
	for _, mode := range []string{"uppercase", "alias"} {
		t.Run(mode, func(t *testing.T) {
			f := fixture(t, true)
			app, _ := f.create(t)
			base := "/api/v1/applications/" + app + "/permission-groups"
			gid := value(t, data(t, f.call("POST", base, map[string]any{"name": "closed", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201), "id")
			op := f.operation(t)
			grant := `{"ResourceKind":"application","ResourceID":"` + app + `","Action":"menu.enter","RowScope":"all","Fields":[]}`
			if mode == "alias" {
				grant = `{"resourceKind":"application","resourceId":"` + f.uuid(t) + `","ResourceID":"` + app + `","action":"menu.enter","rowScope":"all","fields":[]}`
			}
			body := `{"grants":[` + grant + `],"operationId":"` + op + `","expectedPolicyRevision":2}`
			w := f.request("PUT", base+"/"+gid+"/grants", body, true, true)
			if w.Code != 400 {
				t.Fatalf("closed grant keys must reject case-folded/alias names: %d %s", w.Code, w.Body.String())
			}
			var rev, n int
			if err := f.owner.QueryRow(context.Background(), "SELECT policy_revision FROM applications.apps WHERE id=$1", app).Scan(&rev); err != nil || rev != 2 {
				t.Fatal("closed DTO rejection must preserve revision")
			}
			if err := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&n); err != nil || n != 0 {
				t.Fatal("closed DTO rejection must leave no operation result")
			}
		})
	}
}
