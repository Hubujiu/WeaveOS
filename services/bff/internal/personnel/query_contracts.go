package personnel

// Q36 wire declarations only. OpenAPI and the approved PLAN define validation;
// these declarations do not install routes, SQL, locks, Redis state or behavior.
import (
	"encoding/json"
	"time"
)

type QuerySort struct {
	Key       string `json:"key"` // only occurredAt in this iteration
	Direction string `json:"direction"`
}

type QueryRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type CalendarDateValue struct {
	Date     string `json:"date"`
	TimeZone string `json:"timeZone"`
}

// Each raw child must decode to exactly one group or allowlisted condition.
// Use a strict decoder and global depth/leaf/byte bounds; RawMessage preserves
// explicit JSON null and timestamp strings without float64/interface coercion.
type FilterGroup struct {
	Operator string            `json:"operator"`
	Children []json.RawMessage `json:"children"`
}

type FilterCondition struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
}

type QueryOptions struct {
	Filter       *FilterGroup
	QueryVersion string
}

type MemberQueryInput struct {
	MemberQuery
	QueryOptions
}

type EventQueryInput struct {
	FrozenTimeBounds []QueryRange `json:"-"`
	EventQuery
	QueryOptions
	Sort *QuerySort
}

// POST search wire bodies; the core query input remains transport-independent.
// Decode with route-local raw 64KiB bound, then apply canonical filter 16KiB.
type MemberSearchInput struct {
	Page         int          `json:"page,omitempty"`
	PageSize     int          `json:"pageSize,omitempty"`
	Search       string       `json:"search,omitempty"`
	DepartmentID string       `json:"departmentId,omitempty"`
	IdentityID   string       `json:"identityId,omitempty"`
	Filter       *FilterGroup `json:"filter,omitempty"`
	QueryVersion string       `json:"queryVersion,omitempty"`
}

type EventSearchInput struct {
	Page          int          `json:"page,omitempty"`
	PageSize      int          `json:"pageSize,omitempty"`
	Search        string       `json:"search,omitempty"`
	Action        string       `json:"action,omitempty"`
	From          *time.Time   `json:"from,omitempty"`
	To            *time.Time   `json:"to,omitempty"`
	Filter        *FilterGroup `json:"filter,omitempty"`
	QueryVersion  string       `json:"queryVersion,omitempty"`
	SortBy        string       `json:"sortBy,omitempty"`
	SortDirection string       `json:"sortDirection,omitempty"`
}

type ActivityDisplay struct {
	Action  string `json:"action"`
	Object  string `json:"object"`
	Detail  string `json:"detail"`
	Outcome string `json:"outcome"`
}

type QueryActivity struct {
	Activity
	Display ActivityDisplay `json:"display"`
}

type QueryPage[T any] struct {
	Page[T]
	QueryVersion string     `json:"queryVersion"`
	Sort         *QuerySort `json:"sort"` // required null for members/default order
}

type MembersQueryPage = QueryPage[Member]

type EventsQueryPage struct {
	QueryPage[QueryActivity]
	Range QueryRange `json:"range"`
}

type DraftReference struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type QueryWriteGuard struct {
	QueryVersion string          `json:"queryVersion"`
	DraftRef     *DraftReference `json:"draftRef,omitempty"`
}

type DraftAcknowledgement struct {
	DraftRef *DraftReference `json:"draftRef,omitempty"`
}

type DraftKind string

const (
	DraftMemberIdentities DraftKind = "member-identities"
	DraftMemberGroups     DraftKind = "member-groups"
	DraftDepartment       DraftKind = "department"
	DraftIdentity         DraftKind = "identity"
	DraftTemplate         DraftKind = "template"
)

type DraftMemberIdentitiesPayload struct {
	IdentityIDs []string `json:"identityIds"`
}

type DraftMemberGroupsPayload struct {
	Operation          string  `json:"operation"`
	DepartmentID       *string `json:"departmentId"`
	SourceDepartmentID *string `json:"sourceDepartmentId"`
}

type DraftDepartmentPayload struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

type DraftIdentityPayload struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	TemplateIDs     []string `json:"templateIds"`
	PermissionCodes []string `json:"permissionCodes"`
}

type DraftTemplatePayload struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	PermissionCodes []string `json:"permissionCodes"`
}

type DraftCreateInput struct {
	Kind        DraftKind       `json:"kind"`
	TargetID    *string         `json:"targetId"`
	BaseVersion *int64          `json:"baseVersion"`
	Payload     json.RawMessage `json:"payload"`
}

type DraftUpdateInput struct {
	Version int64           `json:"version"`
	Payload json.RawMessage `json:"payload"`
}

type PersonnelDraftSummary struct {
	ID          string    `json:"id"`
	Kind        DraftKind `json:"kind"`
	TargetID    *string   `json:"targetId"`
	BaseVersion *int64    `json:"baseVersion"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type PersonnelDraft struct {
	PersonnelDraftSummary
	Payload json.RawMessage `json:"payload"`
}

type PersonnelDraftList struct {
	Items []PersonnelDraftSummary `json:"items"`
}
