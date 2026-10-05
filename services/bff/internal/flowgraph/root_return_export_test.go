package flowgraph

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

// This exports the existing production compiler output for the independent
// Flowable compatibility experiment. It does not implement return behavior.
func TestRootExportReturnCompatibilityFixtures(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		g := Graph{Version: 1, Nodes: []Node{
			{ID: rid(1), Kind: "start"},
			{ID: rid(2), Kind: "approval", Approval: &Approval{Mode: "all", AssigneeIDs: []string{rid(8), rid(9)}}},
			{ID: rid(3), Kind: "approval", Approval: &Approval{Mode: mode, AssigneeIDs: []string{rid(10), rid(11), rid(12)}}},
			{ID: rid(4), Kind: "end"},
		}, Edges: []Edge{{rid(1), rid(2), ""}, {rid(2), rid(3), ""}, {rid(3), rid(4), ""}}}
		raw, err := CompileBPMN(g, nil, rid(100))
		if err != nil {
			t.Fatal(err)
		}
		var doc rootXML
		if err = xml.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.names("userTask")) != 2 {
			t.Fatal("expected exactly two approval nodes")
		}
		if dir := os.Getenv("WEAVEOS_RETURN_BPMN_OUTPUT"); dir != "" {
			if !filepath.IsAbs(dir) {
				t.Fatal("artifact directory must be absolute")
			}
			if err = os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "return-"+mode+".bpmn20.xml"), raw, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
