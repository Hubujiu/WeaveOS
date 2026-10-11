package apprecordservice

import (
	"encoding/json"
	"testing"
)

// V037 P3 / frozen OpenAPI: no_effect's sequence is required, including zero;
// pending has exactly four keys and may not pretend to have terminal fields.
func TestRootWorkflowResultJSONPreservesTerminalZeroAndPendingShape(t *testing.T) {
	base := WorkflowOperationResult{OperationID: "77777777-7777-4777-8777-777777777771", CommandID: "77777777-7777-4777-8777-777777777772", InstanceID: "77777777-7777-4777-8777-777777777773"}
	for _, tc := range []struct {
		name, status, state, reason string
		sequence                    int64
		keys                        int
	}{
		{"pending", "pending", "", "", 0, 4},
		{"success", "success", "completed", "", 1, 6},
		{"zero no effect", "no_effect", "unchanged", "instance_missing", 0, 7},
		{"nonzero no effect", "no_effect", "unchanged", "task_inactive", 2, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := base
			value.Status = tc.status
			value.InstanceState = tc.state
			value.Reason = tc.reason
			value.Sequence = tc.sequence
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]json.RawMessage
			if err = json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.keys {
				t.Errorf("closed %s result has %d keys; want %d: %s", tc.status, len(got), tc.keys, raw)
			}
			seq, ok := got["sequence"]
			if tc.status == "pending" {
				if ok {
					t.Error("pending includes terminal sequence")
				}
				return
			}
			if !ok {
				t.Fatal("terminal result missing required sequence")
			}
			var number int64
			if err = json.Unmarshal(seq, &number); err != nil || number != tc.sequence {
				t.Fatalf("sequence %s, want %d: %v", seq, tc.sequence, err)
			}
		})
	}
}
