package workflowrpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"reflect"
	"testing"
	"time"
)

type rootExecutionConn struct {
	calls   int
	method  string
	request proto.Message
	handle  func(context.Context, string, proto.Message, proto.Message) error
}

func (c *rootExecutionConn) Invoke(ctx context.Context, method string, in, out any, opts ...grpc.CallOption) error {
	c.calls++
	c.method = method
	c.request = proto.Clone(in.(proto.Message))
	return c.handle(ctx, method, in.(proto.Message), out.(proto.Message))
}
func (c *rootExecutionConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("unary RPC only")
}
func rootExecutionID(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }
func rootExecutionInput() (fc.Command, fc.ExecutionPayload) {
	p := fc.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-evidence")), Start: &fc.ExecutionStart{AllowWithdraw: true, Approvers: map[string][]string{rootExecutionID(2): {rootExecutionID(8), rootExecutionID(9)}, rootExecutionID(3): {rootExecutionID(10), rootExecutionID(11), rootExecutionID(12)}}}, Routes: map[string]bool{}}
	b, e := fc.EncodeExecutionPayload("start", p)
	if e != nil {
		panic(e)
	}
	c := fc.Command{ProtocolVersion: 2, CommandID: rootExecutionID(201), AppID: rootExecutionID(101), TableID: rootExecutionID(103), ViewID: rootExecutionID(104), RecordID: rootExecutionID(105), FlowID: rootExecutionID(102), VersionID: rootExecutionID(100), InstanceID: rootExecutionID(202), ActorID: rootExecutionID(106), Action: "start", DefinitionVersion: 1, SchemaVersion: 1, RecordVersion: 1, FenceEpoch: 1, PayloadHash: sha256.Sum256(b)}
	return c, p
}

