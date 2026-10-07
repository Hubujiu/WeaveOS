package flowcommands

import (
	"encoding/binary"
	"encoding/json"
)

// CanonicalBytes validates the envelope and returns independently owned identity bytes.
// Version 1 preserves historical JSON; version 2 uses the fixed cross-language framing.
func CanonicalBytes(command Command) ([]byte, error) {
	if !validCommand(command) {
		return nil, ErrInvalid
	}
	if command.ProtocolVersion == 1 {
		encoded, err := json.Marshal(command)
		if err != nil {
			return nil, ErrInvalid
		}
		return encoded, nil
	}

	strings := [...]string{
		command.CommandID, command.AppID, command.TableID, command.ViewID,
		command.RecordID, command.FlowID, command.VersionID, command.InstanceID,
		command.TaskID, command.ActorID, command.Action, command.TargetNodeID,
	}
	integers := [...]int64{
		command.DefinitionVersion, command.SchemaVersion, command.RecordVersion,
		command.FenceEpoch, command.TaskEpoch, command.ExpectedSequence,
	}
	// Validation bounds every string to a UUID or a fixed action, so all lengths
	// fit uint32 and the complete encoding has a constant size bound.
	size := 8 + 4*len(strings) + 8*len(integers) + len(command.PayloadHash)
	for _, value := range strings {
		size += len(value)
	}
	encoded := make([]byte, 0, size)
	encoded = append(encoded, 0x57, 0x56, 0x46, 0x43, 0x4d, 0x44, 0x00, 0x02)
	for _, value := range strings {
		encoded = binary.BigEndian.AppendUint32(encoded, uint32(len(value)))
		encoded = append(encoded, value...)
	}
	for _, value := range integers {
		encoded = binary.BigEndian.AppendUint64(encoded, uint64(value))
	}
	return append(encoded, command.PayloadHash[:]...), nil
}
