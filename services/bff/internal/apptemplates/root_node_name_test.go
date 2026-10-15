package apptemplates

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"strings"
	"testing"
)

func TestRootNodeNameTemplateDecodeAndNewIdentityRemapPreserveLabels(t *testing.T) {
	original := rootManifest()
	var raw map[string]any
	json.Unmarshal(mbytes(t, original), &raw)
	nodes := raw["workflows"].([]any)[0].(map[string]any)["graph"].(map[string]any)["nodes"].([]any)
	for _, n := range nodes {
		n.(map[string]any)["name"] = "模板原节点😀"
	}
	b, _ := json.Marshal(raw)
	decoded, e := DecodeManifest(b)
	if e != nil {
		t.Fatal("named complete template rejected", e)
	}
	mapped, e := RemapInternalIDs(decoded, rootNewIDs())
	if e != nil {
		t.Fatal(e)
	}
	var output map[string]any
	json.Unmarshal(mbytes(t, mapped), &output)
	out := output["workflows"].([]any)[0].(map[string]any)["graph"].(map[string]any)["nodes"].([]any)
	for i, n := range out {
		v := n.(map[string]any)
		if v["name"] != "模板原节点😀" || v["id"] == nodes[i].(map[string]any)["id"] {
			t.Fatal("copy lost name or reused original identity", v)
		}
	}
}
func TestRootNodeNameStoredGraphExportDoesNotDropLabels(t *testing.T) {
	g := flowgraph.Graph{Version: 1, Nodes: []flowgraph.Node{{ID: mid(1), Kind: "start", Name: " 原开始 "}, {ID: mid(2), Kind: "end", Name: "原结束"}}, Edges: []flowgraph.Edge{{From: mid(1), To: mid(2)}}}
	raw, e := json.Marshal(fromGraph(g))
	if e != nil {
		t.Fatal(e)
	}
	var out struct {
		Nodes []map[string]any `json:"nodes"`
	}
	json.Unmarshal(raw, &out)
	if out.Nodes[0]["name"] != " 原开始 " || out.Nodes[1]["name"] != "原结束" {
		t.Fatal("export silently dropped stored display metadata")
	}
}

func TestRootNodeNameTemplateRejectsInvalidDisplayMetadata(t *testing.T) {
	for _, name := range []any{nil, 1, "\x00", "\n\u0085", strings.Repeat("x", 101)} {
		m := rootManifest()
		var raw map[string]any
		json.Unmarshal(mbytes(t, m), &raw)
		node := raw["workflows"].([]any)[0].(map[string]any)["graph"].(map[string]any)["nodes"].([]any)[1].(map[string]any)
		node["name"] = name
		b, _ := json.Marshal(raw)
		if _, e := DecodeManifest(b); e == nil {
			t.Fatal("invalid template display name accepted")
		}
	}
}
