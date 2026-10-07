package flowcommands

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

type rootDecodeNode struct {
	id     string
	actors []string
}
type rootDecodeRoute struct {
	id   string
	flag byte
}

// Independent malformed-wire builder; it deliberately does not validate inputs.
func rootDecodeWire(start, allow byte, nodes []rootDecodeNode, routes []rootDecodeRoute, evidence [32]byte) []byte {
	b := append([]byte("WVFPAY\x00\x01"), evidence[:]...)
	b = append(b, start)
	text := func(s string) { b = binary.BigEndian.AppendUint32(b, uint32(len(s))); b = append(b, s...) }
	if start != 0 {
		b = append(b, allow)
		b = binary.BigEndian.AppendUint32(b, uint32(len(nodes)))
		for _, n := range nodes {
			text(n.id)
			b = binary.BigEndian.AppendUint32(b, uint32(len(n.actors)))
			for _, actor := range n.actors {
				text(actor)
			}
		}
	}
	b = binary.BigEndian.AppendUint32(b, uint32(len(routes)))
	for _, r := range routes {
		text(r.id)
		b = append(b, r.flag)
	}
	return b
}

func TestRootDecodeExecutionPayloadGolden(t *testing.T) {
	vectors := executionVectors(t)
	for _, c := range []struct{ name, action string }{
		{"payload_start", "start"}, {"payload_agree", "agree"}, {"payload_withdraw", "withdraw"},
		{"payload_agree", "reject"}, {"payload_agree", "return"},
	} {
		t.Run(c.action, func(t *testing.T) {
			body := vectorBytes(t, vectors[c.name])
			got, err := DecodeExecutionPayload(c.action, body)
			if err != nil {
				t.Fatalf("valid golden rejected: %v", err)
			}
			roundtrip, err := EncodeExecutionPayload(c.action, got)
			if err != nil || !bytes.Equal(roundtrip, body) {
				t.Fatal("golden original bytes changed")
			}
			if got.EvidenceHash != sha256.Sum256([]byte("root-audit-evidence")) {
				t.Fatal("evidence binding lost")
			}
			if c.action == "start" {
				if got.Start == nil || !got.Start.AllowWithdraw || len(got.Start.Approvers) != 2 ||
					!reflect.DeepEqual(got.Start.Approvers[executionID(2)], []string{executionID(8), executionID(9)}) {
					t.Fatal("start roster changed")
				}
			} else if got.Start != nil {
				t.Fatal("non-start gained a start payload")
			}
		})
	}
}

func rootDecodeReject(t *testing.T, action string, body []byte) {
	t.Helper()
	got, err := DecodeExecutionPayload(action, body)
	if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(got, ExecutionPayload{}) {
		t.Fatalf("malformed wire must return zero payload and ErrInvalid; got error=%v", err)
	}
}
func TestRootDecodeExecutionPayloadRejectsAllTruncations(t *testing.T) {
	vectors := executionVectors(t)
	for _, c := range []struct{ name, action string }{{"payload_start", "start"}, {"payload_agree", "agree"}, {"payload_withdraw", "withdraw"}} {
		body := vectorBytes(t, vectors[c.name])
		for n := 0; n < len(body); n++ {
			rootDecodeReject(t, c.action, body[:n])
		}
		rootDecodeReject(t, c.action, append(append([]byte(nil), body...), 0))
	}
	rootDecodeReject(t, "start", make([]byte, 262145))
	rootDecodeReject(t, "unknown", vectorBytes(t, vectors["payload_start"]))
}

func TestRootDecodeExecutionPayloadRejectsNoncanonicalStructures(t *testing.T) {
	h := sha256.Sum256([]byte("root-decode"))
	n1, n2, a1, a2 := executionID(1), executionID(2), executionID(8), executionID(9)
	goodNodes := []rootDecodeNode{{n1, []string{a1, a2}}}
	cases := []struct {
		name, action string
		body         []byte
	}{
		{"start-flag", "start", rootDecodeWire(2, 1, goodNodes, nil, h)},
		{"allow-flag", "start", rootDecodeWire(1, 2, goodNodes, nil, h)},
		{"route-flag", "agree", rootDecodeWire(0, 0, nil, []rootDecodeRoute{{n1, 2}}, h)},
		{"missing-start", "start", rootDecodeWire(0, 0, nil, nil, h)},
		{"unexpected-start", "agree", rootDecodeWire(1, 1, goodNodes, nil, h)},
		{"zero-evidence", "agree", rootDecodeWire(0, 0, nil, nil, [32]byte{})},
		{"duplicate-node", "start", rootDecodeWire(1, 1, []rootDecodeNode{{n1, []string{a1}}, {n1, []string{a2}}}, nil, h)},
		{"node-order", "start", rootDecodeWire(1, 1, []rootDecodeNode{{n2, []string{a1}}, {n1, []string{a2}}}, nil, h)},
		{"duplicate-actor", "start", rootDecodeWire(1, 1, []rootDecodeNode{{n1, []string{a1, a1}}}, nil, h)},
		{"actor-order", "start", rootDecodeWire(1, 1, []rootDecodeNode{{n1, []string{a2, a1}}}, nil, h)},
		{"empty-actors", "start", rootDecodeWire(1, 1, []rootDecodeNode{{n1, nil}}, nil, h)},
		{"bad-node", "start", rootDecodeWire(1, 1, []rootDecodeNode{{"bad", []string{a1}}}, nil, h)},
		{"zero-node", "start", rootDecodeWire(1, 1, []rootDecodeNode{{"00000000-0000-0000-0000-000000000000", []string{a1}}}, nil, h)},
		{"bad-actor", "start", rootDecodeWire(1, 1, []rootDecodeNode{{n1, []string{"BAD"}}}, nil, h)},
		{"invalid-utf8", "start", rootDecodeWire(1, 1, []rootDecodeNode{{string([]byte{255}), []string{a1}}}, nil, h)},
		{"duplicate-route", "agree", rootDecodeWire(0, 0, nil, []rootDecodeRoute{{n1, 0}, {n1, 1}}, h)},
		{"route-order", "agree", rootDecodeWire(0, 0, nil, []rootDecodeRoute{{n2, 0}, {n1, 1}}, h)},
		{"bad-route", "agree", rootDecodeWire(0, 0, nil, []rootDecodeRoute{{"bad", 0}}, h)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { rootDecodeReject(t, c.action, c.body) })
	}
	// Declared lengths/counts must be checked before allocation or slicing.
	base := rootDecodeWire(1, 1, goodNodes, nil, h)
	for _, offset := range []int{42, 46, 86, 90} {
		body := append([]byte(nil), base...)
		binary.BigEndian.PutUint32(body[offset:offset+4], ^uint32(0))
		rootDecodeReject(t, "start", body)
	}
	body := rootDecodeWire(0, 0, nil, nil, h)
	binary.BigEndian.PutUint32(body[41:45], ^uint32(0))
	rootDecodeReject(t, "agree", body)
	body = append([]byte(nil), base...)
	body[0] = 'X'
	rootDecodeReject(t, "start", body)
}

