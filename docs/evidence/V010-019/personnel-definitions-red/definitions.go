package personnel

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type DefinitionKind string

const Identity DefinitionKind = "identity"
const Template DefinitionKind = "template"

type DefinitionInput struct {
	Name, Description            string
	Version                      int64
	PermissionCodes, TemplateIDs []string
}
type RequestMetadata struct{ RequestID, ClientIP, UserAgent string }
type PageQuery struct {
	Page, PageSize int
	Search         string
}
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func (a *Application) SaveDefinition(context.Context, session.Principal, DefinitionKind, string, DefinitionInput, RequestMetadata) (Definition, error) {
	return Definition{}, ErrNotImplemented
}
func (a *Application) GetDefinition(context.Context, session.Principal, DefinitionKind, string) (Definition, error) {
	return Definition{}, ErrNotImplemented
}
func (a *Application) ListDefinitions(context.Context, session.Principal, DefinitionKind, PageQuery) (Page[Definition], error) {
	return Page[Definition]{}, ErrNotImplemented
}
func (a *Application) DeleteDefinition(context.Context, session.Principal, DefinitionKind, string, int64, RequestMetadata) error {
	return ErrNotImplemented
}
