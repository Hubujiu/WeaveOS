package apptemplates

import (
	"bytes"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
)

type identityKey struct{ kind, scope, id string }
type optionConfig struct {
	Options []appfields.Option `json:"options"`
}

// RemapInternalIDs builds a detached structure. Generator failure cannot expose
// a partial manifest or cause a database/filesystem/engine side effect.
func RemapInternalIDs(in Manifest, newID func() (string, error)) (Manifest, error) {
	fail := func(e error) (Manifest, error) { return Manifest{}, e }
	if newID == nil {
		return fail(ErrInvalid)
	}
	m, e := NormalizeManifest(in)
	if e != nil {
		return fail(e)
	}
	refs, e := references(&m)
	if e != nil {
		return fail(e)
	}
	forbidden := map[string]bool{}
	for _, id := range refs.UserIDs {
		forbidden[id] = true
	}
	for _, id := range refs.DepartmentIDs {
		forbidden[id] = true
	}
	keys := []identityKey{}
	seen := map[identityKey]bool{}
	add := func(kind, scope, id string) error {
		k := identityKey{kind, scope, id}
		if !validID(id) || seen[k] {
			return ErrInvalid
		}
		seen[k] = true
		keys = append(keys, k)
		forbidden[id] = true
		return nil
	}
	if e = add("app", "", m.Application.ID); e != nil {
		return fail(e)
	}
	for _, d := range m.Directories {
		if e = add("dir", "", d.ID); e != nil {
			return fail(e)
		}
	}
	kinds := map[string]map[string]string{}
	for _, t := range m.Tables {
		if e = add("table", "", t.ID); e != nil {
			return fail(e)
		}
		kinds[t.ID] = map[string]string{}
		for _, f := range t.Fields {
			if e = add("field", "", f.ID); e != nil {
				return fail(e)
			}
			kinds[t.ID][f.ID] = f.Kind
			if f.Kind == "single_select" || f.Kind == "multi_select" {
				var cfg optionConfig
				if json.Unmarshal(f.Config, &cfg) != nil {
					return fail(ErrInvalid)
				}
				for _, o := range cfg.Options {
					if e = add("option", f.ID, o.ID); e != nil {
						return fail(e)
					}
				}
			}
		}
	}
	var addLayout func(string, []appfields.LayoutNode) error
	addLayout = func(scope string, ns []appfields.LayoutNode) error {
		for _, n := range ns {
			if e := add("layout", scope, n.ID); e != nil {
				return e
			}
			if e := addLayout(scope, n.Children); e != nil {
				return e
			}
		}
		return nil
	}
	for _, f := range m.Forms {
		if e = add("form", "", f.ID); e != nil {
			return fail(e)
		}
		if e = addLayout(f.ID, f.Layout); e != nil {
			return fail(e)
		}
	}
	for _, w := range m.Workflows {
		if e = add("flow", "", w.ID); e != nil {
			return fail(e)
		}
		for _, n := range w.Graph.Nodes {
			if e = add("node", w.ID, n.ID); e != nil {
				return fail(e)
			}
		}
	}
	for _, g := range m.PermissionGroups {
		if e = add("group", "", g.ID); e != nil {
			return fail(e)
		}
	}
	ids := map[identityKey]string{}
	for _, k := range keys {
		id, err := newID()
		if err != nil {
			return fail(err)
		}
		if !validID(id) || forbidden[id] {
			return fail(ErrInvalid)
		}
		forbidden[id] = true
		ids[k] = id
	}
	get := func(kind, scope, id string) string { return ids[identityKey{kind, scope, id}] }
	optionValue := func(field, kind string, raw json.RawMessage) (json.RawMessage, error) {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return raw, nil
		}
		if kind == "single_select" {
			var old string
			if json.Unmarshal(raw, &old) != nil {
				return nil, ErrInvalid
			}
			id := get("option", field, old)
			if id == "" {
				return nil, ErrInvalid
			}
			return json.Marshal(id)
		}
		var old []string
		if json.Unmarshal(raw, &old) != nil {
			return nil, ErrInvalid
		}
		out := []string{}
		for _, v := range old {
			id := get("option", field, v)
			if id == "" {
				return nil, ErrInvalid
			}
			out = append(out, id)
		}
		return json.Marshal(out)
	}
	m.Application.ID = get("app", "", m.Application.ID)
	for i := range m.Directories {
		d := &m.Directories[i]
		d.ID = get("dir", "", d.ID)
		if d.ParentID != nil {
			*d.ParentID = get("dir", "", *d.ParentID)
		}
	}
	for i := range m.Tables {
		t := &m.Tables[i]
		t.ID = get("table", "", t.ID)
		if t.DirectoryID != nil {
			*t.DirectoryID = get("dir", "", *t.DirectoryID)
		}
		for j := range t.Fields {
			f := &t.Fields[j]
			old := f.ID
			f.ID = get("field", "", old)
			if f.Kind == "single_select" || f.Kind == "multi_select" {
				var cfg optionConfig
				if json.Unmarshal(f.Config, &cfg) != nil {
					return fail(ErrInvalid)
				}
				for i := range cfg.Options {
					cfg.Options[i].ID = get("option", old, cfg.Options[i].ID)
				}
				f.Config, _ = json.Marshal(cfg)
				f.Default, e = optionValue(old, f.Kind, f.Default)
				if e != nil {
					return fail(e)
				}
			}
		}
	}
	var layout func(string, []appfields.LayoutNode)
	layout = func(scope string, ns []appfields.LayoutNode) {
		for i := range ns {
			n := &ns[i]
			n.ID = get("layout", scope, n.ID)
			if n.Kind == "field" {
				n.FieldID = get("field", "", n.FieldID)
			}
			layout(scope, n.Children)
		}
	}
	for i := range m.Forms {
		f := &m.Forms[i]
		old := f.ID
		f.ID = get("form", "", old)
		f.TableID = get("table", "", f.TableID)
		if f.DirectoryID != nil {
			*f.DirectoryID = get("dir", "", *f.DirectoryID)
		}
		layout(old, f.Layout)
	}
	for i := range m.Workflows {
		w := &m.Workflows[i]
		flow, table := w.ID, w.TableID
		w.ID = get("flow", "", flow)
		w.TableID = get("table", "", table)
		w.ViewID = get("form", "", w.ViewID)
		for j := range w.Graph.Nodes {
			n := &w.Graph.Nodes[j]
			n.ID = get("node", flow, n.ID)
			if n.Approval != nil {
				for k := range n.Approval.EditableFieldIDs {
					n.Approval.EditableFieldIDs[k] = get("field", "", n.Approval.EditableFieldIDs[k])
				}
			}
			if n.Kind == "condition" {
				n.Condition, e = remapCondition(n.Condition, kinds[table], get, optionValue)
				if e != nil {
					return fail(e)
				}
			}
		}
		for j := range w.Graph.Edges {
			edge := &w.Graph.Edges[j]
			edge.From = get("node", flow, edge.From)
			edge.To = get("node", flow, edge.To)
		}
		for j := range w.Triggers {
			w.Triggers[j].Condition, e = remapCondition(w.Triggers[j].Condition, kinds[table], get, optionValue)
			if e != nil {
				return fail(e)
			}
		}
	}
	for i := range m.PermissionGroups {
		g := &m.PermissionGroups[i]
		g.ID = get("group", "", g.ID)
		for j := range g.Grants {
			grant := &g.Grants[j]
			kind := map[string]string{"application": "app", "directory": "dir", "form": "form"}[grant.ResourceKind]
			grant.ResourceID = get(kind, "", grant.ResourceID)
			for k := range grant.Fields {
				grant.Fields[k] = get("field", "", grant.Fields[k])
			}
		}
	}
	return NormalizeManifest(m)
}
func remapCondition(raw json.RawMessage, kinds map[string]string, get func(string, string, string) string, optionValue func(string, string, json.RawMessage) (json.RawMessage, error)) (json.RawMessage, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw, nil
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return nil, ErrInvalid
	}
	var walk func(map[string]json.RawMessage) error
	walk = func(n map[string]json.RawMessage) error {
		if b, ok := n["children"]; ok {
			var children []map[string]json.RawMessage
			if json.Unmarshal(b, &children) != nil {
				return ErrInvalid
			}
			for _, c := range children {
				if e := walk(c); e != nil {
					return e
				}
			}
			n["children"], _ = json.Marshal(children)
			return nil
		}
		var old string
		if json.Unmarshal(n["fieldId"], &old) != nil {
			return ErrInvalid
		}
		if old == "createdAt" || old == "updatedAt" {
			return nil
		}
		id := get("field", "", old)
		if id == "" {
			return ErrInvalid
		}
		n["fieldId"], _ = json.Marshal(id)
		if kind := kinds[old]; kind == "single_select" || kind == "multi_select" {
			value, e := optionValue(old, kind, n["value"])
			if e != nil {
				return e
			}
			n["value"] = value
		}
		return nil
	}
	if e := walk(root); e != nil {
		return nil, e
	}
	return json.Marshal(root)
}
