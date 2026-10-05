package workflowrpc

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const executionRequestLimit = 270336
const executionLookupLimit = 1024
const executionReceiptLimit = 66560

var errExecutionReceipt = errors.New("unbound execution RPC result")

type ExecutionConfirmed struct {
	Receipt flowcommands.Receipt
	Result  flowcommands.ExecutionResult
}

// The caller owns the connection, credentials and disabling transport retries.
// Each operation issues one RPC and preserves uncertain transport failures.
type ExecutionClient struct {
	service pb.ExecutionServiceClient
	timeout time.Duration
}

func executionNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
func NewExecutionClient(conn grpc.ClientConnInterface, timeout time.Duration) (*ExecutionClient, error) {
	if executionNil(conn) || timeout <= 0 || timeout > 30*time.Second {
		return nil, flowcommands.ErrInvalid
	}
	return &ExecutionClient{service: pb.NewExecutionServiceClient(conn), timeout: timeout}, nil
}
func (c *ExecutionClient) valid(ctx context.Context) bool {
	return c != nil && !executionNil(c.service) && !executionNil(ctx) && c.timeout > 0 && c.timeout <= 30*time.Second
}
func executionCommand(command flowcommands.Command) ([]byte, error) {
	if command.ProtocolVersion != 2 {
		return nil, flowcommands.ErrInvalid
	}
	body, err := flowcommands.CanonicalBytes(command)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || len(body) > 538 {
		return nil, flowcommands.ErrInvalid
	}
	return body, nil
}
func executionRequest(command flowcommands.Command, payload flowcommands.ExecutionPayload) (*pb.ExecutionRequest, error) {
	envelope, err := executionCommand(command)
	if err != nil {
		return nil, err
	}
	body, err := flowcommands.EncodeExecutionPayload(command.Action, payload)
	if err != nil {
		return nil, err
	}
	if len(body) > 262144 || sha256.Sum256(body) != command.PayloadHash {
		return nil, flowcommands.ErrInvalid
	}
	request := &pb.ExecutionRequest{CommandEnvelope: envelope, Payload: body}
	if proto.Size(request) > executionRequestLimit {
		return nil, flowcommands.ErrInvalid
	}
	return request, nil
}
func executionConfirmed(command flowcommands.Command, wire *pb.ExecutionReceipt) (*ExecutionConfirmed, error) {
	if wire == nil || proto.Size(wire) > executionReceiptLimit || len(wire.CommandHash) != 32 || len(wire.ResultHash) != 32 || wire.Sequence > uint64(maxVersion) || len(wire.ResultBytes) > 65536 {
		return nil, errExecutionReceipt
	}
	receipt := flowcommands.Receipt{CommandID: wire.CommandId, Outcome: wire.Outcome, Sequence: int64(wire.Sequence), ProofID: wire.ProofId}
	copy(receipt.CommandHash[:], wire.CommandHash)
	copy(receipt.ResultHash[:], wire.ResultHash)
	result, err := flowcommands.DecodeExecutionResult(command, receipt, wire.ResultBytes)
	if err != nil {
		return nil, err
	}
	return &ExecutionConfirmed{Receipt: receipt, Result: result}, nil
}
func (c *ExecutionClient) Execute(ctx context.Context, command flowcommands.Command, payload flowcommands.ExecutionPayload) (*ExecutionConfirmed, error) {
	return c.execute(ctx, command, payload, false)
}
func (c *ExecutionClient) EstablishNoEffect(ctx context.Context, command flowcommands.Command, payload flowcommands.ExecutionPayload) (*ExecutionConfirmed, error) {
	return c.execute(ctx, command, payload, true)
}
func (c *ExecutionClient) execute(ctx context.Context, command flowcommands.Command, payload flowcommands.ExecutionPayload, cancelExecution bool) (*ExecutionConfirmed, error) {
	if !c.valid(ctx) {
		return nil, flowcommands.ErrInvalid
	}
	request, err := executionRequest(command, payload)
	if err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var response *pb.ExecutionReceipt
	options := []grpc.CallOption{grpc.MaxCallSendMsgSize(executionRequestLimit), grpc.MaxCallRecvMsgSize(executionReceiptLimit)}
	if cancelExecution {
		response, err = c.service.EstablishNoEffect(call, request, options...)
	} else {
		response, err = c.service.Execute(call, request, options...)
	}
	if err != nil {
		return nil, err
	}
	return executionConfirmed(command, response)
}
func (c *ExecutionClient) Lookup(ctx context.Context, command flowcommands.Command) (*ExecutionConfirmed, bool, error) {
	if !c.valid(ctx) {
		return nil, false, flowcommands.ErrInvalid
	}
	envelope, err := executionCommand(command)
	if err != nil {
		return nil, false, err
	}
	hash := sha256.Sum256(envelope)
	request := &pb.ExecutionLookupRequest{CommandId: command.CommandID, CommandHash: hash[:]}
	if proto.Size(request) > executionLookupLimit {
		return nil, false, flowcommands.ErrInvalid
	}
	call, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.service.Lookup(call, request, grpc.MaxCallSendMsgSize(executionLookupLimit), grpc.MaxCallRecvMsgSize(executionReceiptLimit))
	if err != nil {
		return nil, false, err
	}
	if response == nil || proto.Size(response) > executionReceiptLimit {
		return nil, false, errExecutionReceipt
	}
	switch result := response.Result.(type) {
	case *pb.ExecutionLookupResponse_Confirmed:
		if result == nil {
			return nil, false, errExecutionReceipt
		}
		confirmed, err := executionConfirmed(command, result.Confirmed)
		if err != nil {
			return nil, false, err
		}
		return confirmed, true, nil
	case *pb.ExecutionLookupResponse_NotObserved:
		if result != nil && result.NotObserved != nil {
			return nil, false, nil
		}
	}
	return nil, false, errExecutionReceipt
}
