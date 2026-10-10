// Package apptemplates validates portable structure; parsing grants no authority.
package apptemplates

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
)

var ErrInvalid = errors.New("invalid structure template")

type Application struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Directory struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
	Position int     `json:"position"`
}
type Table struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DirectoryID *string           `json:"directoryId"`
	Position    int               `json:"position"`
	Fields      []appfields.Field `json:"fields"`
}
type Form struct {
	ID          string                 `json:"id"`
	TableID     string                 `json:"tableId"`
	Name        string                 `json:"name"`
	DirectoryID *string                `json:"directoryId"`
	Position    int                    `json:"position"`
	Layout      []appfields.LayoutNode `json:"layout"`
}
type Graph struct {
	Version int    `json:"version"`
	Nodes   []Node `json:"nodes"`
	Edges   []Edge `json:"edges"`
}
type Node struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Approval  *Approval       `json:"approval,omitempty"`
	Condition json.RawMessage `json:"condition,omitempty"`
}
type Approval struct {
	Mode             string   `json:"mode"`
	AssigneeIDs      []string `json:"assigneeIds"`
	EditableFieldIDs []string `json:"editableFieldIds"`
}
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Branch string `json:"branch,omitempty"`
}
type Workflow struct {
	ID            string                    `json:"id"`
	TableID       string                    `json:"tableId"`
	ViewID        string                    `json:"viewId"`
	Name          string                    `json:"name"`
	Graph         Graph                     `json:"graph"`
	AllowWithdraw bool                      `json:"allowWithdraw"`
	Triggers      []workflowcatalog.Trigger `json:"triggers"`
}
type Grant struct {
	ResourceKind string   `json:"resourceKind"`
	ResourceID   string   `json:"resourceId"`
	Action       string   `json:"action"`
	RowScope     string   `json:"rowScope"`
	Fields       []string `json:"fields"`
}
type PermissionGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	MemberIDs []string `json:"memberIds"`
	Grants    []Grant  `json:"grants"`
}
type Manifest struct {
	Format           string            `json:"format"`
	Version          int               `json:"version"`
	Application      Application       `json:"application"`
	Directories      []Directory       `json:"directories"`
	Tables           []Table           `json:"tables"`
	Forms            []Form            `json:"forms"`
	Workflows        []Workflow        `json:"workflows"`
	PermissionGroups []PermissionGroup `json:"permissionGroups"`
}
