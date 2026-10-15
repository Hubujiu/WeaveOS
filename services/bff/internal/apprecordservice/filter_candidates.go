package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Declarations only for first real P26 permission/cursor tests. No candidate
// behavior is implemented until the target tests have actually failed in CI.
type FilterCandidatesRequest struct {
	AppID, ViewID, FieldID, Q, PageToken string
	PageSize                             int
}
type FilterCandidatesResult struct {
	Items         []ReferenceDisplay `json:"items"`
	NextPageToken string             `json:"-"`
	HasMore       bool               `json:"-"`
}

func (s *Service) FilterCandidates(ctx context.Context, p session.Principal, req FilterCandidatesRequest) (FilterCandidatesResult, error) {
	return FilterCandidatesResult{}, ErrUnavailable
}
