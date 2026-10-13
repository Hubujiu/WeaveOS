package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Test-first declaration for the current-runtime slice, not archived rounds.
type WorkflowStartedRequest struct {
	Page, PageSize int
	QueryVersion   string
}
type WorkflowStartedItem struct {
	AppID    string                  `json:"appId"`
	ViewID   string                  `json:"viewId"`
	RecordID string                  `json:"recordId"`
	Instance WorkflowInstanceSummary `json:"instance"`
}
type WorkflowStartedResult struct {
	Items        []WorkflowStartedItem `json:"items"`
	Total        int64                 `json:"total"`
	Page         int                   `json:"page"`
	PageSize     int                   `json:"pageSize"`
	QueryVersion string                `json:"queryVersion"`
}

func (s *Service) SearchWorkflowStarted(context.Context, session.Principal, WorkflowStartedRequest) (WorkflowStartedResult, error) {
	return WorkflowStartedResult{}, ErrUnavailable
}
