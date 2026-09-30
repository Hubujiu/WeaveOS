package personnel

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID      string `json:"id"`
	Account string `json:"account"`
}
type Source struct {
	IdentityID   string `json:"identityId"`
	IdentityName string `json:"identityName"`
	TemplateID   string `json:"templateId,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
}
type Permission struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	AppID    *string  `json:"appId"`
	Enabled  bool     `json:"enabled"`
	Sources  []Source `json:"sources"`
}
type Definition struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Version            int64    `json:"version"`
	PermissionCodes    []string `json:"permissionCodes"`
	TemplateIDs        []string `json:"templateIds,omitempty"`
	AffectedMembers    int64    `json:"affectedMembers"`
	AffectedIdentities int64    `json:"affectedIdentities"`
}
type Access struct {
	User            User         `json:"user"`
	BootstrapAdmin  bool         `json:"bootstrapAdmin"`
	PersonnelManage bool         `json:"personnelManage"`
	Identities      []Definition `json:"identities"`
	Permissions     []Permission `json:"permissions"`
	Applications    []Permission `json:"applications"`
}

var ErrDenied = errors.New("personnel permission denied")
var ErrConflict = errors.New("personnel configuration conflict")
var ErrMissing = errors.New("personnel object absent")
var ErrInvalid = errors.New("invalid personnel argument")
var ErrNotImplemented = errors.New("personnel behavior not implemented")

type Application struct{ Pool *pgxpool.Pool }

func (a *Application) Me(context.Context, session.Principal) (Access, error) {
	return Access{}, ErrNotImplemented
}
func (a *Application) AuthorizeWrite(context.Context, pgx.Tx, session.Principal) error {
	return ErrNotImplemented
}
func (a *Application) AllowApplication(context.Context, session.Principal, string) error {
	return ErrNotImplemented
}
