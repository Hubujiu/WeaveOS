package flowgraph

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

const scopedAppA = "11111111-1111-4111-8111-111111111111"
const scopedAppB = "22222222-2222-4222-8222-222222222222"
const scopedFlow = "33333333-3333-4333-8333-333333333333"

func rootScopedGraph(approval bool) Graph {
	const start = "44444444-4444-4444-8444-444444444444"
	const end = "55555555-5555-4555-8555-555555555555"
	const task = "66666666-6666-4666-8666-666666666666"
	g := Graph{Version: 1, Nodes: []Node{{ID: start, Kind: "start"}, {ID: end, Kind: "end"}}, Edges: []Edge{{From: start, To: end}}}
	if approval {
		g.Nodes = append(g.Nodes, Node{ID: task, Kind: "approval", Approval: &Approval{Mode: "all", AssigneeIDs: []string{scopedAppA}}})
		g.Edges = []Edge{{From: start, To: task}, {From: task, To: end}}
	}
	return g
}

func rootProcessKey(t *testing.T, body []byte) string {
	t.Helper()
	var document struct {
		Processes []struct {
			ID string `xml:"id,attr"`
		} `xml:"process"`
	}
	if err := xml.Unmarshal(body, &document); err != nil || len(document.Processes) != 1 {
		t.Fatalf("expected one actual BPMN process: %v", err)
	}
	return document.Processes[0].ID
}

// User-approved V038: a publication changes the definition, not the identity
// under which Flowable allocates native versions. Expectations are fixed IDs.
func TestRootScopedCompilerKeepsFlowIdentityAcrossRevisions(t *testing.T) {
	const want = "p_11111111111141118111111111111111_33333333333343338333333333333333"
	var prior []byte
	for _, approval := range []bool{false, true} {
		body, err := CompileScopedBPMN(rootScopedGraph(approval), nil, scopedAppA, scopedFlow)
		if err != nil {
			t.Fatalf("valid scoped flow must compile: %v", err)
		}
		if got := rootProcessKey(t, body); got != want {
			t.Fatalf("process identity changed with revision: got %q want %q", got, want)
		}
		if prior != nil && string(prior) == string(body) {
			t.Fatal("structural revision was lost")
		}
		prior = body
	}
}

func TestRootScopedCompilerSeparatesAppsWithSameFlowID(t *testing.T) {
	a, err := CompileScopedBPMN(rootScopedGraph(false), nil, scopedAppA, scopedFlow)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CompileScopedBPMN(rootScopedGraph(false), nil, scopedAppB, scopedFlow)
	if err != nil {
		t.Fatal(err)
	}
	if rootProcessKey(t, a) == rootProcessKey(t, b) {
		t.Fatal("different applications share an engine version sequence")
	}
	if got := rootProcessKey(t, b); got != "p_22222222222242228222222222222222_33333333333343338333333333333333" {
		t.Fatalf("unexpected independent application identity: %s", got)
	}
}

func TestRootScopedCompilerRejectsInvalidScopeBeforeProducingXML(t *testing.T) {
	for _, invalid := range []string{"", "00000000-0000-0000-0000-000000000000", strings.ToUpper("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"), scopedAppA + "\""} {
		for _, app := range []bool{true, false} {
			a, f := scopedAppA, scopedFlow
			if app {
				a = invalid
			} else {
				f = invalid
			}
			body, err := CompileScopedBPMN(rootScopedGraph(false), nil, a, f)
			if !errors.Is(err, ErrInvalid) || len(body) != 0 {
				t.Fatalf("invalid scope produced executable XML: %q %v", body, err)
			}
		}
	}
}

func TestRootScopedCompilerPreservesLegacyVersionKeyEntryPoint(t *testing.T) {
	body, err := CompileBPMN(rootScopedGraph(false), nil, scopedFlow)
	if err != nil {
		t.Fatal(err)
	}
	if got := rootProcessKey(t, body); got != "p_33333333333343338333333333333333" {
		t.Fatalf("legacy artifacts must retain their original identity: %s", got)
	}
}
