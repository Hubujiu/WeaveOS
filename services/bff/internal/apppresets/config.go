// Package apppresets owns private application/form-view table configurations.
package apppresets

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"reflect"
	"strings"
	"unicode/utf8"
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

// Prepare accepts only server-loaded policy/schema facts. It returns owned
// copies and signatures, never changes authorization or query projection.
func Prepare(state State, policy appaccess.Policy, fields []appquery.Field) (Prepared, error) {
	if !utf8.ValidString(state.Name) || strings.ContainsRune(state.Name, 0) || len(state.Filter) == 0 || len(state.Sort) == 0 {
		return Prepared{}, ErrInvalid
	}
	state.Name = strings.TrimSpace(state.Name)
	if n := utf8.RuneCountInString(state.Name); n < 1 || n > 100 {
		return Prepared{}, ErrInvalid
	}
	raw, e := json.Marshal(state)
	if e != nil {
		return Prepared{}, ErrInvalid
	}
	state, e = DecodeState(raw)
	if e != nil {
		return Prepared{}, e
	}
	if policy.VisibleScope() == appaccess.None {
		return Prepared{}, ErrPermission
	}
	plan, e := appquery.Compile(state.Filter, state.Sort, fields, 1)
	if e != nil {
		return Prepared{}, ErrInvalid
	}
	if !editableFilter(state.Filter) {
		return Prepared{}, ErrInvalid
	}
	if !policy.CoversRead(plan.ReferencedFields) {
		return Prepared{}, ErrPermission
	}
	kinds := make(map[string]appquery.FieldKind, len(fields))
	for _, f := range fields {
		kinds[f.ID] = f.Kind
	}
	references := map[string]appquery.FieldKind{}
	for _, id := range plan.ReferencedFields {
		references[id] = kinds[id]
	}
	checkColumn := func(id string) error {
		if systemColumn(id) {
			return nil
		}
		kind, ok := kinds[id]
		if !ok {
			return ErrInvalid
		}
		if policy.FieldScope(appaccess.Read, id) == appaccess.None {
			return ErrPermission
		}
		references[id] = kind
		return nil
	}
	for _, ids := range [][]string{state.HiddenColumnIDs, state.ColumnOrder} {
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return Prepared{}, ErrInvalid
			}
			seen[id] = true
			if e = checkColumn(id); e != nil {
				return Prepared{}, e
			}
		}
	}
	for id, width := range state.ColumnWidths {
		if width < 1 || width > 9007199254740991 {
			return Prepared{}, ErrInvalid
		}
		if e = checkColumn(id); e != nil {
			return Prepared{}, e
		}
	}
	hidden := map[string]bool{}
	for _, id := range state.HiddenColumnIDs {
		hidden[id] = true
	}
	visible := false
	for _, f := range fields {
		if !hidden[f.ID] && policy.FieldScope(appaccess.Read, f.ID) != appaccess.None {
			visible = true
			break
		}
	}
	if !visible {
		return Prepared{}, ErrInvalid
	}
	state.Filter = append(json.RawMessage(nil), plan.Canonical...)
	if len(state.Filter) == 0 {
		state.Filter = json.RawMessage("null")
	}
	var compact bytes.Buffer
	if json.Compact(&compact, state.Sort) != nil {
		return Prepared{}, ErrInvalid
	}
	state.Sort = append(json.RawMessage(nil), compact.Bytes()...)
	raw, e = json.Marshal(state)
	if e != nil || len(raw) > 32768 {
		return Prepared{}, ErrInvalid
	}
	return Prepared{State: state, Definition: raw, FieldKinds: references}, nil
}

// CheckStored reports only a public reason. Original bytes remain untouched;
// invalid DTO redaction is the caller's responsibility.
func CheckStored(raw []byte, kinds map[string]appquery.FieldKind, policy appaccess.Policy, fields []appquery.Field) Reason {
	state, e := DecodeState(raw)
	if e != nil || len(raw) > 32768 {
		return DefinitionChanged
	}
	current := map[string]appquery.FieldKind{}
	for _, f := range fields {
		current[f.ID] = f.Kind
	}
	for id, kind := range kinds {
		now, ok := current[id]
		if !ok {
			return FieldUnavailable
		}
		if now != kind {
			return DefinitionChanged
		}
	}
	prepared, e := Prepare(state, policy, fields)
	if errors.Is(e, ErrPermission) {
		return PermissionChanged
	}
	if e != nil || !reflect.DeepEqual(kinds, prepared.FieldKinds) {
		return DefinitionChanged
	}
	return ""
}
func systemColumn(id string) bool {
	switch id {
	case "id", "createdBy", "createdAt", "updatedAt", "recordVersion":
		return true
	}
	return false
}
func editableFilter(raw json.RawMessage) bool {
	if string(bytes.TrimSpace(raw)) == "null" {
		return true
	}
	type group struct {
		Operator string            `json:"operator"`
		Children []json.RawMessage `json:"children"`
	}
	var root group
	if json.Unmarshal(raw, &root) != nil {
		return false
	}
	leafGroup := func(g group) bool {
		if g.Operator != "and" || len(g.Children) == 0 {
			return false
		}
		for _, raw := range g.Children {
			var node map[string]json.RawMessage
			if json.Unmarshal(raw, &node) != nil || node["children"] != nil {
				return false
			}
		}
		return true
	}
	if root.Operator == "and" {
		return leafGroup(root)
	}
	if root.Operator != "or" || len(root.Children) == 0 {
		return false
	}
	for _, raw := range root.Children {
		var g group
		if json.Unmarshal(raw, &g) != nil || !leafGroup(g) {
			return false
		}
	}
	return true
}
