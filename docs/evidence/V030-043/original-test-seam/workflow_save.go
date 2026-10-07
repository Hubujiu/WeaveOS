package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Declaration-only V043 seam: no behavior until Root's real storage RED.
type WorkflowTaskSaveRequest struct {
	WorkflowTaskRequest
	OperationID, BasisToken string
	Changes                 map[string]any
}

func (s *Service) SaveWorkflowTask(context.Context, session.Principal, WorkflowTaskSaveRequest, applications.Metadata) (MutationResult, error) {
	return MutationResult{}, ErrUnavailable
}
