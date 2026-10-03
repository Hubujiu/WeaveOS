package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnavailable = errors.New("record service unavailable")

type Service struct {
	Pool    *pgxpool.Pool
	Queries *querycontext.Store
}

type SearchRequest struct {
	AppID, ViewID, QueryVersion string
	Filter, Sort, QuickSearch   json.RawMessage
	Page, PageSize              int64
}

type ReferenceDisplay struct {
	ID, Label string
	Deleted   bool
}
type Record struct {
	ID, CreatedBy        string
	CreatedAt, UpdatedAt string
	RecordVersion        int64
	Values               map[string]any
	ReferenceDisplays    map[string]map[string]ReferenceDisplay
}
type SearchResult struct {
	Items                                      []Record
	Total                                      int64
	QueryVersion                               string
	SchemaVersion, ViewVersion, PolicyRevision int64
}

func (s *Service) Search(context.Context, session.Principal, SearchRequest) (SearchResult, error) {
	return SearchResult{}, ErrUnavailable
}
