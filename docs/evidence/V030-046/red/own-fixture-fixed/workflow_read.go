package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowInstanceSearchRequest struct {
	AppID, ViewID, RecordID, QueryVersion string
	Page, PageSize                        int
}
type WorkflowInstanceSummary struct {
	ID, FlowID, Name, InitiatorID, State, CreatedAt, UpdatedAt string
	DefinitionVersion, Sequence                                int64
}
type WorkflowInstanceSearchResult struct {
	Items          []WorkflowInstanceSummary
	Total          int64
	Page, PageSize int
	QueryVersion   string
}

func (s *Service) SearchRecordWorkflows(ctx context.Context, p session.Principal, req WorkflowInstanceSearchRequest) (WorkflowInstanceSearchResult, error) {
	return WorkflowInstanceSearchResult{}, ErrUnavailable
}
