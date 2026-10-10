package apptemplates

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"strings"
	"testing"
)

func rootRemapManifest() Manifest {
	m := rootBindingManifest()
	options := json.RawMessage(`{"options":[{"id":"` + mid(61) + `","label":"First"},{"id":"` + mid(62) + `","label":"Second"}]}`)
	m.Tables[0].Fields = append(m.Tables[0].Fields,
		appfields.Field{ID: mid(60), Name: "Single", Kind: "single_select", Default: json.RawMessage(`"` + mid(61) + `"`), Config: options},
		appfields.Field{ID: mid(63), Name: "Multi", Kind: "multi_select", Default: json.RawMessage(`["` + mid(61) + `","` + mid(62) + `"]`), Config: options},
	)
	parent := m.Directories[0].ID
	m.Directories = append(m.Directories, Directory{ID: mid(70), Name: "Child", ParentID: &parent})
	m.Forms[0].DirectoryID = &m.Directories[1].ID
	m.Forms[0].Layout = []appfields.LayoutNode{{ID: mid(71), Kind: "group", Title: "Group", Span: 12, Children: []appfields.LayoutNode{{ID: mid(72), Kind: "field", FieldID: mid(60), Span: 12}, {ID: mid(73), Kind: "system_field", FieldID: "createdAt", Span: 12}}}}
	m.Forms = append(m.Forms, Form{ID: mid(74), TableID: mid(3), Name: "Second", Layout: m.Forms[0].Layout})
	m.Workflows[0].Triggers[0].Condition = json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + mid(60) + `","operator":"eq","value":"` + mid(61) + `"},{"operator":"or","children":[{"fieldId":"` + mid(63) + `","operator":"neq","value":["` + mid(62) + `"]},{"fieldId":"createdAt","operator":"gt","value":"2026-01-01T00:00:00Z"}]}]}`)
	m.PermissionGroups[0].Grants = append(m.PermissionGroups[0].Grants, Grant{ResourceKind: "application", ResourceID: mid(1), Action: "menu.enter", RowScope: "all", Fields: []string{}}, Grant{ResourceKind: "directory", ResourceID: mid(70), Action: "menu.enter", RowScope: "all", Fields: []string{}})
	return m
}
func rootNewIDs() func() (string, error) {
	n := 1000
	return func() (string, error) { n++; return mid(n), nil }
}
func TestRootTemplateRemapCompleteInternalRelationships(t *testing.T) {
	in := rootRemapManifest()
	before := mbytes(t, in)
	if _, e := NormalizeManifest(in); e != nil {
		t.Fatal("independent complete fixture invalid", e)
	}
	got, e := RemapInternalIDs(in, rootNewIDs())
	if e != nil {
		t.Fatal("complete identity rebuild unavailable", e)
	}
	if got.Application.ID == in.Application.ID || got.Tables[0].ID == in.Tables[0].ID || got.Forms[0].ID == in.Forms[0].ID || got.Workflows[0].ID == in.Workflows[0].ID || got.PermissionGroups[0].ID == in.PermissionGroups[0].ID {
		t.Fatal("source structural identity reused")
	}
	if *got.Directories[1].ParentID != got.Directories[0].ID || *got.Forms[0].DirectoryID != got.Directories[1].ID || *got.Tables[0].DirectoryID != got.Directories[0].ID {
		t.Fatal("directory mapping broken")
	}
	table := got.Tables[0]
	form := got.Forms[0]
	flow := got.Workflows[0]
	if form.TableID != table.ID || got.Forms[1].TableID != table.ID || flow.TableID != table.ID || flow.ViewID != form.ID {
		t.Fatal("shared table or flow binding broken")
	}
	if form.Layout[0].ID == in.Forms[0].Layout[0].ID || form.Layout[0].ID == got.Forms[1].Layout[0].ID || form.Layout[0].Children[0].FieldID != table.Fields[3].ID || form.Layout[0].Children[1].FieldID != "createdAt" {
		t.Fatal("scoped nested layout or system field broken")
	}
	if flow.Graph.Edges[0].From != flow.Graph.Nodes[0].ID || flow.Graph.Edges[0].To != flow.Graph.Nodes[3].ID || flow.Graph.Nodes[1].Approval.EditableFieldIDs[0] != table.Fields[0].ID {
		t.Fatal("graph topology or editable field mapping broken")
	}
	if !strings.Contains(string(flow.Graph.Nodes[3].Condition), table.Fields[1].ID) || !strings.Contains(string(flow.Graph.Nodes[3].Condition), mid(43)) {
		t.Fatal("routing condition field or external identity broken")
	}
	grants := got.PermissionGroups[0].Grants
	if grants[0].ResourceID != form.ID || grants[1].Fields[0] != table.Fields[0].ID || grants[2].ResourceID != got.Application.ID || grants[3].ResourceID != got.Directories[1].ID {
		t.Fatal("permission binding broken")
	}
	var single, multi struct {
		Options []appfields.Option `json:"options"`
	}
	if json.Unmarshal(table.Fields[3].Config, &single) != nil || json.Unmarshal(table.Fields[4].Config, &multi) != nil {
		t.Fatal("invalid option config")
	}
	if single.Options[0].ID == mid(61) || single.Options[0].ID == multi.Options[0].ID || string(table.Fields[3].Default) != `"`+single.Options[0].ID+`"` {
		t.Fatal("field-scoped option identity lost")
	}
	var defaults []string
	if json.Unmarshal(table.Fields[4].Default, &defaults) != nil || len(defaults) != 2 || defaults[0] != multi.Options[0].ID || defaults[1] != multi.Options[1].ID {
		t.Fatal("multi default lost")
	}
	condition := string(flow.Triggers[0].Condition)
	for _, s := range []string{table.Fields[3].ID, table.Fields[4].ID, single.Options[0].ID, multi.Options[1].ID, "createdAt", "2026-01-01T00:00:00Z"} {
		if !strings.Contains(condition, s) {
			t.Fatal("condition remap lost", s)
		}
	}
	if got.PermissionGroups[0].MemberIDs[0] != mid(12) || flow.Graph.Nodes[1].Approval.AssigneeIDs[0] != mid(12) || string(table.Fields[1].Default) != `"`+mid(40)+`"` || string(table.Fields[0].Default) != `"`+mid(40)+`"` {
		t.Fatal("external identities or text changed")
	}
	if string(mbytes(t, in)) != string(before) {
		t.Fatal("identity mapping mutated input")
	}
	if _, e = DecodeManifest(mbytes(t, got)); e != nil {
		t.Fatal("rebuilt manifest is not valid", e)
	}
}
func TestRootTemplateRemapSeparatesOverlappingNamespaces(t *testing.T) {
	m := rootManifest()
	m.Tables[0].ID = m.Application.ID
	m.Forms[0].TableID = m.Application.ID
	m.Workflows[0].TableID = m.Application.ID
	got, e := RemapInternalIDs(m, rootNewIDs())
	if e != nil || got.Application.ID == got.Tables[0].ID || got.Forms[0].TableID != got.Tables[0].ID {
		t.Fatal("app/table namespaces conflated", e)
	}
}
func TestRootTemplateRemapRejectsGeneratorFailureAndCollisions(t *testing.T) {
	for _, mode := range []string{"error", "repeated", "source", "external", "zero", "invalid", "nil", "dangling-option"} {
		t.Run(mode, func(t *testing.T) {
			m := rootRemapManifest()
			before := mbytes(t, m)
			gen := rootNewIDs()
			switch mode {
			case "error":
				gen = func() (string, error) { return "", errors.New("rng unavailable") }
			case "repeated":
				gen = func() (string, error) { return mid(999), nil }
			case "source":
				gen = func() (string, error) { return mid(61), nil }
			case "external":
				gen = func() (string, error) { return mid(43), nil }
			case "zero":
				gen = func() (string, error) { return "00000000-0000-0000-0000-000000000000", nil }
			case "invalid":
				gen = func() (string, error) { return "bad", nil }
			case "nil":
				gen = nil
			case "dangling-option":
				m.Workflows[0].Triggers[0].Condition = json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + mid(60) + `","operator":"eq","value":"` + mid(888) + `"}]}`)
				before = mbytes(t, m)
			}
			got, e := RemapInternalIDs(m, gen)
			if e == nil || got.Application.ID != "" {
				t.Fatal("unsafe or partial structure returned", e)
			}
			if string(mbytes(t, m)) != string(before) {
				t.Fatal("failure mutated input")
			}
		})
	}
}
