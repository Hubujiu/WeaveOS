package apptemplates

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRootCopyGroupMappingExplicitDetached(t *testing.T) {
	in := copyFixture(t)
	before := string(mbytes(t, in))
	got, err := MapCopyGroups(in, []CopyGroupMapping{{mid(50), "  财务副本  "}, {mid(11), "Independent"}})
	if err != nil {
		t.Fatal("complete explicit mapping rejected", err)
	}
	if got.PermissionGroups[0].ID != mid(11) || got.PermissionGroups[0].Name != "Independent" || got.PermissionGroups[1].Name != "财务副本" || got.PermissionGroups[1].Enabled {
		t.Fatal("identity/order/name/enabled changed incorrectly", got.PermissionGroups)
	}
	for i := range in.PermissionGroups {
		if !reflect.DeepEqual(in.PermissionGroups[i].Grants, got.PermissionGroups[i].Grants) || !reflect.DeepEqual(in.PermissionGroups[i].MemberIDs, got.PermissionGroups[i].MemberIDs) {
			t.Fatal("mapping changed authority")
		}
	}
	got.PermissionGroups[0].MemberIDs[0] = mid(99)
	got.PermissionGroups[0].Grants[0].ResourceID = mid(99)
	if string(mbytes(t, in)) != before {
		t.Fatal("mapping mutated source")
	}
}
func TestRootCopyGroupNamesDoNotMergeIdentities(t *testing.T) {
	got, err := MapCopyGroups(copyFixture(t), []CopyGroupMapping{{mid(11), "Same"}, {mid(50), "Same"}})
	if err != nil || len(got.PermissionGroups) != 2 || got.PermissionGroups[0].ID == got.PermissionGroups[1].ID {
		t.Fatal("same names merged or rejected", err)
	}
}
func TestRootCopyGroupMappingEmptySelection(t *testing.T) {
	in := copyFixture(t)
	in.PermissionGroups = []PermissionGroup{}
	got, err := MapCopyGroups(in, []CopyGroupMapping{})
	if err != nil || len(got.PermissionGroups) != 0 {
		t.Fatal("no-group copy rejected", err)
	}
}
func TestRootCopyGroupMappingClosedAndValidated(t *testing.T) {
	cases := map[string][]CopyGroupMapping{
		"missing":   {{mid(11), "New"}},
		"duplicate": {{mid(11), "New"}, {mid(11), "Other"}},
		"unknown":   {{mid(11), "New"}, {mid(99), "Other"}},
		"zero":      {{mid(11), "New"}, {"00000000-0000-0000-0000-000000000000", "Other"}},
		"emptyName": {{mid(11), "New"}, {mid(50), " \t "}},
		"longName":  {{mid(11), "New"}, {mid(50), strings.Repeat("文", 101)}},
		"nul":       {{mid(11), "New"}, {mid(50), "a\x00b"}},
		"utf8":      {{mid(11), "New"}, {mid(50), string([]byte{255})}},
		"extra":     {{mid(11), "New"}, {mid(50), "Other"}, {mid(99), "Extra"}},
	}
	for label, mappings := range cases {
		t.Run(label, func(t *testing.T) {
			if _, err := MapCopyGroups(copyFixture(t), mappings); !errors.Is(err, ErrInvalid) {
				t.Fatal("invalid mapping accepted", err)
			}
		})
	}
	in := copyFixture(t)
	in.Forms[0].TableID = mid(99)
	if _, err := MapCopyGroups(in, []CopyGroupMapping{{mid(11), "New"}, {mid(50), "Other"}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid source accepted", err)
	}
}
func TestRootCopyGroupMappingUnicodeBoundary(t *testing.T) {
	got, err := MapCopyGroups(copyFixture(t), []CopyGroupMapping{{mid(11), strings.Repeat("文", 100)}, {mid(50), "Other"}})
	if err != nil || got.PermissionGroups[0].Name != strings.Repeat("文", 100) {
		t.Fatal("100 code points rejected", err)
	}
}