func TestRootDecodeExecutionPayloadBoundsAndOwnership(t *testing.T) {
	h := sha256.Sum256([]byte("root-max"))
	nodes := make([]rootDecodeNode, 100)
	routes := make([]rootDecodeRoute, 100)
	for i := range nodes {
		actors := make([]string, 50)
		for j := range actors {
			actors[j] = executionID(1000 + j)
		}
		nodes[i] = rootDecodeNode{executionID(i + 1), actors}
		routes[i] = rootDecodeRoute{executionID(i + 1), byte(i % 2)}
	}
	body := rootDecodeWire(1, 1, nodes, routes, h)
	got, err := DecodeExecutionPayload("start", body)
	if err != nil || got.Start == nil || len(got.Start.Approvers) != 100 || len(got.Routes) != 100 {
		t.Fatalf("maximum valid payload: %v", err)
	}
	raw, err := EncodeExecutionPayload("start", got)
	if err != nil || !bytes.Equal(raw, body) {
		t.Fatal("maximum payload changed")
	}
	original := append([]byte(nil), body...)
	for i := range body {
		body[i] = 0
	}
	again, err := EncodeExecutionPayload("start", got)
	if err != nil || !bytes.Equal(again, original) {
		t.Fatal("input mutation changed decoded payload")
	}
	second, err := DecodeExecutionPayload("start", original)
	if err != nil {
		t.Fatal(err)
	}
	got.Start.Approvers[executionID(1)][0] = executionID(9999)
	got.Routes[executionID(1)] = true
	secondBytes, err := EncodeExecutionPayload("start", second)
	if err != nil || !bytes.Equal(secondBytes, original) {
		t.Fatal("independent decoded results alias")
	}
	rootDecodeReject(t, "start", rootDecodeWire(1, 1, append(nodes, rootDecodeNode{executionID(101), []string{executionID(1000)}}), routes, h))
	badActors := append([]string(nil), nodes[0].actors...)
	badActors = append(badActors, executionID(1050))
	rootDecodeReject(t, "start", rootDecodeWire(1, 1, []rootDecodeNode{{executionID(1), badActors}}, nil, h))
	rootDecodeReject(t, "agree", rootDecodeWire(0, 0, nil, append(routes, rootDecodeRoute{executionID(101), 0}), h))
}

func FuzzRootDecodeExecutionPayload(f *testing.F) {
	vectors := executionVectors(f)
	for _, name := range []string{"payload_start", "payload_agree", "payload_withdraw"} {
		f.Add(name == "payload_start", vectorBytes(f, vectors[name]))
	}
	f.Add(false, []byte{})
	f.Fuzz(func(t *testing.T, start bool, body []byte) {
		action := "agree"
		if start {
			action = "start"
		}
		got, err := DecodeExecutionPayload(action, body)
		if err != nil {
			if !reflect.DeepEqual(got, ExecutionPayload{}) {
				t.Fatal("partial payload on failure")
			}
			return
		}
		roundtrip, e := EncodeExecutionPayload(action, got)
		if e != nil || !bytes.Equal(roundtrip, body) {
			t.Fatal("accepted noncanonical input")
		}
	})
}

func TestRootDecodeExecutionPayloadEmptyGraphs(t *testing.T) {
	h := sha256.Sum256([]byte("root-empty"))
	for _, action := range []string{"start", "agree", "reject", "withdraw", "return"} {
		start := byte(0)
		if action == "start" {
			start = 1
		}
		body := rootDecodeWire(start, 0, nil, nil, h)
		got, err := DecodeExecutionPayload(action, body)
		if err != nil {
			t.Fatalf("empty graph for %s: %v", action, err)
		}
		encoded, err := EncodeExecutionPayload(action, got)
		if err != nil || !bytes.Equal(encoded, body) {
			t.Fatal("empty graph encoding changed")
		}
	}
}
