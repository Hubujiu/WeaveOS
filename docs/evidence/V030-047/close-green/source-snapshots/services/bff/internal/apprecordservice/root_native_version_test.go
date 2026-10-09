package apprecordservice

import (
	"encoding/xml"
	"strings"
	"testing"
)

// The expected scope is the approved contract, not output from the compiler.
func rootNativeStored(t *testing.T, f recordFixture, flow string, version int64) (string, string, string) {
	t.Helper()
	var id, raw string
	if err := f.runtime.QueryRow(f.ctx, `SELECT version_id::text,bpmn_xml FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=$3`, f.app, flow, version).Scan(&id, &raw); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Process []struct {
			ID string `xml:"id,attr"`
		} `xml:"process"`
	}
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Process) != 1 {
		t.Fatalf("want exactly one persisted process, got %d", len(doc.Process))
	}
	return id, raw, doc.Process[0].ID
}
func TestRootNativeCatalogStableKeyAndImmutablePriorVersion(t *testing.T) {
	f := newRecordFixture(t)
	graph := rootCatalogGraph(t, f, true)
	flow := recordOperationID(t, f)
	first := rootCatalogPut(t, f, flow, 0, graph)
	id1, bytes1, key1 := rootNativeStored(t, f, flow, first.CandidateVersion)
	graph.Nodes[1].Approval.Mode = "any"
	second := rootCatalogPut(t, f, flow, first.Revision, graph)
	id2, bytes2, key2 := rootNativeStored(t, f, flow, second.CandidateVersion)
	oldID, oldBytes, oldKey := rootNativeStored(t, f, flow, first.CandidateVersion)
	if first.CandidateVersion != 1 || second.CandidateVersion != 2 || id1 == id2 {
		t.Fatal("business revision identities were lost")
	}
	if id1 != oldID || bytes1 != oldBytes || key1 != oldKey {
		t.Fatal("saving a new revision rewrote the prior persisted definition")
	}
	if bytes1 == bytes2 {
		t.Fatal("changed approval semantics were not persisted")
	}
	expected := "p_" + strings.ReplaceAll(f.app, "-", "") + "_" + strings.ReplaceAll(flow, "-", "")
	if key1 != expected || key2 != expected {
		t.Fatalf("native process scope must stay %s across revisions; got %s and %s", expected, key1, key2)
	}
}
func TestRootNativeCatalogBindsEachApplicationScope(t *testing.T) {
	a, b := newRecordFixture(t), newRecordFixture(t)
	if a.app == b.app {
		t.Fatal("independent application fixtures required")
	}
	keys := make([]string, 0, 2)
	for _, f := range []recordFixture{a, b} {
		flow := recordOperationID(t, f)
		h := rootCatalogPut(t, f, flow, 0, rootCatalogGraph(t, f, false))
		_, _, key := rootNativeStored(t, f, flow, h.CandidateVersion)
		expected := "p_" + strings.ReplaceAll(f.app, "-", "") + "_" + strings.ReplaceAll(flow, "-", "")
		if key != expected {
			t.Errorf("persisted native key must bind app/flow %s, got %s", expected, key)
		}
		keys = append(keys, key)
	}
	if keys[0] == keys[1] {
		t.Fatal("different applications share a process identity")
	}
}
