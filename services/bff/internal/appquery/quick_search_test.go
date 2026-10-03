package appquery

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
)

func TestQuickSearchFrozenLiteralScopeAndCanonical(t *testing.T) {
	p := appaccess.Policy{ActorID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", OwnerID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", ResourceExists: true, Grants: []appaccess.Grant{
		{AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Action: appaccess.Read, Scope: appaccess.All, Fields: []string{fieldX}},
		{AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Action: appaccess.Read, Scope: appaccess.Own, Fields: []string{fieldN}},
	}}
	plan, err := CompileQuickSearch(json.RawMessage(`{"term":"  A%_\\é  ","fieldIds":["`+fieldX+`"]}`), fixtureFields[:2], p, 3)
	if err != nil || len(plan.Arguments) != 1 || plan.Arguments[0] != `a%_\é` || !strings.Contains(plan.Predicate, "strpos(translate(") || strings.Contains(plan.Predicate, "LIKE") {
		t.Fatalf("literal ASCII plan %+v %v", plan, err)
	}
	for _, raw := range []string{`null`, `{"term":"","fieldIds":["` + fieldX + `"]}`, `{"term":"a","fieldIds":["` + fieldX + `","` + fieldX + `"]}`} {
		if _, err := CompileQuickSearch(json.RawMessage(raw), fixtureFields[:2], p, 1); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid %s: %v", raw, err)
		}
	}
	if _, err := CompileQuickSearch(json.RawMessage(`{"term":"a","fieldIds":["`+fieldN+`"]}`), fixtureFields[:2], p, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("own-only field searched across all rows: %v", err)
	}
}
