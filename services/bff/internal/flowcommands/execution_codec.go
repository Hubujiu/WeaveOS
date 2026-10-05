package flowcommands

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"unicode"
	"unicode/utf8"
)

type ExecutionStart struct {
	AllowWithdraw bool
	Approvers     map[string][]string
}
type ExecutionPayload struct {
	EvidenceHash [32]byte
	Start        *ExecutionStart
	Routes       map[string]bool
}
type ExecutionTask struct {
	ID, NodeID, AssigneeID, EngineTaskID string
	ActivationEpoch                      int64
}
type ExecutionResult struct {
	InstanceID, EngineProcessID, State, Reason string
	SchemaVersion, RecordVersion               int64
	Tasks                                      []ExecutionTask
}

// EncodeExecutionPayload returns owned wire bytes without modifying caller maps or rosters.
func EncodeExecutionPayload(action string, payload ExecutionPayload) ([]byte, error) {
	switch action {
	case "start":
		if payload.Start == nil {
			return nil, ErrInvalid
		}
	case "agree", "reject", "withdraw", "return":
		if payload.Start != nil {
			return nil, ErrInvalid
		}
	default:
		return nil, ErrInvalid
	}
	if !isNonzeroHash(payload.EvidenceHash) || len(payload.Routes) > 100 {
		return nil, ErrInvalid
	}
	// Validate counts and identities before any input-sized copies or output allocation.
	size := 8 + 32 + 1 + 4
	if payload.Start != nil {
		if len(payload.Start.Approvers) > 100 {
			return nil, ErrInvalid
		}
		size += 1 + 4
		for node, actors := range payload.Start.Approvers {
			if !isCanonicalNonzeroUUID(node) || len(actors) < 1 || len(actors) > 50 {
				return nil, ErrInvalid
			}
			seen := make(map[string]struct{}, len(actors))
			for _, actor := range actors {
				if !isCanonicalNonzeroUUID(actor) {
					return nil, ErrInvalid
				}
				if _, ok := seen[actor]; ok {
					return nil, ErrInvalid
				}
				seen[actor] = struct{}{}
			}
			size += 4 + 36 + 4 + len(actors)*(4+36)
		}
	}
	for node := range payload.Routes {
		if !isCanonicalNonzeroUUID(node) {
			return nil, ErrInvalid
		}
		size += 4 + 36 + 1
	}
	if size > 262144 {
		return nil, ErrInvalid
	}
	out := make([]byte, 0, size)
	out = append(out, 'W', 'V', 'F', 'P', 'A', 'Y', 0, 1)
	out = append(out, payload.EvidenceHash[:]...)
	out = appendExecutionBool(out, payload.Start != nil)
	if payload.Start != nil {
		out = appendExecutionBool(out, payload.Start.AllowWithdraw)
		nodes := make([]string, 0, len(payload.Start.Approvers))
		for node := range payload.Start.Approvers {
			nodes = append(nodes, node)
		}
		slices.Sort(nodes)
		out = binary.BigEndian.AppendUint32(out, uint32(len(nodes)))
		for _, node := range nodes {
			out = appendExecutionText(out, node)
			actors := slices.Clone(payload.Start.Approvers[node])
			slices.Sort(actors)
			out = binary.BigEndian.AppendUint32(out, uint32(len(actors)))
			for _, actor := range actors {
				out = appendExecutionText(out, actor)
			}
		}
	}
	nodes := make([]string, 0, len(payload.Routes))
	for node := range payload.Routes {
		nodes = append(nodes, node)
	}
	slices.Sort(nodes)
	out = binary.BigEndian.AppendUint32(out, uint32(len(nodes)))
	for _, node := range nodes {
		out = appendExecutionText(out, node)
		out = appendExecutionBool(out, payload.Routes[node])
	}
	return out, nil
}

func appendExecutionText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
func appendExecutionBool(out []byte, value bool) []byte {
	if value {
		return append(out, 1)
	}
	return append(out, 0)
}

