package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowCandidateRequest struct {
	WorkflowTaskRequest
	FieldID, Query, PageToken string
	PageSize                  int
}
type WorkflowCandidateResult struct {
	Items      []any                   `json:"items"`
	Pagination appstructure.Pagination `json:"-"`
}

// Compilation-only declaration for the independent P5 real storage RED.
func (s *Service) WorkflowTaskCandidates(ctx context.Context, principal session.Principal, req WorkflowCandidateRequest) (WorkflowCandidateResult, error) {
	return WorkflowCandidateResult{}, ErrUnavailable
}
