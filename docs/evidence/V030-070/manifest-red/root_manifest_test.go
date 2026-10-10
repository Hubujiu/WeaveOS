package apptemplates

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"reflect"
	"strings"
	"testing"
)

func mid(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func rootManifest() Manifest {
	dir := mid(2)
	return Manifest{Format: "weaveos.structure-template", Version: 1, Application: Application{mid(1), " Expenses "}, Directories: []Directory{{mid(2), " Teams ", nil, 0}}, Tables: []Table{{mid(3), "Claims", &dir, 0, []appfields.Field{{ID: mid(4), Name: "Reason", Kind: "text", Default: json.RawMessage("null"), Config: json.RawMessage(`{"maxLength":null}`)}}}}, Forms: []Form{{mid(5), mid(3), "Claims", &dir, 0, []appfields.LayoutNode{{ID: mid(6), Kind: "field", FieldID: mid(4), Span: 12}}}}, Workflows: []Workflow{{ID: mid(7), TableID: mid(3), ViewID: mid(5), Name: "Approval", AllowWithdraw: true, Graph: Graph{Version: 1, Nodes: []Node{{ID: mid(8), Kind: "start"}, {ID: mid(9), Kind: "approval", Approval: &Approval{Mode: "all", AssigneeIDs: []string{mid(12)}, EditableFieldIDs: []string{mid(4)}}}, {ID: mid(10), Kind: "end"}}, Edges: []Edge{{From: mid(8), To: mid(9)}, {From: mid(9), To: mid(10)}}}, Triggers: []workflowcatalog.Trigger{{Event: "record.created", Condition: json.RawMessage("null")}}}}, PermissionGroups: []PermissionGroup{{ID: mid(11), Name: "Finance", Enabled: true, MemberIDs: []string{mid(12)}, Grants: []Grant{{ResourceKind: "form", ResourceID: mid(5), Action: "menu.enter", RowScope: "all", Fields: []string{}}, {ResourceKind: "form", ResourceID: mid(5), Action: "data.edit", RowScope: "all", Fields: []string{mid(4)}}}}}}
}
func mbytes(t *testing.T, m Manifest) []byte {
	t.Helper()
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRootManifestCompleteStructureRoundTripAndInputUnchanged(t *testing.T) {
	in := rootManifest()
	before := mbytes(t, in)
	got, e := NormalizeManifest(in)
	if e != nil {
		t.Fatal("complete structural template rejected", e)
	}
	if got.Application.Name != "Expenses" || got.Directories[0].Name != "Teams" || len(got.Tables[0].Fields) != 1 || len(got.Workflows) != 1 || len(got.PermissionGroups[0].Grants) != 2 {
		t.Fatal("supported configuration lost")
	}
	if string(mbytes(t, in)) != string(before) {
		t.Fatal("normalization mutated caller")
	}
	back, e := DecodeManifest(mbytes(t, got))
	if e != nil || !reflect.DeepEqual(got, back) {
		t.Fatal("complete template roundtrip changed", e)
	}
	got.Workflows[0].Graph.Nodes[1].Approval.AssigneeIDs[0] = mid(99)
	got.Tables[0].Fields[0].Config[0] = 'x'
	if string(mbytes(t, in)) != string(before) {
		t.Fatal("normalization returned aliases")
	}
}
func TestRootManifestMultipleViewsAndEmptyApplication(t *testing.T) {
	in := rootManifest()
	in.Forms = append(in.Forms, Form{ID: mid(21), TableID: mid(3), Name: "Second", Layout: []appfields.LayoutNode{}})
	if _, e := NormalizeManifest(in); e != nil {
		t.Fatal("shared physical table views rejected", e)
	}
	empty := Manifest{Format: "weaveos.structure-template", Version: 1, Application: Application{mid(1), "Empty"}, Directories: []Directory{}, Tables: []Table{}, Forms: []Form{}, Workflows: []Workflow{}, PermissionGroups: []PermissionGroup{}}
	if _, e := DecodeManifest(mbytes(t, empty)); e != nil {
		t.Fatal("valid empty application rejected", e)
	}
}
func TestRootManifestRejectsInvalidBindingsAndGraph(t *testing.T) {
	cases := map[string]func(*Manifest){
		"cycle":               func(m *Manifest) { m.Directories[0].ParentID = &m.Directories[0].ID },
		"missing-parent":      func(m *Manifest) { id := mid(80); m.Directories[0].ParentID = &id },
		"duplicate-table":     func(m *Manifest) { m.Tables = append(m.Tables, m.Tables[0]) },
		"foreign-table":       func(m *Manifest) { m.Forms[0].TableID = mid(80) },
		"foreign-grant-field": func(m *Manifest) { m.PermissionGroups[0].Grants[1].Fields = []string{mid(80)} },
		"foreign-flow-view":   func(m *Manifest) { m.Workflows[0].ViewID = mid(80) },
		"foreign-node-field":  func(m *Manifest) { m.Workflows[0].Graph.Nodes[1].Approval.EditableFieldIDs = []string{mid(80)} },
		"script":              func(m *Manifest) { m.Workflows[0].Graph.Nodes[1].Kind = "script" },
		"unknown-trigger":     func(m *Manifest) { m.Workflows[0].Triggers[0].Event = "webhook" },
		"global-permission":   func(m *Manifest) { m.PermissionGroups[0].Grants[1].Action = "personnel.manage" },
		"invalid-field":       func(m *Manifest) { m.Tables[0].Fields[0].Kind = "password" },
		"zero-identity":       func(m *Manifest) { m.Application.ID = "00000000-0000-0000-0000-000000000000" },
		"null-collection":     func(m *Manifest) { m.Forms = nil },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			m := rootManifest()
			edit(&m)
			if _, e := NormalizeManifest(m); !errors.Is(e, ErrInvalid) {
				t.Fatal("invalid template accepted", e)
			}
		})
	}
}
func TestRootManifestRejectsUntrustedJSONAndCapacity(t *testing.T) {
	raw := string(mbytes(t, rootManifest()))
	cases := map[string][]byte{"null": []byte("null"), "array": []byte("[]"), "trailing": []byte(raw + "{}"), "duplicate": []byte(strings.Replace(raw, `"version":1`, `"version":1,"version":1`, 1)), "case-key": []byte(strings.Replace(raw, `"format":`, `"Format":`, 1)), "records": []byte(strings.TrimSuffix(raw, "}") + `,"records":[{"value":"secret"}]}`), "owner": []byte(strings.Replace(raw, `"application":{`, `"application":{"ownerUserId":"`+mid(99)+`",`, 1)), "too-large": []byte(strings.Repeat(" ", 1048577) + raw), "utf8": append([]byte{255}, []byte(raw)...), "missing-array": []byte(strings.Replace(raw, `"directories":`, `"unknownDirectories":`, 1))}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := DecodeManifest(b); !errors.Is(e, ErrInvalid) {
				t.Fatal("unsafe JSON accepted", e)
			}
		})
	}
}