// DecodeExecutionResult validates existing receipt rules and the exact bound result body.
// An unchanged result is a no-effect verdict, not an instance projection.
func DecodeExecutionResult(command Command, receipt Receipt, body []byte) (ExecutionResult, error) {
	if command.ProtocolVersion != 2 {
		return ExecutionResult{}, ErrInvalid
	}
	if _, err := PlanReceipt(command, receipt, command.ExpectedSequence, nil); err != nil {
		return ExecutionResult{}, err
	}
	if len(body) < 8 || len(body) > 65536 {
		return ExecutionResult{}, ErrInvalid
	}
	if sha256.Sum256(body) != receipt.ResultHash {
		return ExecutionResult{}, ErrConflict
	}
	if string(body[:8]) != "WVFRSL\x00\x01" {
		return ExecutionResult{}, ErrInvalid
	}
	reader := executionResultReader{body: body, offset: 8}
	result := ExecutionResult{
		InstanceID: reader.text(36), EngineProcessID: reader.text(200),
		State: reader.text(9), Reason: reader.text(32),
	}
	result.SchemaVersion = reader.positiveInteger()
	result.RecordVersion = reader.positiveInteger()
	count := reader.uint32()
	// Each task requires at least three UUID strings, a nonempty engine ID, and an epoch.
	if reader.invalid || count > 50 || uint64(count)*133 > uint64(len(body)-reader.offset) {
		return ExecutionResult{}, ErrInvalid
	}
	if count > 0 {
		result.Tasks = make([]ExecutionTask, int(count))
	}
	engines := make(map[string]struct{}, int(count))
	assignees := make(map[string]struct{}, int(count))
	for i := range result.Tasks {
		task := ExecutionTask{ID: reader.text(36), NodeID: reader.text(36), AssigneeID: reader.text(36), EngineTaskID: reader.text(200)}
		task.ActivationEpoch = reader.positiveInteger()
		if reader.invalid || !isCanonicalNonzeroUUID(task.ID) || !isCanonicalNonzeroUUID(task.NodeID) ||
			!isCanonicalNonzeroUUID(task.AssigneeID) || !validExecutionEngineID(task.EngineTaskID) {
			return ExecutionResult{}, ErrInvalid
		}
		if i > 0 && (task.ID <= result.Tasks[i-1].ID || task.NodeID != result.Tasks[0].NodeID || task.ActivationEpoch != result.Tasks[0].ActivationEpoch) {
			return ExecutionResult{}, ErrInvalid
		}
		if _, ok := engines[task.EngineTaskID]; ok {
			return ExecutionResult{}, ErrInvalid
		}
		engines[task.EngineTaskID] = struct{}{}
		if _, ok := assignees[task.AssigneeID]; ok {
			return ExecutionResult{}, ErrInvalid
		}
		assignees[task.AssigneeID] = struct{}{}
		result.Tasks[i] = task
	}
	if reader.invalid || reader.offset != len(body) || !isCanonicalNonzeroUUID(result.InstanceID) {
		return ExecutionResult{}, ErrInvalid
	}
	switch result.State {
	case "active":
		if len(result.Tasks) == 0 || !validExecutionEngineID(result.EngineProcessID) || result.Reason != "" {
			return ExecutionResult{}, ErrInvalid
		}
	case "completed", "rejected", "withdrawn":
		if len(result.Tasks) != 0 || !validExecutionEngineID(result.EngineProcessID) || result.Reason != "" {
			return ExecutionResult{}, ErrInvalid
		}
	case "unchanged":
		if len(result.Tasks) != 0 || result.EngineProcessID != "" || !validExecutionReason(result.Reason) {
			return ExecutionResult{}, ErrInvalid
		}
	default:
		return ExecutionResult{}, ErrInvalid
	}
	if result.InstanceID != command.InstanceID || result.SchemaVersion != command.SchemaVersion || result.RecordVersion != command.RecordVersion ||
		(receipt.Outcome == "no_effect") != (result.State == "unchanged") {
		return ExecutionResult{}, ErrConflict
	}
	return result, nil
}

// The reader bounds declared lengths before conversions and allocations.
type executionResultReader struct {
	body    []byte
	offset  int
	invalid bool
}

func (r *executionResultReader) uint32() uint32 {
	if r.invalid || len(r.body)-r.offset < 4 {
		r.invalid = true
		return 0
	}
	value := binary.BigEndian.Uint32(r.body[r.offset : r.offset+4])
	r.offset += 4
	return value
}
func (r *executionResultReader) text(limit uint32) string {
	size := r.uint32()
	if r.invalid || size > limit || uint64(size) > uint64(len(r.body)-r.offset) {
		r.invalid = true
		return ""
	}
	raw := r.body[r.offset : r.offset+int(size)]
	r.offset += int(size)
	if !utf8.Valid(raw) {
		r.invalid = true
		return ""
	}
	return string(raw)
}
func (r *executionResultReader) positiveInteger() int64 {
	if r.invalid || len(r.body)-r.offset < 8 {
		r.invalid = true
		return 0
	}
	value := binary.BigEndian.Uint64(r.body[r.offset : r.offset+8])
	r.offset += 8
	if value < 1 || value > uint64(maxSafeInteger) {
		r.invalid = true
		return 0
	}
	return int64(value)
}
func validExecutionEngineID(value string) bool {
	if len(value) < 1 || len(value) > 200 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
func validExecutionReason(reason string) bool {
	switch reason {
	case "instance_missing", "instance_exists", "deployment_missing", "deployment_mismatch", "scope_mismatch", "terminal_instance",
		"stale_sequence", "stale_fence", "stale_schema_version", "stale_record_version", "task_missing", "task_inactive",
		"task_epoch_mismatch", "actor_mismatch", "withdrawal_forbidden", "return_target_unvisited", "return_target_forbidden", "cancelled":
		return true
	default:
		return false
	}
}
