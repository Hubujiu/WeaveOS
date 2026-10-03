package appstructure

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/jackc/pgx/v5"
)

var ErrUnavailable = errors.New("definition adapter unavailable")

type Error struct {
	Code string
	Data any
}

func (e *Error) Error() string { return e.Code }

type DependencyRegistry interface {
	Available(context.Context, pgx.Tx, string) error
}
type ReferenceValidator interface {
	Validate(context.Context, pgx.Tx, string, []string) error
}
type LocalRegistry struct{}

func (LocalRegistry) Available(c context.Context, tx pgx.Tx, id string) error {
	var ready bool
	e := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM applications.logical_tables WHERE id=$1) AND to_regclass('applications.table_field_dependencies') IS NOT NULL", id).Scan(&ready)
	if e != nil || !ready {
		return ErrUnavailable
	}
	return nil
}

type Directory struct {
	ID       string  `json:"id"`
	AppID    string  `json:"appId"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
	Position int     `json:"position"`
}
type Table struct {
	ID            string  `json:"id"`
	AppID         string  `json:"appId"`
	Name          string  `json:"name"`
	DirectoryID   *string `json:"directoryId"`
	Position      int     `json:"position"`
	SchemaVersion int64   `json:"schemaVersion"`
	SchemaReady   bool    `json:"schemaReady"`
}
type Form struct {
	ID          string  `json:"id"`
	AppID       string  `json:"appId"`
	TableID     string  `json:"tableId"`
	Name        string  `json:"name"`
	DirectoryID *string `json:"directoryId"`
	Position    int     `json:"position"`
	ViewVersion int64   `json:"viewVersion"`
}
type Capabilities struct {
	CanManageDefinition bool `json:"canManageDefinition"`
}
type Structure struct {
	AppID            string       `json:"appId"`
	StructureVersion int64        `json:"structureVersion"`
	Directories      []Directory  `json:"directories"`
	Tables           []Table      `json:"tables"`
	Forms            []Form       `json:"forms"`
	Capabilities     Capabilities `json:"capabilities"`
}
type SystemField struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	ReadOnly bool   `json:"readOnly"`
}
type Definition struct {
	AppID        string                 `json:"appId"`
	Table        Table                  `json:"table"`
	Form         Form                   `json:"form"`
	Fields       []appfields.Field      `json:"fields"`
	SystemFields []SystemField          `json:"systemFields"`
	Layout       []appfields.LayoutNode `json:"layout"`
	Capabilities Capabilities           `json:"capabilities"`
}
type Source struct {
	Kind    string `json:"kind"`
	TableID string `json:"tableId,omitempty"`
}
type OptionMapping struct {
	FieldID      string  `json:"fieldId"`
	FromOptionID string  `json:"fromOptionId"`
	ToOptionID   *string `json:"toOptionId"`
}
type Input struct {
	OperationID              string                 `json:"operationId,omitempty"`
	Name                     string                 `json:"name,omitempty"`
	ParentID                 *string                `json:"parentId,omitempty"`
	DirectoryID              *string                `json:"directoryId,omitempty"`
	Position                 int                    `json:"position,omitempty"`
	ExpectedStructureVersion int64                  `json:"expectedStructureVersion,omitempty"`
	Source                   *Source                `json:"source,omitempty"`
	ExpectedSchemaVersion    int64                  `json:"expectedSchemaVersion"`
	ExpectedViewVersion      int64                  `json:"expectedViewVersion"`
	Fields                   []appfields.Field      `json:"fields,omitempty"`
	Layout                   []appfields.LayoutNode `json:"layout,omitempty"`
	OptionMappings           []OptionMapping        `json:"optionMappings,omitempty"`
	ConfirmationToken        *string                `json:"confirmationToken,omitempty"`
}
type Dependency struct {
	FieldID    string `json:"fieldId"`
	Kind       string `json:"kind"`
	ResourceID string `json:"resourceId"`
}
type Impact struct {
	FieldID     string  `json:"fieldId"`
	Kind        string  `json:"kind"`
	NonNullRows int64   `json:"nonNullRows"`
	OptionID    *string `json:"optionId"`
}
type SchemaChange struct {
	Kind       string  `json:"kind"`
	FieldID    string  `json:"fieldId"`
	BeforeKind *string `json:"beforeKind"`
	AfterKind  *string `json:"afterKind"`
}
type ChangePlan struct {
	SchemaChanges   []SchemaChange `json:"schemaChanges"`
	MetadataChanged bool           `json:"metadataChanged"`
	LayoutChanged   bool           `json:"layoutChanged"`
}
type Issue struct {
	Code     string   `json:"code"`
	FieldIDs []string `json:"fieldIds"`
}
type Confirmation struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}
type Preflight struct {
	AppID              string        `json:"appId"`
	TableID            string        `json:"tableId"`
	ViewID             string        `json:"viewId"`
	SchemaVersion      int64         `json:"schemaVersion"`
	ViewVersion        int64         `json:"viewVersion"`
	DataRevision       int64         `json:"dataRevision"`
	DependencyRevision int64         `json:"dependencyRevision"`
	Plan               ChangePlan    `json:"plan"`
	Impacts            []Impact      `json:"impacts"`
	Dependencies       []Dependency  `json:"dependencies"`
	BlockingIssues     []Issue       `json:"blockingIssues"`
	SaveAllowed        bool          `json:"saveAllowed"`
	Confirmation       *Confirmation `json:"confirmation"`
}
