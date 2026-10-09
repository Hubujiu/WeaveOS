package workflowrpc

import (
	"context"
	"errors"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

const deletionOperation = "70000000-0000-4000-8000-000000000001"

func deletionRequest() *pb.FlowDeletionRequest {
	return &pb.FlowDeletionRequest{AppId: appID, FlowId: flowID, OperationId: deletionOperation}
}
func deletionReceipt() *pb.FlowDeletionReceipt {
	return &pb.FlowDeletionReceipt{AppId: appID, FlowId: flowID, OperationId: deletionOperation, DeletedVersions: 2, DeletedAtSeconds: 1791561600, DeletedAtNanos: 123456000}
}
func TestRootDeletionClientBoundResultCopiesRequestAndPreservesEarlierDeadline(t *testing.T) {
	q := deletionRequest()
	before := proto.Clone(q)
	deadline := time.Now().Add(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	conn := &connection{invoke: func(c context.Context, m string, in, out any) error {
		if m != "/weaveos.workflow.v1.DeploymentService/DeleteFlow" {
			t.Fatal(m)
		}
		d, ok := c.Deadline()
		if !ok || !d.Equal(deadline) {
			t.Fatal("caller deadline changed")
		}
		if in == q || !proto.Equal(in.(proto.Message), q) {
			t.Fatal("input not independently copied")
		}
		in.(*pb.FlowDeletionRequest).AppId = flowID
		proto.Merge(out.(proto.Message), deletionReceipt())
		return nil
	}}
	r, e := client(t, conn).DeleteFlow(ctx, q)
	if e != nil || !proto.Equal(r, deletionReceipt()) || conn.calls != 1 || !proto.Equal(q, before) {
		t.Fatalf("result %v err %v calls %d", r, e, conn.calls)
	}
}
func TestRootDeletionClientInvalidInputsNeverCallTransport(t *testing.T) {
	qs := []*pb.FlowDeletionRequest{nil, {}, deletionRequest(), deletionRequest(), deletionRequest(), deletionRequest()}
	qs[2].AppId = "bad"
	qs[3].FlowId = "00000000-0000-0000-0000-000000000000"
	qs[4].OperationId = "bad"
	qs[5].ProtoReflect().SetUnknown(append([]byte{0xa2, 6, 0x80, 8}, make([]byte, 1024)...))
	conn := &connection{invoke: func(context.Context, string, any, any) error { t.Fatal("invalid input reached network"); return nil }}
	c := client(t, conn)
	for _, q := range qs {
		if r, e := c.DeleteFlow(context.Background(), q); e == nil || r != nil {
			t.Fatal("bad delete input accepted")
		}
		if r, e := c.LookupFlowDeletion(context.Background(), q); e == nil || r != nil {
			t.Fatal("bad lookup input accepted")
		}
	}
	if _, e := c.DeleteFlow(nil, deletionRequest()); e == nil {
		t.Fatal("nil context accepted")
	}
	var absent *Client
	if _, e := absent.LookupFlowDeletion(context.Background(), deletionRequest()); e == nil {
		t.Fatal("nil client accepted")
	}
	if conn.calls != 0 {
		t.Fatal(conn.calls)
	}
}
func TestRootDeletionClientRejectsEveryUnboundOrInvalidReceipt(t *testing.T) {
	cases := map[string]func(*pb.FlowDeletionReceipt){"app": func(r *pb.FlowDeletionReceipt) { r.AppId = flowID }, "flow": func(r *pb.FlowDeletionReceipt) { r.FlowId = appID }, "operation": func(r *pb.FlowDeletionReceipt) { r.OperationId = appID }, "count": func(r *pb.FlowDeletionReceipt) { r.DeletedVersions = 9007199254740992 }, "time_zero": func(r *pb.FlowDeletionReceipt) { r.DeletedAtSeconds = 0 }, "time_high": func(r *pb.FlowDeletionReceipt) { r.DeletedAtSeconds = 253402300800 }, "nano_high": func(r *pb.FlowDeletionReceipt) { r.DeletedAtNanos = 1000000000 }, "nano_negative": func(r *pb.FlowDeletionReceipt) { r.DeletedAtNanos = -1 }, "submicro": func(r *pb.FlowDeletionReceipt) { r.DeletedAtNanos = 1 }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			bad := deletionReceipt()
			mutate(bad)
			conn := &connection{invoke: func(_ context.Context, _ string, _, out any) error {
				switch v := out.(type) {
				case *pb.FlowDeletionReceipt:
					proto.Merge(v, bad)
				case *pb.FlowDeletionLookupResponse:
					v.Result = &pb.FlowDeletionLookupResponse_Confirmed{Confirmed: bad}
				}
				return nil
			}}
			c := client(t, conn)
			if r, e := c.DeleteFlow(context.Background(), deletionRequest()); e == nil || r != nil {
				t.Fatal("bad receipt accepted")
			}
			if r, e := c.LookupFlowDeletion(context.Background(), deletionRequest()); e == nil || r != nil {
				t.Fatal("bad lookup receipt accepted")
			}
		})
	}
}
func TestRootDeletionClientLookupBranchesAndTransportFailure(t *testing.T) {
	variants := []*pb.FlowDeletionLookupResponse{{Result: &pb.FlowDeletionLookupResponse_NotObserved{NotObserved: &pb.NotObserved{}}}, {Result: &pb.FlowDeletionLookupResponse_Confirmed{Confirmed: deletionReceipt()}}, {}, {Result: &pb.FlowDeletionLookupResponse_Confirmed{}}, {Result: &pb.FlowDeletionLookupResponse_NotObserved{}}}
	for i, v := range variants {
		conn := &connection{invoke: func(_ context.Context, m string, _, out any) error {
			if m != "/weaveos.workflow.v1.DeploymentService/LookupFlowDeletion" {
				t.Fatal(m)
			}
			// Preserve deliberately malformed nil oneof payloads; proto.Merge normalizes them.
			out.(*pb.FlowDeletionLookupResponse).Result = v.Result
			return nil
		}}
		r, e := client(t, conn).LookupFlowDeletion(context.Background(), deletionRequest())
		if i < 2 {
			if e != nil || !proto.Equal(r, v) {
				t.Fatal("valid branch rejected")
			}
		} else if e == nil || r != nil {
			t.Fatal("missing branch accepted")
		}
		if conn.calls != 1 {
			t.Fatal("not one call")
		}
	}
	failure := errors.New("uncertain transport")
	conn := &connection{invoke: func(context.Context, string, any, any) error { return failure }}
	c := client(t, conn)
	if r, e := c.DeleteFlow(context.Background(), deletionRequest()); r != nil || !errors.Is(e, failure) {
		t.Fatal("transport failure misrepresented")
	}
	if r, e := c.LookupFlowDeletion(context.Background(), deletionRequest()); r != nil || !errors.Is(e, failure) {
		t.Fatal("transport failure became notObserved")
	}
	if conn.calls != 2 {
		t.Fatal("unexpected retries")
	}
}
