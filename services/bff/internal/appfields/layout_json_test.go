package appfields

import (
	"encoding/json"
	"testing"
)

func TestEmptyGroupAndDescriptionRetainRequiredKeys(t *testing.T) {
	for _, node := range []LayoutNode{{ID: "10000000-0000-4000-8000-000000000001", Kind: "group", Title: "", Children: []LayoutNode{}, Span: 12}, {ID: "10000000-0000-4000-8000-000000000002", Kind: "description", Text: ""}} {
		raw, e := json.Marshal(node)
		if e != nil {
			t.Fatal(e)
		}
		var keys map[string]json.RawMessage
		json.Unmarshal(raw, &keys)
		if node.Kind == "group" {
			if string(keys["children"]) != "[]" || string(keys["title"]) != `""` {
				t.Fatalf("required group members lost %s", raw)
			}
		} else if string(keys["text"]) != `""` {
			t.Fatalf("required plain text lost %s", raw)
		}
	}
}
