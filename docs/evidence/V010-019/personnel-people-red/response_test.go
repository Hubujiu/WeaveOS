package personnel

import (
	"context"
	"encoding/json"
	"testing"
)

// Q25 full explicit arrays and the OpenAPI DTO must survive actual serialization.
func TestEmptyDefinitionRelationsSerializeAsArraysQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	access, err := f.app.Me(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range access.Identities {
		data, err := json.Marshal(identity)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(data, &raw)
		for _, field := range []string{"permissionCodes", "templateIds"} {
			if len(raw[field]) == 0 || string(raw[field]) == "null" {
				t.Fatalf("%s must remain an explicit array", field)
			}
		}
	}
}
