package apptemplates

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
)

// Independent approved-scope fixture: root 2, child 20, unrelated 30;
// table 3 is outside selected root, table 40 belongs inside; views 5/21
// share 3 inside root, view 31 shares it outside and must not be pulled in.
func copyFixture(t *testing.T) Manifest {
	t.Helper()
	m := rootManifest()
	root, child, other := mid(2), mid(20), mid(30)
	m.Directories = append(m.Directories, Directory{child, "Child", &root, 1}, Directory{other, "Other", nil, 2}, Directory{mid(60), "Empty", nil, 3})
	m.Tables[0].DirectoryID = &other
	m.Tables = append(m.Tables, Table{mid(40), "Inside empty table", &child, 1, []appfields.Field{}})
	m.Forms = append(m.Forms, Form{mid(21), mid(3), "Inside second", &child, 1, []appfields.LayoutNode{}}, Form{mid(31), mid(3), "Outside view", &other, 2, []appfields.LayoutNode{}})
	flow := m.Workflows[0]
	flow.ID = mid(32)
	flow.ViewID = mid(31)
	flow.Name = "Outside flow"
	m.Workflows = append(m.Workflows, flow)
	m.PermissionGroups[0].Grants = append(m.PermissionGroups[0].Grants, Grant{"application", mid(1), "menu.enter", "all", []string{}}, Grant{"directory", root, "menu.enter", "all", []string{}}, Grant{"directory", child, "menu.enter", "all", []string{}}, Grant{"form", mid(31), "menu.enter", "all", []string{}})
	m.PermissionGroups = append(m.PermissionGroups, PermissionGroup{mid(50), "Unrelated", false, []string{mid(12)}, []Grant{{"form", mid(31), "data.read", "own", []string{mid(4)}}}})
	if _, err := NormalizeManifest(m); err != nil {
		t.Fatal("independent fixture invalid", err)
	}
	return m
}
func assertCopyIDs(t *testing.T, m Manifest, dirs, tables, forms, flows, groups []string) {
	t.Helper()
	got := [][]string{{}, {}, {}, {}, {}}
	for _, v := range m.Directories {
		got[0] = append(got[0], v.ID)
	}
	for _, v := range m.Tables {
		got[1] = append(got[1], v.ID)
	}
	for _, v := range m.Forms {
		got[2] = append(got[2], v.ID)
	}
	for _, v := range m.Workflows {
		got[3] = append(got[3], v.ID)
	}
	for _, v := range m.PermissionGroups {
		got[4] = append(got[4], v.ID)
	}
	if !reflect.DeepEqual(got, [][]string{dirs, tables, forms, flows, groups}) {
		t.Fatalf("wrong selected identities: %#v", got)
	}
	if _, err := NormalizeManifest(m); err != nil {
		t.Fatal("selection is not closed", err)
	}
}
func TestRootCopySelectDirectoryClosure(t *testing.T) {
	in := copyFixture(t)
	before := string(mbytes(t, in))
	got, e := SelectCopyManifest(in, "directory", mid(2))
	if e != nil {
		t.Fatal("valid directory selection rejected", e)
	}
	assertCopyIDs(t, got, []string{mid(2), mid(20)}, []string{mid(3), mid(40)}, []string{mid(5), mid(21)}, []string{mid(7)}, []string{mid(11)})
	if got.Directories[0].ParentID != nil || *got.Directories[1].ParentID != mid(2) || *got.Tables[0].DirectoryID != mid(2) || *got.Tables[1].DirectoryID != mid(20) {
		t.Fatal("root/required external table placement wrong")
	}
	if len(got.PermissionGroups[0].Grants) != 4 {
		t.Fatal("ancestor or unrelated grants leaked", got.PermissionGroups)
	}
	if string(mbytes(t, in)) != before {
		t.Fatal("selection mutated source")
	}
}
func TestRootCopySelectTableIncludesAllViewsNotParents(t *testing.T) {
	got, e := SelectCopyManifest(copyFixture(t), "table", mid(3))
	if e != nil {
		t.Fatal(e)
	}
	assertCopyIDs(t, got, []string{}, []string{mid(3)}, []string{mid(5), mid(21), mid(31)}, []string{mid(7), mid(32)}, []string{mid(11), mid(50)})
	if got.Tables[0].DirectoryID != nil {
		t.Fatal("copied parent directory unexpectedly")
	}
	for _, f := range got.Forms {
		if f.DirectoryID != nil {
			t.Fatal("source directory leaked")
		}
	}
	if len(got.PermissionGroups[0].Grants) != 3 {
		t.Fatal("table-scope grants wrong")
	}
}
func TestRootCopySelectFormHasIndependentCompleteTableOnly(t *testing.T) {
	got, e := SelectCopyManifest(copyFixture(t), "form", mid(5))
	if e != nil {
		t.Fatal(e)
	}
	assertCopyIDs(t, got, []string{}, []string{mid(3)}, []string{mid(5)}, []string{mid(7)}, []string{mid(11)})
	if got.Tables[0].DirectoryID != nil || got.Forms[0].DirectoryID != nil || len(got.Tables[0].Fields) != 1 || got.Tables[0].Fields[0].ID != mid(4) || len(got.PermissionGroups[0].Grants) != 2 {
		t.Fatal("single view configuration incomplete or widened")
	}
}
func TestRootCopySelectApplicationPreservesWholeDetachedConfiguration(t *testing.T) {
	in := copyFixture(t)
	before := string(mbytes(t, in))
	got, e := SelectCopyManifest(in, "application", mid(1))
	if e != nil {
		t.Fatal(e)
	}
	assertCopyIDs(t, got, []string{mid(2), mid(20), mid(30), mid(60)}, []string{mid(3), mid(40)}, []string{mid(5), mid(21), mid(31)}, []string{mid(7), mid(32)}, []string{mid(11), mid(50)})
	if got.Application.Name != "Expenses" || len(got.PermissionGroups[0].Grants) != 6 {
		t.Fatal("full application lost configuration")
	}
	got.Tables[0].Fields[0].Config = json.RawMessage(`{"maxLength":42}`)
	got.Workflows[0].Graph.Nodes[1].Approval.AssigneeIDs[0] = mid(99)
	got.PermissionGroups[0].Grants[1].Fields[0] = mid(99)
	if string(mbytes(t, in)) != before {
		t.Fatal("returned aliases to source")
	}
}
func TestRootCopySelectEmptyAndNestedDirectory(t *testing.T) {
	for _, id := range []string{mid(60), mid(20)} {
		t.Run(id, func(t *testing.T) {
			got, e := SelectCopyManifest(copyFixture(t), "directory", id)
			if e != nil {
				t.Fatal(e)
			}
			if len(got.Directories) != 1 || got.Directories[0].ParentID != nil {
				t.Fatal("selected root retains external parent")
			}
			if id == mid(60) {
				assertCopyIDs(t, got, []string{id}, []string{}, []string{}, []string{}, []string{})
			} else {
				assertCopyIDs(t, got, []string{id}, []string{mid(3), mid(40)}, []string{mid(21)}, []string{}, []string{mid(11)})
				if *got.Tables[0].DirectoryID != mid(20) || len(got.PermissionGroups[0].Grants) != 1 {
					t.Fatal("nested closure widened")
				}
			}
		})
	}
}
func TestRootCopySelectRejectsMissingOrUnsupportedScope(t *testing.T) {
	for _, x := range [][2]string{{"application", mid(2)}, {"directory", mid(3)}, {"form", mid(3)}, {"table", mid(5)}, {"all", mid(1)}, {"form", "00000000-0000-0000-0000-000000000000"}, {"form", "not-a-uuid"}} {
		t.Run(x[0]+x[1], func(t *testing.T) {
			got, e := SelectCopyManifest(copyFixture(t), x[0], x[1])
			if !errors.Is(e, ErrInvalid) || got.Application.ID != "" {
				t.Fatal("invalid scope accepted or partial output", e)
			}
		})
	}
	m := copyFixture(t)
	m.Forms[0].TableID = mid(99)
	if _, e := SelectCopyManifest(m, "directory", mid(60)); !errors.Is(e, ErrInvalid) {
		t.Fatal("malformed source silently truncated", e)
	}
}
func TestRootCopySelectNamespacedIDsDoNotCrossSelect(t *testing.T) {
	m := copyFixture(t)
	m.Forms[1].ID = mid(40)
	got, e := SelectCopyManifest(m, "form", mid(40))
	if e != nil {
		t.Fatal(e)
	}
	assertCopyIDs(t, got, []string{}, []string{mid(3)}, []string{mid(40)}, []string{}, []string{})
}