// Independent fixture encoder, never the implementation's result encoder.
func rootExecutionBody(c fc.Command, outcome string) []byte {
	b := []byte{'W', 'V', 'F', 'R', 'S', 'L', 0, 1}
	text := func(s string) { b = binary.BigEndian.AppendUint32(b, uint32(len(s))); b = append(b, s...) }
	text(c.InstanceID)
	if outcome == "success" {
		text("engine-process-一")
		text("active")
		text("")
	} else {
		text("")
		text("unchanged")
		text("cancelled")
	}
	b = binary.BigEndian.AppendUint64(b, uint64(c.SchemaVersion))
	b = binary.BigEndian.AppendUint64(b, uint64(c.RecordVersion))
	if outcome == "success" {
		b = binary.BigEndian.AppendUint32(b, 1)
		text(rootExecutionID(203))
		text(rootExecutionID(2))
		text(rootExecutionID(8))
		text("engine-task-一")
		b = binary.BigEndian.AppendUint64(b, 1)
	} else {
		b = binary.BigEndian.AppendUint32(b, 0)
	}
	return b
}
func rootExecutionReceipt(c fc.Command, outcome string) *pb.ExecutionReceipt {
	b := rootExecutionBody(c, outcome)
	rh := sha256.Sum256(b)
	ch, e := fc.Fingerprint(c)
	if e != nil {
		panic(e)
	}
	seq := c.ExpectedSequence
	if outcome == "success" {
		seq++
	}
	return &pb.ExecutionReceipt{CommandId: c.CommandID, CommandHash: ch[:], Outcome: outcome, Sequence: uint64(seq), ProofId: rootExecutionID(204), ResultHash: rh[:], ResultBytes: b}
}
func rootExecutionClient(t *testing.T, conn *rootExecutionConn) *ExecutionClient {
	t.Helper()
	c, e := NewExecutionClient(conn, time.Second)
	if e != nil || c == nil {
		t.Fatalf("constructor: %v", e)
	}
	return c
}
func rootExecutionSuccess(c fc.Command) *rootExecutionConn {
	return &rootExecutionConn{handle: func(ctx context.Context, m string, in, out proto.Message) error {
		proto.Merge(out, rootExecutionReceipt(c, "success"))
		return nil
	}}
}
func TestRootExecutionClientConstructorBounds(t *testing.T) {
	var typedNil *rootExecutionConn
	for _, x := range []struct {
		conn grpc.ClientConnInterface
		d    time.Duration
	}{{nil, time.Second}, {typedNil, time.Second}, {&rootExecutionConn{}, 0}, {&rootExecutionConn{}, -1}, {&rootExecutionConn{}, 31 * time.Second}} {
		if c, e := NewExecutionClient(x.conn, x.d); e == nil || c != nil {
			t.Fatal("invalid constructor accepted")
		}
	}
	if c, e := NewExecutionClient(&rootExecutionConn{}, 30*time.Second); e != nil || c == nil {
		t.Fatalf("upper limit rejected: %v", e)
	}
}
func TestRootExecutionClientSendsCanonicalBoundBytes(t *testing.T) {
	c, p := rootExecutionInput()
	conn := rootExecutionSuccess(c)
	client := rootExecutionClient(t, conn)
	r, e := client.Execute(context.Background(), c, p)
	if e != nil || r == nil {
		t.Fatalf("execute: %v", e)
	}
	if conn.calls != 1 || conn.method != "/weaveos.workflow.v1.ExecutionService/Execute" {
		t.Fatal("wrong method or call count")
	}
	q, ok := conn.request.(*pb.ExecutionRequest)
	if !ok {
		t.Fatal("wrong request type")
	}
	cb, _ := fc.CanonicalBytes(c)
	payload, _ := fc.EncodeExecutionPayload(c.Action, p)
	if !bytes.Equal(q.CommandEnvelope, cb) || !bytes.Equal(q.Payload, payload) {
		t.Fatal("noncanonical bytes")
	}
	ch, _ := fc.Fingerprint(c)
	if r.Receipt.CommandHash != ch || r.Receipt.Outcome != "success" || r.Result.InstanceID != c.InstanceID || r.Result.State != "active" || len(r.Result.Tasks) != 1 || r.Result.Tasks[0].EngineTaskID != "engine-task-一" {
		t.Fatal("incorrect result")
	}
}
func TestRootExecutionClientRejectsInputsBeforeRPC(t *testing.T) {
	c, p := rootExecutionInput()
	conn := rootExecutionSuccess(c)
	client := rootExecutionClient(t, conn)
	bad := c
	bad.ProtocolVersion = 1
	wrongHash := c
	wrongHash.PayloadHash = sha256.Sum256([]byte("wrong"))
	wrongID := c
	wrongID.CommandID = "BAD"
	for _, q := range []fc.Command{bad, wrongHash, wrongID} {
		if r, e := client.Execute(context.Background(), q, p); e == nil || r != nil {
			t.Fatal("invalid execute accepted")
		}
		if r, e := client.EstablishNoEffect(context.Background(), q, p); e == nil || r != nil {
			t.Fatal("invalid cancellation accepted")
		}
	}
	if r, e := client.Execute(nil, c, p); e == nil || r != nil {
		t.Fatal("nil context accepted")
	}
	if r, found, e := client.Lookup(nil, c); e == nil || found || r != nil {
		t.Fatal("nil lookup context accepted")
	}
	if r, e := client.Execute(context.Background(), c, fc.ExecutionPayload{}); e == nil || r != nil {
		t.Fatal("invalid payload accepted")
	}
	if conn.calls != 0 {
		t.Fatal("invalid input reached transport")
	}
}
func TestRootExecutionClientNilReceiverReturnsError(t *testing.T) {
	c, p := rootExecutionInput()
	var client *ExecutionClient
	if r, e := client.Execute(context.Background(), c, p); e == nil || r != nil {
		t.Fatal("nil execute")
	}
	if r, found, e := client.Lookup(context.Background(), c); e == nil || r != nil || found {
		t.Fatal("nil lookup")
	}
	if r, e := client.EstablishNoEffect(context.Background(), c, p); e == nil || r != nil {
		t.Fatal("nil cancel")
	}
}
func TestRootExecutionClientRejectsDamagedReceipts(t *testing.T) {
	c, p := rootExecutionInput()
	mutations := []func(*pb.ExecutionReceipt){
		func(r *pb.ExecutionReceipt) { r.CommandId = rootExecutionID(999) },
		func(r *pb.ExecutionReceipt) { r.CommandHash = []byte{1} },
		func(r *pb.ExecutionReceipt) { r.CommandHash = make([]byte, 32) },
		func(r *pb.ExecutionReceipt) { r.ResultHash = make([]byte, 31) },
		func(r *pb.ExecutionReceipt) { r.ResultHash = make([]byte, 32) },
		func(r *pb.ExecutionReceipt) { r.Sequence = 0 },
		func(r *pb.ExecutionReceipt) { r.Sequence = ^uint64(0) },
		func(r *pb.ExecutionReceipt) { r.ProofId = "bad" },
		func(r *pb.ExecutionReceipt) { r.Outcome = "pending" },
		func(r *pb.ExecutionReceipt) { r.Outcome = "no_effect" },
		func(r *pb.ExecutionReceipt) {
			other := c
			other.InstanceID = rootExecutionID(999)
			r.ResultBytes = rootExecutionBody(other, "success")
			h := sha256.Sum256(r.ResultBytes)
			r.ResultHash = h[:]
		},
		func(r *pb.ExecutionReceipt) {
			other := c
			other.RecordVersion++
			r.ResultBytes = rootExecutionBody(other, "success")
			h := sha256.Sum256(r.ResultBytes)
			r.ResultHash = h[:]
		},
		func(r *pb.ExecutionReceipt) {
			r.ResultBytes = make([]byte, 65537)
			h := sha256.Sum256(r.ResultBytes)
			r.ResultHash = h[:]
		},
		func(r *pb.ExecutionReceipt) {
			r.ProtoReflect().SetUnknown(append([]byte{0xa2, 0x06, 0x80, 0x80, 0x08}, make([]byte, 131072)...))
		},
	}
	for i, mutate := range mutations {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			conn := &rootExecutionConn{handle: func(ctx context.Context, m string, in, out proto.Message) error {
				r := rootExecutionReceipt(c, "success")
				mutate(r)
				proto.Merge(out, r)
				return nil
			}}
			client := rootExecutionClient(t, conn)
			if r, e := client.Execute(context.Background(), c, p); e == nil || r != nil {
				t.Fatal("damaged receipt accepted")
			}
		})
	}
}
func TestRootExecutionClientLookupStatesAndBinding(t *testing.T) {
	c, _ := rootExecutionInput()
	for _, kind := range []string{"absent", "confirmed", "empty", "wrong"} {
		t.Run(kind, func(t *testing.T) {
			conn := &rootExecutionConn{handle: func(ctx context.Context, m string, in, out proto.Message) error {
				q := in.(*pb.ExecutionLookupRequest)
				h, _ := fc.Fingerprint(c)
				if q.CommandId != c.CommandID || !bytes.Equal(q.CommandHash, h[:]) {
					t.Fatal("lookup identity mismatch")
				}
				r := out.(*pb.ExecutionLookupResponse)
				if kind == "absent" {
					r.Result = &pb.ExecutionLookupResponse_NotObserved{NotObserved: &pb.NotObserved{}}
				}
				if kind == "confirmed" || kind == "wrong" {
					v := rootExecutionReceipt(c, "success")
					if kind == "wrong" {
						v.CommandId = rootExecutionID(999)
					}
					r.Result = &pb.ExecutionLookupResponse_Confirmed{Confirmed: v}
				}
				return nil
			}}
			client := rootExecutionClient(t, conn)
			r, found, e := client.Lookup(context.Background(), c)
			switch kind {
			case "absent":
				if e != nil || r != nil || found {
					t.Fatal("not observed became confirmed")
				}
			case "confirmed":
				if e != nil || r == nil || !found {
					t.Fatal("confirmed lookup rejected")
				}
			default:
				if e == nil || r != nil || found {
					t.Fatal("malformed lookup accepted")
				}
			}
			if conn.calls != 1 || conn.method != "/weaveos.workflow.v1.ExecutionService/Lookup" {
				t.Fatal("lookup transport count")
			}
		})
	}
}
func TestRootExecutionClientCancellationMayRecoverSuccess(t *testing.T) {
	c, p := rootExecutionInput()
	for _, outcome := range []string{"success", "no_effect"} {
		t.Run(outcome, func(t *testing.T) {
			conn := &rootExecutionConn{handle: func(ctx context.Context, m string, in, out proto.Message) error {
				proto.Merge(out, rootExecutionReceipt(c, outcome))
				return nil
			}}
			client := rootExecutionClient(t, conn)
			r, e := client.EstablishNoEffect(context.Background(), c, p)
			if e != nil || r == nil || r.Receipt.Outcome != outcome {
				t.Fatalf("arbitration result lost: %v", e)
			}
			if conn.calls != 1 || conn.method != "/weaveos.workflow.v1.ExecutionService/EstablishNoEffect" {
				t.Fatal("wrong cancellation method")
			}
		})
	}
}
func TestRootExecutionClientTransportErrorsStayUnknown(t *testing.T) {
	c, p := rootExecutionInput()
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Canceled} {
		for _, method := range []string{"execute", "lookup", "cancel"} {
			t.Run(fmt.Sprint(code, method), func(t *testing.T) {
				conn := &rootExecutionConn{handle: func(context.Context, string, proto.Message, proto.Message) error {
					return status.Error(code, "synthetic")
				}}
				client := rootExecutionClient(t, conn)
				var r *ExecutionConfirmed
				var e error
				var found bool
				switch method {
				case "execute":
					r, e = client.Execute(context.Background(), c, p)
				case "lookup":
					r, found, e = client.Lookup(context.Background(), c)
				case "cancel":
					r, e = client.EstablishNoEffect(context.Background(), c, p)
				}
				if status.Code(e) != code || r != nil || found || conn.calls != 1 {
					t.Fatal("transport error was retried or reclassified")
				}
			})
		}
	}
}
func TestRootExecutionClientDeadlineAndParentCancellation(t *testing.T) {
	c, p := rootExecutionInput()
	parent, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	conn := &rootExecutionConn{handle: func(ctx context.Context, m string, in, out proto.Message) error {
		d, ok := ctx.Deadline()
		pd, _ := parent.Deadline()
		if !ok || d.After(pd) {
			t.Fatal("parent deadline lost")
		}
		<-ctx.Done()
		return status.FromContextError(ctx.Err()).Err()
	}}
	client := rootExecutionClient(t, conn)
	cancel()
	r, e := client.Execute(parent, c, p)
	if e == nil || r != nil {
		t.Fatal("cancelled caller succeeded")
	}
	if conn.calls > 1 {
		t.Fatal("unexpected retry")
	}
	conn2 := &rootExecutionConn{handle: func(ctx context.Context, m string, in, out proto.Message) error {
		d, ok := ctx.Deadline()
		if !ok || time.Until(d) > 120*time.Millisecond {
			t.Fatal("client deadline missing")
		}
		<-ctx.Done()
		return status.FromContextError(ctx.Err()).Err()
	}}
	short, e := NewExecutionClient(conn2, 50*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := short.Execute(context.Background(), c, p); e == nil || r != nil {
		t.Fatal("timeout became success")
	}
}
func TestRootExecutionClientDoesNotMutateCallerInput(t *testing.T) {
	c, p := rootExecutionInput()
	p.Start.Approvers[rootExecutionID(2)] = []string{rootExecutionID(9), rootExecutionID(8)}
	before := append([]string(nil), p.Start.Approvers[rootExecutionID(2)]...)
	conn := rootExecutionSuccess(c)
	client := rootExecutionClient(t, conn)
	if _, e := client.Execute(context.Background(), c, p); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, p.Start.Approvers[rootExecutionID(2)]) {
		t.Fatal("caller roster mutated")
	}
}
