package flowgraph

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRootNodeNameLegacyEncodingAndNonexecutingBPMN(t *testing.T) {
	g := graph()
	type oldNode struct {
		ID        string
		Kind      string
		Approval  *Approval
		Condition json.RawMessage
	}
	type oldGraph struct {
		Version int
		Nodes   []oldNode
		Edges   []Edge
	}
	old := oldGraph{Version: g.Version, Edges: g.Edges, Nodes: []oldNode{}}
	for _, n := range g.Nodes {
		old.Nodes = append(old.Nodes, oldNode{n.ID, n.Kind, n.Approval, n.Condition})
	}
	oldBytes, _ := json.Marshal(old)
	current, _ := json.Marshal(g)
	if !bytes.Equal(oldBytes, current) {
		t.Fatal("missing labels changed legacy stored graph bytes")
	}
	original, e := CompileScopedBPMN(g, fields(), rid(98), rid(99))
	if e != nil {
		t.Fatal(e)
	}
	for i := range g.Nodes {
		g.Nodes[i].Name = " 原文 <script> ${never_execute} 😀 "
	}
	named, e := CompileScopedBPMN(g, fields(), rid(98), rid(99))
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(original, named) {
		t.Fatal("display names changed executable BPMN")
	}
	g.Nodes[1].Name = strings.Repeat("😀", 100)
	if _, e = Validate(g, fields()); e != nil {
		t.Fatal("100 scalar label rejected", e)
	}
}
func TestRootNodeNameInvalidTextFailsGraphValidation(t *testing.T) {
	for _, name := range []string{strings.Repeat("😀", 101), strings.Repeat("x", 101), "\x00", "\n\u0085\u2003", string([]byte{255})} {
		g := graph()
		g.Nodes[1].Name = name
		if _, e := Validate(g, fields()); e == nil {
			t.Fatalf("invalid display name accepted: %q", name)
		}
	}
}
func TestRootNodeNameRawJSONPreservesScalarsAndRejectsLossyEscapes(t *testing.T) {
	for _, c := range []struct{ raw, want string }{{`""`, ""}, {`" 财务审批 "`, " 财务审批 "}, {`"\ud83d\ude00"`, "😀"}, {`"\uFFFD"`, "�"}, {`"\\ud800"`, `\ud800`}} {
		got, e := DecodeNodeNameJSON([]byte(c.raw))
		if e != nil || got != c.want {
			t.Fatalf("valid name changed or rejected %s: %q %v", c.raw, got, e)
		}
	}
	for _, raw := range []string{`null`, `1`, `false`, `{}`, `"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\u0000"`, `"  "`, `"x" "y"`, string([]byte{'"', 255, '"'})} {
		if _, e := DecodeNodeNameJSON([]byte(raw)); e == nil {
			t.Fatalf("invalid raw name accepted %s", raw)
		}
	}
}
