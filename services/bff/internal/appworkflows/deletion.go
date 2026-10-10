package appworkflows

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
)

type DeletionClient interface {
	DeleteFlow(context.Context, *pb.FlowDeletionRequest) (*pb.FlowDeletionReceipt, error)
	LookupFlowDeletion(context.Context, *pb.FlowDeletionRequest) (*pb.FlowDeletionLookupResponse, error)
}

// Compilation-only R2 declaration; no worker or cleanup exists yet.
func (a *Application) DispatchDeletion(ctx context.Context) (bool, error) {
	return false, workflowcatalog.ErrNotReady
}
