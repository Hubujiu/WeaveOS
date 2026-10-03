package apprecordservice

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ErrUnavailable = errors.New("record service unavailable")

type Service struct {
	Pool    *pgxpool.Pool
	Queries *querycontext.Store
}

func New(pool *pgxpool.Pool, client redis.UniversalClient, generation string) *Service {
	policy := querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			parts := strings.Split(m.View, ":")
			if len(parts) != 4 || parts[0] != "apprecords" {
				return false
			}
			for _, id := range parts[1:] {
				if !appfields.ValidID(id) {
					return false
				}
			}
			var c criteria
			if len(m.Criteria) == 0 || m.Criteria[0] != '{' || !closedJSON(m.Criteria, &c) || len(c.Filter) == 0 || len(c.Sort) == 0 {
				return false
			}
			var r revisions
			return closedJSON(m.Revision, &r) && validRevisions(r)
		},
		Forward: func(view string, old, next json.RawMessage) bool {
			if !strings.HasPrefix(view, "apprecords:") {
				return false
			}
			var a, b revisions
			return closedJSON(old, &a) && closedJSON(next, &b) && validRevisions(a) && validRevisions(b) &&
				b.Data >= a.Data && b.Dependency >= a.Dependency && b.Schema >= a.Schema && b.View >= a.View && b.Policy >= a.Policy && b.Source >= a.Source
		},
	}
	return &Service{Pool: pool, Queries: querycontext.NewStore(client, "applications", generation, policy)}
}

func closedJSON(raw json.RawMessage, out any) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return false
	}
	var extra any
	return d.Decode(&extra) == io.EOF
}
func validRevisions(r revisions) bool {
	const max = 9007199254740991
	return r.Data >= 0 && r.Data <= max && r.Dependency >= 0 && r.Dependency <= max && r.Schema >= 1 && r.Schema <= max && r.View >= 0 && r.View <= max && r.Policy >= 1 && r.Policy <= max && r.Source >= 0 && r.Source <= max
}

type SearchRequest struct {
	AppID, ViewID, QueryVersion string
	Filter, Sort, QuickSearch   json.RawMessage
	Page, PageSize              int64
}

type ReferenceDisplay struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Deleted bool   `json:"deleted"`
}
type Record struct {
	ID                string                                 `json:"id"`
	CreatedBy         string                                 `json:"createdBy"`
	CreatedAt         string                                 `json:"createdAt"`
	UpdatedAt         string                                 `json:"updatedAt"`
	RecordVersion     int64                                  `json:"recordVersion"`
	Values            map[string]any                         `json:"values"`
	ReferenceDisplays map[string]map[string]ReferenceDisplay `json:"referenceDisplays"`
}
type SearchResult struct {
	Items                                      []Record
	Total                                      int64
	QueryVersion                               string
	SchemaVersion, ViewVersion, PolicyRevision int64
}

type CreateRequest struct {
	AppID, ViewID, OperationID string
	ExpectedSchemaVersion      int64
	Values                     map[string]any
}
type MutationResult struct {
	OperationID, ID              string
	RecordVersion, SchemaVersion int64
	CreatedAt, UpdatedAt         string
}

func (s *Service) Create(context.Context, session.Principal, CreateRequest, applications.Metadata) (MutationResult, error) {
	return MutationResult{}, ErrUnavailable
}
