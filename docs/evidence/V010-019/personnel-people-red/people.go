package personnel

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"time"
)

type Department struct {
	ID            string  `json:"id"`
	ParentID      *string `json:"parentId"`
	Name          string  `json:"name"`
	IsRoot        bool    `json:"isRoot"`
	Version       int64   `json:"version"`
	MemberCount   int64   `json:"memberCount"`
	ChildrenCount int64   `json:"childrenCount"`
}
type DepartmentInput struct {
	Name, ParentID string
	Version        int64
}
type Member struct {
	User
	Status         string       `json:"status"`
	BootstrapAdmin bool         `json:"bootstrapAdmin"`
	Version        int64        `json:"version"`
	DepartmentIDs  []string     `json:"departmentIds"`
	IdentityIDs    []string     `json:"identityIds"`
	Departments    []Department `json:"departments"`
	Identities     []Definition `json:"identities"`
	Permissions    []Permission `json:"permissions"`
}
type MemberQuery struct {
	PageQuery
	DepartmentID, IdentityID string
}
type GroupInput struct {
	Operation, DepartmentID, SourceDepartmentID string
	Version                                     int64
}
type Activity struct {
	ID           string         `json:"id"`
	OccurredAt   time.Time      `json:"occurredAt"`
	ActorAccount string         `json:"actorAccount"`
	Action       string         `json:"action"`
	ObjectType   *string        `json:"objectType"`
	ObjectID     *string        `json:"objectId"`
	Summary      map[string]any `json:"summary"`
	Outcome      string         `json:"outcome"`
}
type EventQuery struct {
	PageQuery
	Action   string
	From, To time.Time
}

func (a *Application) Departments(context.Context, session.Principal) ([]Department, error) {
	return nil, ErrNotImplemented
}
func (a *Application) SaveDepartment(context.Context, session.Principal, string, DepartmentInput, RequestMetadata) (Department, error) {
	return Department{}, ErrNotImplemented
}
func (a *Application) DeleteDepartment(context.Context, session.Principal, string, int64, RequestMetadata) error {
	return ErrNotImplemented
}
func (a *Application) GetMember(context.Context, session.Principal, string) (Member, error) {
	return Member{}, ErrNotImplemented
}
func (a *Application) ListMembers(context.Context, session.Principal, MemberQuery) (Page[Member], error) {
	return Page[Member]{}, ErrNotImplemented
}
func (a *Application) SetMemberIdentities(context.Context, session.Principal, string, []string, int64, RequestMetadata) (Member, error) {
	return Member{}, ErrNotImplemented
}
func (a *Application) ChangeMemberGroups(context.Context, session.Principal, string, GroupInput, RequestMetadata) (Member, error) {
	return Member{}, ErrNotImplemented
}
func (a *Application) Events(context.Context, session.Principal, EventQuery) (Page[Activity], error) {
	return Page[Activity]{}, ErrNotImplemented
}
func (a *Application) Catalog(context.Context, session.Principal) ([]Permission, error) {
	return nil, ErrNotImplemented
}
