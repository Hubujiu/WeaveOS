package flowcommands

import "bytes"

func DecodeExecutionPayload(action string, body []byte) (ExecutionPayload, error) {
	switch action {
	case "start", "agree", "reject", "withdraw", "return":
	default:
		return ExecutionPayload{}, ErrInvalid
	}
	if len(body) < 45 || len(body) > 262144 || string(body[:8]) != "WVFPAY\x00\x01" {
		return ExecutionPayload{}, ErrInvalid
	}
	var payload ExecutionPayload
	copy(payload.EvidenceHash[:], body[8:40])
	if !isNonzeroHash(payload.EvidenceHash) {
		return ExecutionPayload{}, ErrInvalid
	}
	reader := executionResultReader{body: body, offset: 40}
	readBool := func() bool {
		if reader.invalid || len(reader.body)-reader.offset < 1 {
			reader.invalid = true
			return false
		}
		value := reader.body[reader.offset]
		reader.offset++
		if value > 1 {
			reader.invalid = true
			return false
		}
		return value == 1
	}
	hasStart := readBool()
	if reader.invalid || hasStart != (action == "start") {
		return ExecutionPayload{}, ErrInvalid
	}
	if hasStart {
		allowWithdraw := readBool()
		count := reader.uint32()
		// Each node needs a UUID, an actor count, and at least one actor UUID.
		// The route count must remain available after all nodes.
		if reader.invalid || count > 100 || uint64(count)*84+4 > uint64(len(body)-reader.offset) {
			return ExecutionPayload{}, ErrInvalid
		}
		payload.Start = &ExecutionStart{
			AllowWithdraw: allowWithdraw,
			Approvers:     make(map[string][]string, int(count)),
		}
		previousNode := ""
		for i := uint32(0); i < count; i++ {
			node := reader.text(36)
			if reader.invalid || !isCanonicalNonzeroUUID(node) || node <= previousNode {
				return ExecutionPayload{}, ErrInvalid
			}
			actorCount := reader.uint32()
			if reader.invalid || actorCount < 1 || actorCount > 50 || uint64(actorCount)*40 > uint64(len(body)-reader.offset) {
				return ExecutionPayload{}, ErrInvalid
			}
			actors := make([]string, int(actorCount))
			previousActor := ""
			for j := range actors {
				actor := reader.text(36)
				if reader.invalid || !isCanonicalNonzeroUUID(actor) || actor <= previousActor {
					return ExecutionPayload{}, ErrInvalid
				}
				actors[j] = actor
				previousActor = actor
			}
			payload.Start.Approvers[node] = actors
			previousNode = node
		}
	}
	count := reader.uint32()
	// Each route needs a UUID and one strict boolean byte.
	if reader.invalid || count > 100 || uint64(count)*41 > uint64(len(body)-reader.offset) {
		return ExecutionPayload{}, ErrInvalid
	}
	payload.Routes = make(map[string]bool, int(count))
	previousNode := ""
	for i := uint32(0); i < count; i++ {
		node := reader.text(36)
		if reader.invalid || !isCanonicalNonzeroUUID(node) || node <= previousNode {
			return ExecutionPayload{}, ErrInvalid
		}
		flag := readBool()
		if reader.invalid {
			return ExecutionPayload{}, ErrInvalid
		}
		payload.Routes[node] = flag
		previousNode = node
	}
	if reader.invalid || reader.offset != len(body) {
		return ExecutionPayload{}, ErrInvalid
	}
	encoded, err := EncodeExecutionPayload(action, payload)
	if err != nil || !bytes.Equal(encoded, body) {
		return ExecutionPayload{}, ErrInvalid
	}
	return payload, nil
}
