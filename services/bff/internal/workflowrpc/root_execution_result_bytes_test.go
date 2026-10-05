package workflowrpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/protobuf/proto"
	"testing"
)

// Root-authored acceptance: preserve the exact engine evidence across all ports.
func TestRootExecutionResultBytesOwnership(t *testing.T) {
	for _, method := range []string{"execute", "lookup", "no_effect"} {
		for _, outcome := range []string{"success", "no_effect"} {
			t.Run(method+"/"+outcome, func(t *testing.T) {
				command, payload := rootExecutionInput()
				expected := rootExecutionBody(command, outcome)
				var wire *pb.ExecutionReceipt
				conn := &rootExecutionConn{handle: func(_ context.Context, _ string, _, out proto.Message) error {
					wire = rootExecutionReceipt(command, outcome)
					if method == "lookup" {
						out.(*pb.ExecutionLookupResponse).Result = &pb.ExecutionLookupResponse_Confirmed{Confirmed: wire}
					} else {
						proto.Merge(out, wire)
						wire = out.(*pb.ExecutionReceipt)
					}
					return nil
				}}
				client := rootExecutionClient(t, conn)
				call := func() *ExecutionConfirmed {
					t.Helper()
					var got *ExecutionConfirmed
					var err error
					switch method {
					case "execute":
						got, err = client.Execute(context.Background(), command, payload)
					case "no_effect":
						got, err = client.EstablishNoEffect(context.Background(), command, payload)
					case "lookup":
						var found bool
						got, found, err = client.Lookup(context.Background(), command)
						if !found && err == nil {
							t.Fatal("confirmed receipt was lost")
						}
					}
					if err != nil || got == nil {
						t.Fatalf("confirmed call: %v", err)
					}
					return got
				}
				got := call()
				if !bytes.Equal(got.ResultBytes, expected) {
					t.Fatal("exact original result bytes must reach the projection")
				}
				if sha256.Sum256(got.ResultBytes) != got.Receipt.ResultHash {
					t.Fatal("original result hash changed")
				}
				decoded, err := fc.DecodeExecutionResult(command, got.Receipt, got.ResultBytes)
				if err != nil || decoded.InstanceID != command.InstanceID || decoded.State != got.Result.State {
					t.Fatalf("projection cannot independently validate original evidence: %v", err)
				}
				wire.ResultBytes[0] ^= 0xff
				if !bytes.Equal(got.ResultBytes, expected) {
					t.Fatal("transport storage aliases returned evidence")
				}
				wire.ResultBytes[0] ^= 0xff
				got.ResultBytes[1] ^= 0xff
				if !bytes.Equal(wire.ResultBytes, expected) {
					t.Fatal("caller mutation changed transport evidence")
				}
				next := call()
				if !bytes.Equal(next.ResultBytes, expected) {
					t.Fatal("later call reused caller-mutated bytes")
				}
				next.ResultBytes[2] ^= 0xff
				if got.ResultBytes[2] != expected[2] {
					t.Fatal("independent calls share mutable evidence")
				}
			})
		}
	}
}
