package personnel

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// Q25 full explicit arrays and the OpenAPI DTO must survive actual serialization.
func TestEmptyDefinitionRelationsSerializeAsArraysQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.owner.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	access, err := f.app.Me(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(access.Identities) != 2 {
		t.Fatalf("expected both fixture identities, got %+v", access.Identities)
	}
	seen := map[string]bool{}
	for _, identity := range access.Identities {
		if seen[identity.ID] || (identity.ID != f.i1 && identity.ID != f.i2) {
			t.Fatalf("unexpected or duplicate identity: %s", identity.ID)
		}
		seen[identity.ID] = true
		data, err := json.Marshal(identity)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		expected := map[string][]string{"permissionCodes": {}, "templateIds": {}}
		if identity.ID == f.i1 {
			expected["permissionCodes"] = []string{"app.test.A"}
			expected["templateIds"] = []string{f.template}
		}
		for field, want := range expected {
			var got []string
			if err := json.Unmarshal(raw[field], &got); err != nil {
				t.Fatalf("%s must be an array of strings: %v", field, err)
			}
			sort.Strings(got)
			if got == nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("identity %s %s = %#v, want %#v", identity.ID, field, got, want)
			}
		}
	}
}
