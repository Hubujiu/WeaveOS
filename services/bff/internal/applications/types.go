package applications

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrMissing = errors.New("application not found")
var ErrDenied = errors.New("application forbidden")
var ErrPolicyConflict = errors.New("application policy conflict")
var ErrOperationConflict = errors.New("application operation conflict")
var ErrUnconfirmed = errors.New("application operation outcome unconfirmed")
var ErrResourceInvalid = errors.New("application resource invalid")
var ErrInvalid = errors.New("invalid application request")
var ErrExpectedActorInvalid = errors.New("invalid expected actor constraint")
var ErrSessionChanged = errors.New("session actor does not match expected actor")

type Application struct{ Pool *pgxpool.Pool }
type App struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	OwnerUserID    string `json:"ownerUserId"`
	PolicyRevision int64  `json:"policyRevision"`
}
type Group struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	PolicyRevision int64  `json:"policyRevision"`
}
type Menu struct {
	ResourceKind string `json:"resourceKind"`
	ResourceID   string `json:"resourceId"`
}
type MemberDisplay struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	Selectable bool   `json:"selectable"`
}
type Access struct {
	AppID          string `json:"appId"`
	PolicyRevision int64  `json:"policyRevision"`
	CanEnter       bool   `json:"canEnter"`
	Menus          []Menu `json:"menus"`
}
type Grant struct {
	ResourceKind string   `json:"resourceKind"`
	ResourceID   string   `json:"resourceId"`
	Action       string   `json:"action"`
	RowScope     string   `json:"rowScope"`
	Fields       []string `json:"fields"`
}
type Input struct {
	Name                   string   `json:"name,omitempty"`
	Enabled                bool     `json:"enabled"`
	MemberIDs              []string `json:"memberIds,omitempty"`
	Grants                 []Grant  `json:"grants,omitempty"`
	OperationID            string   `json:"operationId"`
	ExpectedPolicyRevision int64    `json:"expectedPolicyRevision,omitempty"`
}
type Result struct {
	Status   int
	Location string
	Data     json.RawMessage
}
type Operation struct {
	OperationID string          `json:"operationId"`
	Status      string          `json:"status"`
	HTTPStatus  int             `json:"httpStatus"`
	Location    string          `json:"location"`
	Result      json.RawMessage `json:"result"`
}
type Metadata struct{ RequestID, ClientIP, UserAgent string }
