// Package apppresets owns private application/form-view table configurations.
package apppresets

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
)

var ErrInvalid = errors.New("invalid application table preset")
var ErrPermission = errors.New("application preset permission changed")

type Reason string

const (
	FieldUnavailable  Reason = "FIELD_UNAVAILABLE"
	PermissionChanged Reason = "PERMISSION_CHANGED"
	DefinitionChanged Reason = "DEFINITION_CHANGED"
)

type State struct {
	Name            string           `json:"name"`
	Filter          json.RawMessage  `json:"filter"`
	Sort            json.RawMessage  `json:"sort"`
	HiddenColumnIDs []string         `json:"hiddenColumnIds"`
	ColumnOrder     []string         `json:"columnOrder"`
	ColumnWidths    map[string]int64 `json:"columnWidths"`
}
type Prepared struct {
	State      State
	Definition json.RawMessage
	FieldKinds map[string]appquery.FieldKind
}

// Minimal RED-only declarations. No behavior has been implemented.
func DecodeState(raw []byte) (State, error) { return State{}, ErrInvalid }
func Prepare(state State, policy appaccess.Policy, fields []appquery.Field) (Prepared, error) {
	return Prepared{}, ErrInvalid
}
func CheckStored(raw []byte, kinds map[string]appquery.FieldKind, policy appaccess.Policy, fields []appquery.Field) Reason {
	return DefinitionChanged
}
