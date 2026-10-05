package flowcommands

import (
	"crypto/sha256"
	"errors"
)

var (
	ErrInvalid    = errors.New("invalid workflow command")
	ErrConflict   = errors.New("workflow receipt conflict")
	ErrOutOfOrder = errors.New("workflow sequence conflict")
)

const maxSafeInteger int64 = 9007199254740991

// Command is a trusted internal envelope, never an authorization result.
type Command struct {
	ProtocolVersion                                                          int
	CommandID, AppID, TableID, RecordID, InstanceID, TaskID, ActorID, Action string
	RecordVersion, FenceEpoch, TaskEpoch, ExpectedSequence                   int64
	PayloadHash                                                              [32]byte
	ViewID, FlowID, VersionID, TargetNodeID                                  string `json:",omitempty"`
	DefinitionVersion, SchemaVersion                                         int64  `json:",omitempty"`
}

type Receipt struct {
	CommandID   string
	CommandHash [32]byte
	Outcome     string
	Sequence    int64
	ProofID     string
	ResultHash  [32]byte
}

// ApplyPlan is only a transaction plan. It is not evidence of a committed write.
type ApplyPlan struct {
	Outcome   string
	Sequence  int64
	Duplicate bool
}

func Fingerprint(command Command) ([32]byte, error) {
	encoded, err := CanonicalBytes(command)
	if err != nil {
		return [32]byte{}, ErrInvalid
	}
	return sha256.Sum256(encoded), nil
}

func PlanReceipt(command Command, receipt Receipt, currentSequence int64, previous *Receipt) (ApplyPlan, error) {
	commandHash, err := Fingerprint(command)
	if err != nil {
		return ApplyPlan{}, err
	}
	if !isSafeNonNegative(currentSequence) || !validReceiptShape(receipt) {
		return ApplyPlan{}, ErrInvalid
	}
	if receipt.CommandID != command.CommandID || receipt.CommandHash != commandHash {
		return ApplyPlan{}, ErrConflict
	}

	if previous != nil {
		if receipt != *previous {
			return ApplyPlan{}, ErrConflict
		}
		if !receiptSequenceMatches(command.ExpectedSequence, receipt) {
			return ApplyPlan{}, ErrOutOfOrder
		}
		return ApplyPlan{Outcome: receipt.Outcome, Sequence: receipt.Sequence, Duplicate: true}, nil
	}

	if currentSequence != command.ExpectedSequence {
		return ApplyPlan{}, ErrOutOfOrder
	}
	if !receiptSequenceMatches(currentSequence, receipt) {
		return ApplyPlan{}, ErrOutOfOrder
	}

	return ApplyPlan{Outcome: receipt.Outcome, Sequence: receipt.Sequence}, nil
}

func validCommand(command Command) bool {
	if (command.ProtocolVersion != 1 && command.ProtocolVersion != 2) ||
		!isCanonicalNonzeroUUID(command.CommandID) ||
		!isCanonicalNonzeroUUID(command.AppID) ||
		!isCanonicalNonzeroUUID(command.TableID) ||
		!isCanonicalNonzeroUUID(command.RecordID) ||
		!isCanonicalNonzeroUUID(command.InstanceID) ||
		!isCanonicalNonzeroUUID(command.ActorID) ||
		command.RecordVersion < 1 || command.RecordVersion > maxSafeInteger ||
		command.FenceEpoch < 1 || command.FenceEpoch > maxSafeInteger ||
		!isSafeNonNegative(command.ExpectedSequence) ||
		!isNonzeroHash(command.PayloadHash) {
		return false
	}

	if command.ProtocolVersion == 2 {
		return validV2Command(command)
	}
	if command.ViewID != "" || command.FlowID != "" || command.VersionID != "" ||
		command.TargetNodeID != "" || command.DefinitionVersion != 0 || command.SchemaVersion != 0 {
		return false
	}

	switch command.Action {
	case "start":
		return command.TaskID == "" && command.TaskEpoch == 0
	case "agree", "reject", "withdraw":
		return isCanonicalNonzeroUUID(command.TaskID) &&
			command.TaskEpoch >= 1 && command.TaskEpoch <= maxSafeInteger
	default:
		return false
	}
}

func validV2Command(command Command) bool {
	if !isCanonicalNonzeroUUID(command.ViewID) ||
		!isCanonicalNonzeroUUID(command.FlowID) ||
		!isCanonicalNonzeroUUID(command.VersionID) ||
		command.DefinitionVersion < 1 || command.DefinitionVersion > maxSafeInteger ||
		command.SchemaVersion < 1 || command.SchemaVersion > maxSafeInteger {
		return false
	}

	switch command.Action {
	case "start":
		return command.TaskID == "" && command.TaskEpoch == 0 &&
			command.TargetNodeID == "" && command.ExpectedSequence == 0
	case "withdraw":
		return command.TaskID == "" && command.TaskEpoch == 0 && command.TargetNodeID == ""
	case "agree", "reject":
		return isCanonicalNonzeroUUID(command.TaskID) &&
			command.TaskEpoch >= 1 && command.TaskEpoch <= maxSafeInteger && command.TargetNodeID == ""
	case "return":
		return isCanonicalNonzeroUUID(command.TaskID) &&
			command.TaskEpoch >= 1 && command.TaskEpoch <= maxSafeInteger &&
			isCanonicalNonzeroUUID(command.TargetNodeID)
	default:
		return false
	}
}

func validReceiptShape(receipt Receipt) bool {
	if receipt.Outcome != "success" && receipt.Outcome != "no_effect" {
		return false
	}
	return isSafeNonNegative(receipt.Sequence) &&
		isCanonicalNonzeroUUID(receipt.ProofID) &&
		isNonzeroHash(receipt.ResultHash)
}

func receiptSequenceMatches(expectedSequence int64, receipt Receipt) bool {
	switch receipt.Outcome {
	case "no_effect":
		return receipt.Sequence == expectedSequence
	case "success":
		if expectedSequence >= maxSafeInteger {
			return false
		}
		return receipt.Sequence == expectedSequence+1
	default:
		return false
	}
}

func isSafeNonNegative(value int64) bool {
	return value >= 0 && value <= maxSafeInteger
}

func isNonzeroHash(value [32]byte) bool {
	return value != [32]byte{}
}

func isCanonicalNonzeroUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}

	nonzero := false
	for index := 0; index < len(value); index++ {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		character := value[index]
		if character >= '1' && character <= '9' || character >= 'a' && character <= 'f' {
			nonzero = true
			continue
		}
		if character != '0' {
			return false
		}
	}
	return nonzero
}
