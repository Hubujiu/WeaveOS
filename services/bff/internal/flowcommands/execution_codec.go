package flowcommands

// Declaration-only Root contract; implementation follows independently written tests.
type ExecutionStart struct {
 AllowWithdraw bool
 Approvers map[string][]string
}
type ExecutionPayload struct {
 EvidenceHash [32]byte
 Start *ExecutionStart
 Routes map[string]bool
}
type ExecutionTask struct {
 ID, NodeID, AssigneeID, EngineTaskID string
 ActivationEpoch int64
}
type ExecutionResult struct {
 InstanceID, EngineProcessID, State, Reason string
 SchemaVersion, RecordVersion int64
 Tasks []ExecutionTask
}
func EncodeExecutionPayload(action string, payload ExecutionPayload) ([]byte,error) {
 return nil, ErrInvalid
}
func DecodeExecutionResult(command Command, receipt Receipt, body []byte) (ExecutionResult,error) {
 return ExecutionResult{}, ErrInvalid
}
