package personnel

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const presetCanonicalBytes = 32768

func validPresetView(view string) bool { return view == "members" || view == "events" }
func presetColumns(view string) []string {
	if view == "members" {
		return []string{"account", "departments", "identities", "personnelManage"}
	}
	if view == "events" {
		return []string{"occurredAt", "actorAccount", "action", "object", "detail", "outcome"}
	}
	return nil
}
func canonicalPreset(view, name string, raw json.RawMessage, hidden []string, schema int) (string, json.RawMessage, error) {
	if schema != 1 {
		return "", nil, ErrPresetSchema
	}
	name = strings.TrimSpace(name)
	if !validPresetView(view) || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || !validName(name) || hidden == nil {
		return "", nil, ErrInvalid
	}
	columns := presetColumns(view)
	seen := map[string]bool{}
	if len(hidden) >= len(columns) {
		return "", nil, ErrInvalid
	}
	for _, id := range hidden {
		known := false
		for _, col := range columns {
			if id == col {
				known = true
				break
			}
		}
		if !known || seen[id] {
			return "", nil, ErrInvalid
		}
		seen[id] = true
	}
	canonical := json.RawMessage("null")
	if len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		plan, e := CompileFilter(view, raw, 1)
		if e != nil {
			return "", nil, e
		}
		var root FilterGroup
		if json.Unmarshal(plan.Canonical, &root) != nil {
			return "", nil, ErrInvalid
		}
		if !presetEditableGroup(root) {
			return "", nil, ErrInvalid
		}
		canonical = plan.Canonical
	}
	content, e := projectionJSON(map[string]any{"view": view, "name": name, "filter": canonical, "hiddenColumnIds": hidden, "schemaVersion": schema})
	if e != nil || len(content) > presetCanonicalBytes {
		return "", nil, ErrInvalid
	}
	return name, canonical, nil
}

// Presets have a narrower editable shape than the recursive query engine.
// Reject unsupported trees; never flatten or duplicate them into DNF.
func presetEditableGroup(root FilterGroup) bool {
	leafGroup := func(group FilterGroup) bool {
		if group.Operator != "and" || len(group.Children) == 0 {
			return false
		}
		for _, raw := range group.Children {
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
		var group FilterGroup
		if json.Unmarshal(raw, &group) != nil || !leafGroup(group) {
			return false
		}
	}
	return true
}

// Only bounded ID references, against the current authorized store; no SQL from
// user input. Configurations do not own their referenced business objects.
func validatePresetReferences(ctx context.Context, tx pgx.Tx, raw json.RawMessage) error {
	if string(raw) == "null" || len(raw) == 0 {
		return nil
	}
	var visit func(json.RawMessage) error
	visit = func(raw json.RawMessage) error {
		var n struct {
			Field    string            `json:"field"`
			Value    json.RawMessage   `json:"value"`
			Children []json.RawMessage `json:"children"`
		}
		if json.Unmarshal(raw, &n) != nil {
			return ErrInvalid
		}
		if n.Field == "identityIds" || n.Field == "departmentIds" {
			var id string
			if json.Unmarshal(n.Value, &id) != nil {
				return ErrInvalid
			}
			query := `SELECT EXISTS(SELECT 1 FROM personnel.identities WHERE id=$1)`
			if n.Field == "departmentIds" {
				query = `SELECT EXISTS(SELECT 1 FROM personnel.departments WHERE id=$1)`
			}
			var exists bool
			if e := tx.QueryRow(ctx, query, id).Scan(&exists); e != nil {
				return e
			}
			if !exists {
				return ErrInvalid
			}
		}
		for _, child := range n.Children {
			if e := visit(child); e != nil {
				return e
			}
		}
		return nil
	}
	return visit(raw)
}
