package appstructure

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestRootWorkflowNodeNameRealHTTPVersionsKeepOriginalLabelsAndExecutionBytes(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	flow := uuid(t, s.f.owner)
	body := rootWorkflowHTTPBody(t, s, s.f.actor)
	nodes := body["graph"].(map[string]any)["nodes"].([]any)
	nodes[1].(map[string]any)["name"] = "原审批😀"
	originalBody, _ := json.Marshal(body)
	first := data(t, rootWorkflowHTTPCall(t, s, flow, "PUT", "definition", body), 201)
	if first["candidateVersion"] != float64(1) {
		t.Fatal(first)
	}
	for version, name := range []string{"原审批😀", "新审批名称", ""} {
		if version > 0 {
			body["operationId"] = uuid(t, s.f.owner)
			body["expectedRevision"] = version
			nodes[1].(map[string]any)["name"] = name
			data(t, rootWorkflowHTTPCall(t, s, flow, "PUT", "definition", body), 200)
		}
		read := data(t, rootWorkflowHTTPCall(t, s, flow, "GET", "definition", nil), 200)
		node := read["graph"].(map[string]any)["nodes"].([]any)[1].(map[string]any)
		if name == "" {
			if _, present := node["name"]; present {
				t.Fatal("clear did not restore implicit type name")
			}
		} else if node["name"] != name {
			t.Fatal("current definition lost label", node)
		}
	}
	rows, e := s.f.owner.Query(context.Background(), `SELECT version,graph_json,bpmn_xml FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 ORDER BY version`, s.f.app, flow)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	count := 0
	var firstXML []byte
	for rows.Next() {
		var version int
		var graphRaw []byte
		var xml string
		if e = rows.Scan(&version, &graphRaw, &xml); e != nil {
			t.Fatal(e)
		}
		var graph struct{ Nodes []map[string]any }
		if e = json.Unmarshal(graphRaw, &graph); e != nil {
			t.Fatal(e)
		}
		expected := []string{"原审批😀", "新审批名称", ""}[count]
		if expected == "" {
			if _, present := graph.Nodes[1]["Name"]; present {
				t.Fatal("stored clear name not omitted")
			}
		} else if graph.Nodes[1]["Name"] != expected {
			t.Fatal("older immutable version was changed")
		}
		if count == 0 {
			firstXML = []byte(xml)
		} else if !bytes.Equal(firstXML, []byte(xml)) {
			t.Fatal("renaming changed executable XML")
		}
		count++
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	if count != 3 {
		t.Fatalf("got %d immutable versions", count)
	}
	var replay map[string]any
	json.Unmarshal(originalBody, &replay)
	data(t, rootWorkflowHTTPCall(t, s, flow, "PUT", "definition", replay), 201)
	var after int
	if e = s.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2`, s.f.app, flow).Scan(&after); e != nil || after != 3 {
		t.Fatal("replay created another version", after, e)
	}
}
