package workflowrpc

import (
	"context"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
)

// Compile-only declarations before the frozen R3 client oracle is run.
func (c *Client) DeleteFlow(ctx context.Context, in *pb.FlowDeletionRequest) (*pb.FlowDeletionReceipt, error) {
	return nil, nil
}
func (c *Client) LookupFlowDeletion(ctx context.Context, in *pb.FlowDeletionRequest) (*pb.FlowDeletionLookupResponse, error) {
	return nil, nil
}
