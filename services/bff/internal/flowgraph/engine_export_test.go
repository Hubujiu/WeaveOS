package flowgraph

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestRootExportEngineCompatibilityFixtures(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		g := graph()
		g.Nodes[1].Approval.Mode = mode
		g.Nodes[1].Approval.AssigneeIDs = []string{rid(8), rid(9)}
		raw, e := CompileBPMN(g, fields(), rid(99))
		if e != nil {
			t.Fatal(e)
		}
		var document rootXML
		if e = xml.Unmarshal(raw, &document); e != nil {
			t.Fatal(e)
		}
		if len(document.names("userTask")) != 1 {
			t.Fatal("fixture must contain one two-person approval node")
		}
		if dir := os.Getenv("WEAVEOS_FLOW_BPMN_OUTPUT"); dir != "" {
			if !filepath.IsAbs(dir) {
				t.Fatal("artifact destination must be absolute")
			}
			if e = os.MkdirAll(dir, 0755); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, "generated-"+mode+".bpmn20.xml"), raw, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
}
