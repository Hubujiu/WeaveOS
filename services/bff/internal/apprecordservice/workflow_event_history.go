package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowEventHistoryRequest struct {
	AppID, ViewID, RecordID, PageToken string
	PageSize                           int
}
type WorkflowEventHistoryResult struct {
	Items         []WorkflowEventSummary
	HasMore       bool
	NextPageToken *string
}

func (s *Service) ReadWorkflowEventHistory(ctx context.Context, p session.Principal, q WorkflowEventHistoryRequest) (WorkflowEventHistoryResult, error) {
	return WorkflowEventHistoryResult{}, ErrUnavailable
}
