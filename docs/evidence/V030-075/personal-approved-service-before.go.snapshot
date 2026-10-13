package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Declaration only: real storage acceptance must observe missing behavior first.
type WorkflowApprovalsRequest struct {
	Page, PageSize int
	QueryVersion   string
}
type WorkflowApprovalsItem struct {
	AppID    string               `json:"appId"`
	ViewID   string               `json:"viewId"`
	RecordID string               `json:"recordId"`
	Event    WorkflowEventSummary `json:"event"`
}
type WorkflowApprovalsResult struct {
	Items        []WorkflowApprovalsItem `json:"items"`
	Total        int64                   `json:"total"`
	Page         int                     `json:"page"`
	PageSize     int                     `json:"pageSize"`
	QueryVersion string                  `json:"queryVersion"`
}

func (s *Service) SearchWorkflowApprovals(context.Context, session.Principal, WorkflowApprovalsRequest) (WorkflowApprovalsResult, error) {
	return WorkflowApprovalsResult{}, ErrUnavailable
}
