package personnel

import (
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
