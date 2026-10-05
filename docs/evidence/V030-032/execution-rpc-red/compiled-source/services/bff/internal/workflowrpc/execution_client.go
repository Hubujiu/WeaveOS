package workflowrpc

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"google.golang.org/grpc"
	"time"
)

// Root declaration only. Tests must compile and expose missing behavior.
var errExecutionUnimplemented = errors.New("execution RPC is not implemented")

type ExecutionConfirmed struct {
	Receipt flowcommands.Receipt
	Result  flowcommands.ExecutionResult
}
type ExecutionClient struct{}

func NewExecutionClient(conn grpc.ClientConnInterface, timeout time.Duration) (*ExecutionClient, error) {
	return nil, errExecutionUnimplemented
}
func (c *ExecutionClient) Execute(ctx context.Context, command flowcommands.Command, payload flowcommands.ExecutionPayload) (*ExecutionConfirmed, error) {
	return nil, errExecutionUnimplemented
}
func (c *ExecutionClient) Lookup(ctx context.Context, command flowcommands.Command) (*ExecutionConfirmed, bool, error) {
	return nil, false, errExecutionUnimplemented
}
func (c *ExecutionClient) EstablishNoEffect(ctx context.Context, command flowcommands.Command, payload flowcommands.ExecutionPayload) (*ExecutionConfirmed, error) {
	return nil, errExecutionUnimplemented
}
