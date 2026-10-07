package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowLifecycleActionRequest struct {
	WorkflowLifecycleRequest
	OperationID, Action, BasisToken, TargetNodeID string
}

func (s *Service) AcceptWorkflowLifecycle(ctx context.Context, p session.Principal, req WorkflowLifecycleActionRequest, metadata applications.Metadata) (WorkflowOperationResult, error) {
	return WorkflowOperationResult{}, ErrUnavailable
}
