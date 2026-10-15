package appworkflows

import (
	"encoding/json"
	"strings"
	"testing"
)

func rootNamedGraphJSON(name string) string {
	return `{"version":1,"nodes":[{"id":"69696969-6969-4696-8696-000000000001","kind":"start","name":` + name + `},{"id":"69696969-6969-4696-8696-000000000002","kind":"end"}],"edges":[{"from":"69696969-6969-4696-8696-000000000001","to":"69696969-6969-4696-8696-000000000002"}]}`
}
func TestRootNodeNamePublicGraphRoundtripPreservesLiteralAndEmptyDefault(t *testing.T) {
	for _, name := range []string{"", " 原名 <b>😀</b> ", strings.Repeat("😀", 100)} {
		raw, _ := json.Marshal(name)
		g, e := decodeGraph([]byte(rootNamedGraphJSON(string(raw))))
		if e != nil {
			t.Fatal("valid display metadata rejected", e)
		}
		if g.Nodes[0].Name != name {
			t.Fatal("decoded name changed")
		}
		b, e := json.Marshal(toGraphDTO(g))
		if e != nil {
			t.Fatal(e)
		}
		var out struct {
			Nodes []map[string]any `json:"nodes"`
		}
		json.Unmarshal(b, &out)
		if name == "" {
			if _, present := out.Nodes[0]["name"]; present {
				t.Fatal("empty name must omit metadata")
			}
		} else if out.Nodes[0]["name"] != name {
			t.Fatal("public read lost original label")
		}
	}
}
func TestRootNodeNamePublicGraphRejectsBadOptionalValues(t *testing.T) {
	for _, raw := range []string{`null`, `1`, `"\ud800"`, `"\udfff"`, `"\u0000"`, `"\u0085\u2003"`, `"` + strings.Repeat("x", 101) + `"`} {
		if _, e := decodeGraph([]byte(rootNamedGraphJSON(raw))); e == nil {
			t.Fatalf("invalid public name accepted %s", raw)
		}
	}
}
