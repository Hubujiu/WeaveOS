package main

import (
	"encoding/json"
	"testing"
)

// P25: real Session/TLS/handler/transaction test. Original human text is not
// a machine-result reason and must survive original-key replay unchanged.
func TestRootWorkflowReasonHTTPSPreservesOriginalAndReplay(t *testing.T) {
	for _, action := range []string{"reject", "return"} {
		t.Run(action, func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			path, previewPath := f.taskPath()+"/actions", f.taskPath()
			if action == "return" {
				path = rootLifecycleHTTPPath(f) + "/actions"
				previewPath = rootLifecycleHTTPPath(f) + "/lifecycle"
			}
			preview := rootHTTPData(t, f.call(t, "GET", previewPath, "", nil), 200)
			const original = "  请补充凭证\n保留原文 🦊  "
			op := f.id(t)
			body := map[string]string{"operationId": op, "action": action, "basisToken": rootHTTPString(t, preview, "basisToken"), "reason": original}
			if action == "return" {
				body["targetNodeId"] = f.node
			}
			raw, e := json.Marshal(body)
			if e != nil {
				t.Fatal(e)
			}
			first := rootHTTPData(t, f.call(t, "POST", path, string(raw), nil), 202)
			if len(first) != 4 {
				t.Fatal("pending response leaked human reason or other fields")
			}
			command := rootHTTPString(t, first, "commandId")
			var got string
			if e = f.runtime.QueryRow(f.ctx, "SELECT reason FROM applications.workflow_action_reasons WHERE command_id=$1", command).Scan(&got); e != nil || got != original {
				t.Fatal("original reason not retained", e)
			}
			again := rootHTTPData(t, f.call(t, "POST", path, string(raw), nil), 202)
			if rootHTTPString(t, again, "commandId") != command {
				t.Fatal("replay changed original command")
			}
		})
	}
}
