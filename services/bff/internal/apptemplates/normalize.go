package apptemplates

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"strings"
	"unicode/utf8"
)

func validID(s string) bool {
	return appfields.ValidID(s) && s != "00000000-0000-0000-0000-000000000000"
}
func name(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, utf8.ValidString(s) && s != "" && !strings.ContainsRune(s, 0) && utf8.RuneCountInString(s) <= 100
}
func unique(ids map[string]bool, id string) bool {
	if !validID(id) || ids[id] {
		return false
	}
	ids[id] = true
	return true
}
func position(p int) bool { return p >= 0 && int64(p) <= 2147483647 }

// Before JSON cloning, reject unused struct members that a custom layout
// marshaler would otherwise discard. The wire validator performs the same gate.
func layoutMembers(ns []appfields.LayoutNode, depth int) bool {
	if depth > 32 || ns == nil {
		return false
	}
	for _, n := range ns {
		if !validID(n.ID) {
			return false
		}
		switch n.Kind {
		case "field", "system_field":
			if n.Title != "" || n.Text != "" || n.Children != nil {
				return false
			}
		case "group":
			if n.FieldID != "" || n.Text != "" || !layoutMembers(n.Children, depth+1) {
				return false
			}
		case "description":
			if n.Title != "" || n.FieldID != "" || n.Span != 0 || n.Children != nil {
				return false
			}
		case "divider":
			if n.Title != "" || n.Text != "" || n.FieldID != "" || n.Span != 0 || n.Children != nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}
func NormalizeManifest(in Manifest) (Manifest, error) {
	bad := func() (Manifest, error) { return Manifest{}, ErrInvalid }
	if in.Format != "weaveos.structure-template" || in.Version != 1 || !validID(in.Application.ID) || in.Directories == nil || in.Tables == nil || in.Forms == nil || in.Workflows == nil || in.PermissionGroups == nil || len(in.Directories) > 1000 || len(in.Tables) > 128 || len(in.Forms) > 256 || len(in.Workflows) > 128 || len(in.PermissionGroups) > 128 {
		return bad()
	}
	for _, f := range in.Forms {
		if !layoutMembers(f.Layout, 0) {
			return bad()
		}
	}
	raw, e := json.Marshal(in)
	if e != nil || len(raw) > 1048576 {
		return bad()
	}
	var out Manifest
	if json.Unmarshal(raw, &out) != nil {
		return bad()
	}
	var ok bool
	out.Application.Name, ok = name(out.Application.Name)
	if !ok {
		return bad()
	}
	dirs := map[string]bool{}
	parents := map[string]*string{}
	for i := range out.Directories {
		d := &out.Directories[i]
		d.Name, ok = name(d.Name)
		if !ok || !position(d.Position) || !unique(dirs, d.ID) {
			return bad()
		}
		parents[d.ID] = d.ParentID
	}
	for id := range dirs {
		seen := map[string]bool{}
		for cur := id; cur != ""; {
			if seen[cur] || !dirs[cur] {
				return bad()
			}
			seen[cur] = true
			p := parents[cur]
			if p == nil {
				break
			}
			cur = *p
			if !validID(cur) {
				return bad()
			}
		}
	}
	hasDir := func(id *string) bool { return id == nil || dirs[*id] }
	tables := map[string]bool{}
	fields := map[string][]appfields.Field{}
	fieldOwner := map[string]string{}
	queryFields := map[string][]appquery.Field{}
	for i := range out.Tables {
		t := &out.Tables[i]
		t.Name, ok = name(t.Name)
		if !ok || !position(t.Position) || !hasDir(t.DirectoryID) || !unique(tables, t.ID) || t.Fields == nil || len(t.Fields) > 200 {
			return bad()
		}
		t.Fields, e = appfields.NormalizeFields(t.Fields)
		if e != nil {
			return bad()
		}
		fields[t.ID] = t.Fields
		queryFields[t.ID] = []appquery.Field{}
		for _, f := range t.Fields {
			if !validID(f.ID) || fieldOwner[f.ID] != "" {
				return bad()
			}
			fieldOwner[f.ID] = t.ID
			queryFields[t.ID] = append(queryFields[t.ID], appquery.Field{ID: f.ID, Kind: appquery.FieldKind(f.Kind)})
		}
	}
	forms := map[string]bool{}
	formTables := map[string]string{}
	for i := range out.Forms {
		f := &out.Forms[i]
		f.Name, ok = name(f.Name)
		if !ok || !position(f.Position) || !hasDir(f.DirectoryID) || !unique(forms, f.ID) || !tables[f.TableID] {
			return bad()
		}
		f.Layout, e = appfields.NormalizeLayout(f.Layout, fields[f.TableID])
		if e != nil {
			return bad()
		}
		formTables[f.ID] = f.TableID
	}
	flows := map[string]bool{}
	for i := range out.Workflows {
		w := &out.Workflows[i]
		w.Name, ok = name(w.Name)
		if !ok || !unique(flows, w.ID) || !tables[w.TableID] || formTables[w.ViewID] != w.TableID || w.Triggers == nil {
			return bad()
		}
		g := flowgraph.Graph{Version: w.Graph.Version, Nodes: []flowgraph.Node{}, Edges: []flowgraph.Edge{}}
		for _, n := range w.Graph.Nodes {
			node := flowgraph.Node{ID: n.ID, Kind: n.Kind, Condition: n.Condition}
			if n.Approval != nil {
				node.Approval = &flowgraph.Approval{Mode: n.Approval.Mode, AssigneeIDs: n.Approval.AssigneeIDs, EditableFieldIDs: n.Approval.EditableFieldIDs}
			}
			g.Nodes = append(g.Nodes, node)
		}
		for _, edge := range w.Graph.Edges {
			g.Edges = append(g.Edges, flowgraph.Edge{From: edge.From, To: edge.To, Branch: edge.Branch})
		}
		if _, e = flowgraph.Validate(g, queryFields[w.TableID]); e != nil {
			return bad()
		}
		w.Triggers, e = workflowcatalog.NormalizeTriggers(w.Triggers, queryFields[w.TableID])
		if e != nil {
			return bad()
		}
	}
	groups := map[string]bool{}
	for i := range out.PermissionGroups {
		g := &out.PermissionGroups[i]
		g.Name, ok = name(g.Name)
		if !ok || !unique(groups, g.ID) || g.MemberIDs == nil || g.Grants == nil || len(g.MemberIDs) > 10000 || len(g.Grants) > 10000 {
			return bad()
		}
		members := map[string]bool{}
		for _, id := range g.MemberIDs {
			if !unique(members, id) {
				return bad()
			}
		}
		for _, grant := range g.Grants {
			if grant.Fields == nil {
				return bad()
			}
			switch grant.ResourceKind {
			case "application":
				if grant.ResourceID != out.Application.ID {
					return bad()
				}
			case "directory":
				if !dirs[grant.ResourceID] {
					return bad()
				}
			case "form":
				if !forms[grant.ResourceID] {
					return bad()
				}
			default:
				return bad()
			}
			if grant.Action == "menu.enter" {
				if grant.RowScope != "all" || len(grant.Fields) != 0 {
					return bad()
				}
				continue
			}
			if grant.ResourceKind != "form" || grant.RowScope != "all" && grant.RowScope != "own" {
				return bad()
			}
			switch grant.Action {
			case "data.create":
				if grant.RowScope != "all" {
					return bad()
				}
			case "data.read", "data.edit", "data.history":
			default:
				return bad()
			}
			ids := map[string]bool{}
			for _, id := range grant.Fields {
				if !unique(ids, id) || fieldOwner[id] != formTables[grant.ResourceID] {
					return bad()
				}
			}
		}
	}
	return out, nil
}
