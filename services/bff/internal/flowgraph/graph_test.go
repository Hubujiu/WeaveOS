package flowgraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"reflect"
	"slices"
	"testing"
)

func rid(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }
func fields() []appquery.Field {
	return []appquery.Field{{ID: rid(6), Kind: appquery.Money}, {ID: rid(7), Kind: appquery.Text}}
}
func predicate(field, op, value string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"operator":"and","children":[{"fieldId":%q,"operator":%q,"value":%q}]}`, field, op, value))
}
func graph() Graph {
	return Graph{Version: 1, Nodes: []Node{
		{ID: rid(1), Kind: "start"},
		{ID: rid(2), Kind: "approval", Approval: &Approval{Mode: "all", AssigneeIDs: []string{rid(8)}, EditableFieldIDs: []string{rid(6)}}},
		{ID: rid(3), Kind: "condition", Condition: predicate(rid(7), "eq", "A")},
		{ID: rid(4), Kind: "end"}, {ID: rid(5), Kind: "end"}},
		Edges: []Edge{{rid(1), rid(2), ""}, {rid(2), rid(3), ""}, {rid(3), rid(4), "true"}, {rid(3), rid(5), "false"}}}
}
func clone(g Graph) Graph {
	out := g
	out.Nodes = slices.Clone(g.Nodes)
	out.Edges = slices.Clone(g.Edges)
	for i := range out.Nodes {
		out.Nodes[i].Condition = slices.Clone(g.Nodes[i].Condition)
		if source := g.Nodes[i].Approval; source != nil {
			value := *source
			value.AssigneeIDs = slices.Clone(source.AssigneeIDs)
			value.EditableFieldIDs = slices.Clone(source.EditableFieldIDs)
			out.Nodes[i].Approval = &value
		}
	}
	return out
}
func assertOrder(t *testing.T, g Graph, v Validated) {
	t.Helper()
	if len(v.Order) != len(g.Nodes) {
		t.Fatalf("order has %d nodes, want %d", len(v.Order), len(g.Nodes))
	}
	pos := map[string]int{}
	for i, id := range v.Order {
		if _, ok := pos[id]; ok {
			t.Fatal("duplicate order entry", id)
		}
		pos[id] = i
	}
	for _, n := range g.Nodes {
		if _, ok := pos[n.ID]; !ok {
			t.Fatal("missing node", n.ID)
		}
	}
	for _, e := range g.Edges {
		if pos[e.From] >= pos[e.To] {
			t.Fatalf("non-topological edge %+v", e)
		}
	}
}
func TestRootValidGraphAndDependencies(t *testing.T) {
	g := graph()
	v, e := Validate(g, fields())
	if e != nil {
		t.Fatal(e)
	}
	assertOrder(t, g, v)
	if !reflect.DeepEqual(v.ReferencedFields, []string{rid(6), rid(7)}) {
		t.Fatalf("dependencies %+v", v.ReferencedFields)
	}
}
func TestRootAllAnyAndMinimalGraph(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		g := graph()
		g.Nodes[1].Approval.Mode = mode
		g.Nodes[1].Approval.AssigneeIDs = []string{rid(8), rid(9)}
		if _, e := Validate(g, fields()); e != nil {
			t.Fatalf("%s: %v", mode, e)
		}
	}
	g := Graph{Version: 1, Nodes: []Node{{ID: rid(1), Kind: "start"}, {ID: rid(2), Kind: "end"}}, Edges: []Edge{{rid(1), rid(2), ""}}}
	if v, e := Validate(g, nil); e != nil {
		t.Fatal(e)
	} else {
		assertOrder(t, g, v)
	}
}
func TestRootGraphIdentityAndKindsFailClosed(t *testing.T) {
	edits := []func(*Graph){
		func(g *Graph) { g.Version = 0 }, func(g *Graph) { g.Nodes[0].ID = "invalid" },
		func(g *Graph) { g.Nodes[0].ID = "00000000-0000-0000-0000-000000000000" },
		func(g *Graph) { g.Nodes[1].ID = g.Nodes[0].ID }, func(g *Graph) { g.Nodes[1].Kind = "script" },
		func(g *Graph) { g.Nodes[1].Kind = "subprocess" }, func(g *Graph) { g.Nodes[1].Kind = "timer" },
		func(g *Graph) { g.Nodes[4].Kind = "start" }, func(g *Graph) { g.Nodes = nil }, func(g *Graph) { g.Edges = nil },
	}
	for i, edit := range edits {
		g := graph()
		edit(&g)
		if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
			t.Fatalf("%d invalid graph accepted: %v", i, e)
		}
	}
}
func TestRootEdgesAndBinaryBranches(t *testing.T) {
	edits := []func(*Graph){
		func(g *Graph) { g.Edges[0].From = rid(90) }, func(g *Graph) { g.Edges[0].To = rid(90) },
		func(g *Graph) { g.Edges[0].To = g.Edges[0].From },
		func(g *Graph) { g.Edges = append(g.Edges, g.Edges[0]) },
		func(g *Graph) { g.Edges[0].Branch = "true" }, func(g *Graph) { g.Edges[2].Branch = "" }, func(g *Graph) { g.Edges[3].Branch = "true" },
		func(g *Graph) { g.Edges = g.Edges[:3] },
		func(g *Graph) { g.Edges = append(g.Edges, Edge{rid(4), rid(5), ""}) },
		func(g *Graph) { g.Edges = append(g.Edges, Edge{rid(2), rid(5), ""}) },
		func(g *Graph) { g.Edges = append(g.Edges, Edge{rid(5), rid(1), ""}) },
	}
	for i, edit := range edits {
		g := graph()
		edit(&g)
		if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
			t.Fatalf("%d invalid edges accepted: %v", i, e)
		}
	}
}
func TestRootReachableCycleRejected(t *testing.T) {
	g := graph()
	g.Nodes = g.Nodes[:4]
	g.Edges = []Edge{{rid(1), rid(2), ""}, {rid(2), rid(3), ""}, {rid(3), rid(2), "true"}, {rid(3), rid(4), "false"}}
	if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
		t.Fatalf("reachable cycle accepted: %v", e)
	}
}
func TestRootDisconnectedNodesRejected(t *testing.T) {
	g := graph()
	g.Nodes = append(g.Nodes, Node{ID: rid(90), Kind: "end"})
	if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
		t.Fatalf("unreachable node accepted: %v", e)
	}
}
func TestRootApprovalConfiguration(t *testing.T) {
	edits := []func(*Graph){
		func(g *Graph) { g.Nodes[1].Approval = nil }, func(g *Graph) { g.Nodes[1].Approval.Mode = "majority" },
		func(g *Graph) { g.Nodes[1].Approval.AssigneeIDs = nil },
		func(g *Graph) { g.Nodes[1].Approval.AssigneeIDs = []string{rid(8), rid(8)} },
		func(g *Graph) { g.Nodes[1].Approval.AssigneeIDs = []string{"00000000-0000-0000-0000-000000000000"} },
		func(g *Graph) { g.Nodes[1].Approval.EditableFieldIDs = []string{rid(90)} },
		func(g *Graph) { g.Nodes[1].Approval.EditableFieldIDs = []string{rid(6), rid(6)} },
		func(g *Graph) { g.Nodes[1].Condition = predicate(rid(7), "eq", "A") },
		func(g *Graph) { g.Nodes[0].Approval = g.Nodes[1].Approval },
		func(g *Graph) { g.Nodes[4].Condition = predicate(rid(7), "eq", "A") },
		func(g *Graph) { g.Nodes[2].Approval = g.Nodes[1].Approval },
	}
	for i, edit := range edits {
		g := clone(graph())
		edit(&g)
		if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
			t.Fatalf("%d invalid config accepted: %v", i, e)
		}
	}
}
func TestRootUnifiedConditionsRejectInvalidTypes(t *testing.T) {
	cases := []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage("{}"), predicate(rid(7), "gt", "A"), predicate(rid(90), "eq", "A"), json.RawMessage(`{"operator":"and","children":[]}`), json.RawMessage(`{"operator":"and","operator":"or","children":[]}`), json.RawMessage(`{"script":"return true"}`)}
	for i, condition := range cases {
		g := graph()
		g.Nodes[2].Condition = condition
		if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
			t.Fatalf("%d invalid condition accepted: %v", i, e)
		}
	}
}
func TestRootNestedAndOrAndExactDecimal(t *testing.T) {
	g := graph()
	g.Nodes[2].Condition = json.RawMessage(fmt.Sprintf(`{"operator":"or","children":[{"operator":"and","children":[{"fieldId":%q,"operator":"gt","value":"9007199254740993.01"},{"fieldId":%q,"operator":"neq","value":"B"}]},{"fieldId":%q,"operator":"eq","value":null}]}`, rid(6), rid(7), rid(6)))
	v, e := Validate(g, fields())
	if e != nil {
		t.Fatal(e)
	}
	assertOrder(t, g, v)
	if !reflect.DeepEqual(v.ReferencedFields, []string{rid(6), rid(7)}) {
		t.Fatal(v.ReferencedFields)
	}
}
func TestRootFieldContextAlwaysValidated(t *testing.T) {
	g := Graph{Version: 1, Nodes: []Node{{ID: rid(1), Kind: "start"}, {ID: rid(2), Kind: "end"}}, Edges: []Edge{{rid(1), rid(2), ""}}}
	for _, fs := range [][]appquery.Field{{{ID: rid(6), Kind: "script"}}, {{ID: rid(6), Kind: appquery.Text}, {ID: rid(6), Kind: appquery.Text}}, {{ID: "00000000-0000-0000-0000-000000000000", Kind: appquery.Text}}} {
		if _, e := Validate(g, fs); !errors.Is(e, ErrInvalid) {
			t.Fatalf("invalid field context accepted: %v", e)
		}
	}
}
func TestRootDeterministicAndNoInputMutation(t *testing.T) {
	g := graph()
	before := clone(g)
	if !reflect.DeepEqual(g, before) {
		t.Fatal("test snapshot changed the graph before validation")
	}
	fs := fields()
	want, e := Validate(g, fs)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		got, e := Validate(g, fs)
		if e != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("unstable result %+v %v", got, e)
		}
	}
	if !reflect.DeepEqual(g, before) || !reflect.DeepEqual(fs, fields()) {
		t.Fatal("input mutated")
	}
	want.Order[0] = rid(99)
	want.ReferencedFields[0] = rid(99)
	if !reflect.DeepEqual(g, before) {
		t.Fatal("result aliases input")
	}
}
func TestRootResourceLimits(t *testing.T) {
	makeChain := func(n int) Graph {
		g := Graph{Version: 1}
		for i := 1; i <= n; i++ {
			node := Node{ID: rid(i), Kind: "approval", Approval: &Approval{Mode: "all", AssigneeIDs: []string{rid(900)}}}
			if i == 1 {
				node.Kind = "start"
				node.Approval = nil
			}
			if i == n {
				node.Kind = "end"
				node.Approval = nil
			}
			g.Nodes = append(g.Nodes, node)
			if i > 1 {
				g.Edges = append(g.Edges, Edge{rid(i - 1), rid(i), ""})
			}
		}
		return g
	}
	if _, e := Validate(makeChain(100), fields()); e != nil {
		t.Fatalf("100-node boundary: %v", e)
	}
	if _, e := Validate(makeChain(101), fields()); !errors.Is(e, ErrInvalid) {
		t.Fatalf("101 nodes: %v", e)
	}
	g := graph()
	for i := 0; i < 201; i++ {
		g.Edges = append(g.Edges, g.Edges[0])
	}
	if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
		t.Fatal("edge limit")
	}
	g = graph()
	g.Nodes[1].Approval.AssigneeIDs = nil
	for i := 100; i < 150; i++ {
		g.Nodes[1].Approval.AssigneeIDs = append(g.Nodes[1].Approval.AssigneeIDs, rid(i))
	}
	if _, e := Validate(g, fields()); e != nil {
		t.Fatalf("50 assignees: %v", e)
	}
	g.Nodes[1].Approval.AssigneeIDs = append(g.Nodes[1].Approval.AssigneeIDs, rid(150))
	if _, e := Validate(g, fields()); !errors.Is(e, ErrInvalid) {
		t.Fatal("assignee limit")
	}
	fs := []appquery.Field{}
	for i := 1; i <= 201; i++ {
		fs = append(fs, appquery.Field{ID: rid(i), Kind: appquery.Text})
	}
	if _, e := Validate(makeChain(2), fs); !errors.Is(e, ErrInvalid) {
		t.Fatal("field limit")
	}
}
