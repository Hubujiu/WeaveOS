package apprecordservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ErrUnavailable = errors.New("record service unavailable")

type Service struct {
	RuntimeReady           func(context.Context) error
	Pool                   *pgxpool.Pool
	Queries                *querycontext.Store
	WorkflowBases          *querycontext.Store
	WorkflowLifecycleBases *querycontext.Store
	Limits                 appschema.Limits
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
	return &Service{Pool: pool, Queries: querycontext.NewStore(client, "applications", generation, policy), WorkflowBases: newWorkflowBasisStore(client, generation), WorkflowLifecycleBases: newWorkflowLifecycleStore(client, generation)}
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
	AppID             string                                 `json:"appId"`
	TableID           string                                 `json:"tableId"`
	ViewID            string                                 `json:"viewId"`
	CreatedBy         string                                 `json:"createdBy"`
	CreatedAt         string                                 `json:"createdAt"`
	UpdatedAt         string                                 `json:"updatedAt"`
	RecordVersion     int64                                  `json:"recordVersion"`
	SchemaVersion     int64                                  `json:"schemaVersion"`
	Values            map[string]any                         `json:"values"`
	ReferenceDisplays map[string]map[string]ReferenceDisplay `json:"referenceDisplays"`
}
type SearchResult struct {
	Items          []Record        `json:"items"`
	Total          int64           `json:"total"`
	Page           int64           `json:"page"`
	PageSize       int64           `json:"pageSize"`
	Sort           json.RawMessage `json:"sort"`
	QueryVersion   string          `json:"queryVersion"`
	SchemaVersion  int64           `json:"schemaVersion"`
	ViewVersion    int64           `json:"viewVersion"`
	PolicyRevision int64           `json:"-"`
}

type CreateRequest struct {
	AppID, ViewID, OperationID string
	QueryVersion               string
	ExpectedSchemaVersion      int64
	Values                     map[string]any
	DraftRef                   *DraftRef
}
type DraftRef struct {
	ID           string `json:"id"`
	DraftVersion int64  `json:"draftVersion"`
}
type EditRequest struct {
	AppID, ViewID, RecordID, OperationID         string
	QueryVersion                                 string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Changes                                      map[string]any
	DraftRef                                     *DraftRef
}
type DraftCreateRequest struct {
	AppID, ViewID, OperationID string
	SchemaVersion              int64
	TargetRecordID             *string
	BaseRecordVersion          *int64
	Values                     appdrafts.Values
}
type DraftUpdateRequest struct {
	AppID, ViewID, DraftID, OperationID string
	ExpectedDraftVersion                int64
	Changes                             appdrafts.Values
	RemoveFieldIDs                      []string
}
type DraftDiscardRequest struct {
	AppID, ViewID, DraftID, OperationID string
	ExpectedDraftVersion                int64
}
type DraftListRequest struct {
	AppID, ViewID, PageToken string
	PageSize                 int
}
type MutationResult struct {
	OperationID   string `json:"operationId"`
	ID            string `json:"id"`
	RecordVersion int64  `json:"recordVersion"`
	SchemaVersion int64  `json:"schemaVersion"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}
