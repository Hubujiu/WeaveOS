package flowcommands

import "errors"

var (
 ErrInvalid = errors.New("invalid workflow command")
 ErrConflict = errors.New("workflow receipt conflict")
 ErrOutOfOrder = errors.New("workflow sequence conflict")
 errNotImplemented = errors.New("root RED-only declaration: not implemented")
)

// Command is a trusted internal envelope, never an authorization result.
type Command struct {
 ProtocolVersion int
 CommandID, AppID, TableID, RecordID, InstanceID, TaskID, ActorID, Action string
 RecordVersion, FenceEpoch, TaskEpoch, ExpectedSequence int64
 PayloadHash [32]byte
}
type Receipt struct {
 CommandID string
 CommandHash [32]byte
 Outcome string
 Sequence int64
 ProofID string
 ResultHash [32]byte
}
// ApplyPlan is only a transaction plan. It is not evidence of a committed write.
type ApplyPlan struct {
 Outcome string
 Sequence int64
 Duplicate bool
}
func Fingerprint(Command) ([32]byte, error) { return [32]byte{}, errNotImplemented }
func PlanReceipt(Command, Receipt, int64, *Receipt) (ApplyPlan, error) {
 return ApplyPlan{}, errNotImplemented
}
