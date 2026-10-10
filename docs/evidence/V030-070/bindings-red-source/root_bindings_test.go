package apptemplates

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"reflect"
	"strings"
	"testing"
)

func rootBindingManifest() Manifest {
	m := rootManifest()
	m.Tables[0].Fields = append(m.Tables[0].Fields,
		appfields.Field{ID: mid(30), Name: "Member", Kind: "member", Default: json.RawMessage(`"` + mid(40) + `"`), Config: json.RawMessage(`{}`)},
		appfields.Field{ID: mid(31), Name: "Department", Kind: "department", Default: json.RawMessage(`"` + mid(12) + `"`), Config: json.RawMessage(`{}`)},
	)
	m.Tables[0].Fields[0].Default = json.RawMessage(`"` + mid(40) + `"`)
	m.Workflows[0].Triggers[0].Condition = json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + mid(30) + `","operator":"eq","value":"` + mid(41) + `"},{"operator":"or","children":[{"fieldId":"` + mid(31) + `","operator":"neq","value":"` + mid(42) + `"},{"fieldId":"` + mid(4) + `","operator":"eq","value":"` + mid(99) + `"}]}]}`)
	// A distinct condition-only user appears in the routing node, not defaults,
	// assignees, permission members, or trigger expressions.
	m.Workflows[0].Graph.Nodes = append(m.Workflows[0].Graph.Nodes, Node{ID: mid(32), Kind: "condition", Condition: json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + mid(30) + `","operator":"eq","value":"` + mid(43) + `"}]}`)})
	m.Workflows[0].Graph.Edges = []Edge{{From: mid(8), To: mid(32)}, {From: mid(32), To: mid(9), Branch: "true"}, {From: mid(32), To: mid(10), Branch: "false"}, {From: mid(9), To: mid(10)}}
	return m
}
func rootBindings() []Binding {
	return []Binding{{"user", mid(12), mid(112)}, {"user", mid(40), mid(140)}, {"user", mid(41), mid(141)}, {"user", mid(43), mid(143)}, {"department", mid(12), mid(212)}, {"department", mid(42), mid(242)}}
}
func TestRootTemplateReferencesCompleteTypedSortedAndUnchanged(t *testing.T) {
	m := rootBindingManifest()
	before := mbytes(t, m)
	if _, e := NormalizeManifest(m); e != nil {
		t.Fatal("independent valid configuration fixture", e)
	}
	got, e := ExternalReferences(m)
	want := References{[]string{mid(12), mid(40), mid(41), mid(43)}, []string{mid(12), mid(42)}}
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("external references incomplete or mistyped", got, e)
	}
	if string(mbytes(t, m)) != string(before) {
		t.Fatal("reference discovery mutated input")
	}
}
func TestRootTemplateExplicitMappingAllSemanticPositions(t *testing.T) {
	m := rootBindingManifest()
	before := mbytes(t, m)
	got, e := MapExternalReferences(m, rootBindings())
	if e != nil {
		t.Fatal("complete explicit mapping rejected", e)
	}
	if got.Application.ID != m.Application.ID || got.Tables[0].Fields[0].ID != mid(4) || string(got.Tables[0].Fields[0].Default) != `"`+mid(40)+`"` {
		t.Fatal("mapped unrelated text or internal identities")
	}
	if string(got.Tables[0].Fields[1].Default) != `"`+mid(140)+`"` || string(got.Tables[0].Fields[2].Default) != `"`+mid(212)+`"` {
		t.Fatal("typed defaults not mapped")
	}
	if got.PermissionGroups[0].MemberIDs[0] != mid(112) || got.Workflows[0].Graph.Nodes[1].Approval.AssigneeIDs[0] != mid(112) {
		t.Fatal("members or approvers not mapped")
	}
	trigger := string(got.Workflows[0].Triggers[0].Condition)
	if !strings.Contains(trigger, mid(141)) || !strings.Contains(trigger, mid(242)) || !strings.Contains(trigger, mid(99)) {
		t.Fatal("nested trigger or literal text corrupted", trigger)
	}
	if !strings.Contains(string(got.Workflows[0].Graph.Nodes[3].Condition), mid(143)) {
		t.Fatal("routing condition not mapped")
	}
	if string(mbytes(t, m)) != string(before) {
		t.Fatal("mapping mutated caller input")
	}
	got.PermissionGroups[0].MemberIDs[0] = mid(999)
	if string(mbytes(t, m)) != string(before) {
		t.Fatal("mapping aliases caller input")
	}
}
func TestRootTemplateMappingRejectsMissingAmbiguousAndUnexpectedBindings(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "unknown-source", "wrong-kind", "bad-target", "zero-target", "duplicate-member-collapse"} {
		t.Run(mode, func(t *testing.T) {
			m := rootBindingManifest()
			b := rootBindings()
			switch mode {
			case "missing":
				b = b[1:]
			case "duplicate":
				b = append(b, b[0])
			case "unknown-source":
				b = append(b, Binding{"user", mid(888), mid(999)})
			case "wrong-kind":
				b[0].Kind = "department"
			case "bad-target":
				b[0].TargetID = "not-a-user"
			case "zero-target":
				b[0].TargetID = "00000000-0000-0000-0000-000000000000"
			case "duplicate-member-collapse":
				m.PermissionGroups[0].MemberIDs = append(m.PermissionGroups[0].MemberIDs, mid(40))
				b[1].TargetID = b[0].TargetID
			}
			if _, e := MapExternalReferences(m, b); !errors.Is(e, ErrInvalid) {
				t.Fatal("invalid binding accepted", e)
			}
		})
	}
}
func TestRootTemplateEmptyReferencesAndExplicitIdentityMapping(t *testing.T) {
	m := rootManifest()
	m.Workflows = []Workflow{}
	m.PermissionGroups = []PermissionGroup{}
	refs, e := ExternalReferences(m)
	if e != nil || refs.UserIDs == nil || refs.DepartmentIDs == nil || len(refs.UserIDs)+len(refs.DepartmentIDs) != 0 {
		t.Fatal("empty reference set failed", e)
	}
	if _, e = MapExternalReferences(m, []Binding{}); e != nil {
		t.Fatal("explicit empty mapping rejected", e)
	}
	m = rootManifest()
	if _, e = MapExternalReferences(m, []Binding{{"user", mid(12), mid(12)}}); e != nil {
		t.Fatal("explicit identity mapping rejected", e)
	}
	if _, e = MapExternalReferences(m, []Binding{}); !errors.Is(e, ErrInvalid) {
		t.Fatal("implicit identity mapping allowed", e)
	}
}
