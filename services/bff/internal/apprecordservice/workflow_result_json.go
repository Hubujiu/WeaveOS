package apprecordservice

import "encoding/json"

// Terminal sequence is required even when the command had no effect at zero.
// Keep pending's original four-key receipt unchanged; it has no result yet.
func (result WorkflowOperationResult) MarshalJSON() ([]byte, error) {
	type plain WorkflowOperationResult
	if result.Status != "success" && result.Status != "no_effect" {
		return json.Marshal(plain(result))
	}
	return json.Marshal(struct {
		plain
		Sequence int64 `json:"sequence"`
	}{plain: plain(result), Sequence: result.Sequence})
}
