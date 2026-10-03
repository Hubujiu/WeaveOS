package apppolicy_test

import (
	"reflect"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
)

func fixture() (apppolicy.TrustedContext, apppolicy.Resource, apppolicy.RowFact) {
	ref := apppolicy.ResourceRef{ApplicationID: "app-a", Kind: "form", ID: "form-a"}
	ctx := apppolicy.TrustedContext{
		Actor:       apppolicy.TrustedActor{ID: "member"},
		Application: apppolicy.Application{ID: "app-a", OwnerID: "owner-a", Exists: true},
		Groups: []apppolicy.PermissionGroup{{ID: "group-a", ApplicationID: "app-a", Enabled: true,
			MemberIDs: []string{"member"}, Grants: []apppolicy.Grant{
				{Resource: ref, Action: apppolicy.MenuEnter, Rows: apppolicy.AllRows},
				{Resource: ref, Action: apppolicy.DataRead, Rows: apppolicy.AllRows, Fields: []string{"title"}},
				{Resource: ref, Action: apppolicy.DataEdit, Rows: apppolicy.OwnRows, Fields: []string{"amount"}},
			}}},
	}
	return ctx, apppolicy.Resource{Ref: ref, Exists: true}, apppolicy.RowFact{
		ApplicationID: "app-a", Exists: true, CreatedBy: "member", LastEditor: "someone-else"}
}

func TestCreateCapabilityIsIndependent(t *testing.T) {
	cases := []struct {
		name  string
		actor apppolicy.TrustedActor
		want  bool
	}{
		{"ordinary", apppolicy.TrustedActor{ID: "member"}, false},
		{"create-only", apppolicy.TrustedActor{ID: "member", CreateApp: true}, true},
		{"bootstrap", apppolicy.TrustedActor{ID: "root", BootstrapAdmin: true}, true},
		{"missing-actor-even-bootstrap", apppolicy.TrustedActor{BootstrapAdmin: true, CreateApp: true}, false},
		{"blank-actor", apppolicy.TrustedActor{ID: "  ", CreateApp: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apppolicy.CanCreate(tc.actor); got != tc.want {
				t.Fatalf("CanCreate = %v; want %v", got, tc.want)
			}
		})
	}
	ctx, resource, row := fixture()
	ctx.Actor.CreateApp = true
	ctx.Groups = nil
	if apppolicy.Allows(ctx, resource, apppolicy.DataEdit, row, "amount") {
		t.Fatal("create-only must not manage another owner's app")
	}
	ctx.Actor.ID = "owner-a"
	ctx.Actor.CreateApp = false
	if !apppolicy.Allows(ctx, resource, apppolicy.DataEdit, row, "amount") {
		t.Fatal("owner retains app rights without current create capability")
	}
	if apppolicy.CanCreate(ctx.Actor) {
		t.Fatal("existing ownership is not global create capability")
	}
}

func TestTrustedBootstrapAndOwnerFullSupportedPrimitives(t *testing.T) {
	for _, actor := range []apppolicy.TrustedActor{{ID: "root", BootstrapAdmin: true}, {ID: "owner-a"}} {
		for _, action := range []apppolicy.Action{apppolicy.MenuEnter, apppolicy.DataRead, apppolicy.DataEdit} {
			t.Run(actor.ID+"/"+string(action), func(t *testing.T) {
				ctx, resource, row := fixture()
				ctx.Actor = actor
				ctx.Groups = nil
				row.CreatedBy = "other"
				field := "unlisted-field"
				if action == apppolicy.MenuEnter {
					field = ""
				}
				if !apppolicy.Allows(ctx, resource, action, row, field) {
					t.Fatal("trusted bootstrap/app owner must have full supported authorization")
				}
			})
		}
	}
	ctx, resource, row := fixture()
	ctx.Actor.ID = "owner-b"
	ctx.Groups = nil
	if apppolicy.Allows(ctx, resource, apppolicy.DataRead, row, "title") {
		t.Fatal("other app's owner is not this app's owner")
	}
	ctx.Actor = apppolicy.TrustedActor{ID: "root", BootstrapAdmin: true}
	ctx.Application.ID = "app-b"
	ctx.Application.OwnerID = "owner-b"
	resource.Ref.ApplicationID = "app-b"
	row.ApplicationID = "app-b"
	if !apppolicy.Allows(ctx, resource, apppolicy.DataRead, row, "title") {
		t.Fatal("bootstrap full applies to each correctly resolved app")
	}
}

func TestInvalidActualContextDeniedBeforeFullAuthorization(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*apppolicy.TrustedContext, *apppolicy.Resource, *apppolicy.RowFact)
	}{
		{"missing-actor", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) { c.Actor.ID = "" }},
		{"missing-app", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) {
			c.Application.Exists = false
		}},
		{"blank-app", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) {
			c.Application.ID = ""
		}},
		{"missing-resource", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) { r.Exists = false }},
		{"blank-resource", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) { r.Ref.ID = "" }},
		{"blank-kind", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) { r.Ref.Kind = "" }},
		{"foreign-resource", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) {
			r.Ref.ApplicationID = "app-b"
		}},
		{"foreign-row", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) {
			row.ApplicationID = "app-b"
		}},
		{"missing-row", func(c *apppolicy.TrustedContext, r *apppolicy.Resource, row *apppolicy.RowFact) { row.Exists = false }},
	}
	for _, tc := range cases {
		for _, subject := range []string{"ordinary", "bootstrap", "owner"} {
			t.Run(tc.name+"/"+subject, func(t *testing.T) {
				ctx, resource, row := fixture()
				if subject == "bootstrap" {
					ctx.Actor.BootstrapAdmin = true
				}
				if subject == "owner" {
					ctx.Actor.ID = "owner-a"
				}
				tc.mutate(&ctx, &resource, &row)
				if apppolicy.Allows(ctx, resource, apppolicy.DataRead, row, "title") {
					t.Fatal("invalid actual context must be denied even with full authorization")
				}
			})
		}
	}
}

func TestAllOwnAndImmutableCreatedBy(t *testing.T) {
	cases := []struct {
		name, creator, editor string
		action                apppolicy.Action
		field                 string
		want                  bool
	}{
		{"all-read-other", "other", "other", apppolicy.DataRead, "title", true},
		{"own-edit", "member", "other", apppolicy.DataEdit, "amount", true},
		{"own-edit-editor-changed", "member", "third", apppolicy.DataEdit, "amount", true},
		{"last-editor-is-not-creator", "other", "member", apppolicy.DataEdit, "amount", false},
		{"missing-creator", "", "member", apppolicy.DataEdit, "amount", false},
		{"not-readable-from-edit", "member", "member", apppolicy.DataRead, "amount", false},
		{"not-editable-from-read", "member", "member", apppolicy.DataEdit, "title", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, resource, row := fixture()
			row.CreatedBy = tc.creator
			row.LastEditor = tc.editor
			if got := apppolicy.Allows(ctx, resource, tc.action, row, tc.field); got != tc.want {
				t.Fatalf("Allows = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestCompleteGrantUnionNeverFormsCartesianProduct(t *testing.T) {
	ctx, resource, row := fixture()
	ctx.Groups = append(ctx.Groups, apppolicy.PermissionGroup{ID: "empty", ApplicationID: "app-a", Enabled: true, MemberIDs: []string{"member"}},
		apppolicy.PermissionGroup{ID: "second", ApplicationID: "app-a", Enabled: true, MemberIDs: []string{"member"}, Grants: []apppolicy.Grant{
			{Resource: resource.Ref, Action: apppolicy.DataRead, Rows: apppolicy.OwnRows, Fields: []string{"amount"}},
			{Resource: resource.Ref, Action: apppolicy.DataEdit, Rows: apppolicy.AllRows, Fields: []string{"memo"}},
		}})
	row.CreatedBy = "other"
	for _, tc := range []struct {
		action apppolicy.Action
		field  string
		want   bool
	}{
		{apppolicy.DataRead, "title", true}, {apppolicy.DataRead, "amount", false},
		{apppolicy.DataEdit, "amount", false}, {apppolicy.DataEdit, "memo", true},
		{apppolicy.DataEdit, "title", false}, {apppolicy.DataRead, "memo", false},
	} {
		t.Run(string(tc.action)+"/other/"+tc.field, func(t *testing.T) {
			if got := apppolicy.Allows(ctx, resource, tc.action, row, tc.field); got != tc.want {
				t.Fatalf("complete-grant union = %v; want %v", got, tc.want)
			}
		})
	}
	if got := apppolicy.AllowedFields(ctx, resource, apppolicy.DataRead, row, []string{"amount", "title"}); !reflect.DeepEqual(got, []string{"title"}) {
		t.Fatalf("other read fields = %v; want [title]", got)
	}
	row.CreatedBy = "member"
	if got := apppolicy.AllowedFields(ctx, resource, apppolicy.DataRead, row, []string{"amount", "title"}); !reflect.DeepEqual(got, []string{"amount", "title"}) {
		t.Fatalf("own read fields = %v; want [amount title]", got)
	}
	if got := apppolicy.AllowedFields(ctx, resource, apppolicy.DataEdit, row, []string{"amount", "memo"}); !reflect.DeepEqual(got, []string{"amount", "memo"}) {
		t.Fatalf("own edit fields = %v; want [amount memo]", got)
	}
}

func TestOnlyEffectiveSameAppMemberGroupsAndExactResources(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*apppolicy.TrustedContext)
	}{
		{"no-group", func(c *apppolicy.TrustedContext) { c.Groups = nil }},
		{"empty-grants", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants = nil }},
		{"disabled-group", func(c *apppolicy.TrustedContext) { c.Groups[0].Enabled = false }},
		{"not-member", func(c *apppolicy.TrustedContext) { c.Groups[0].MemberIDs = []string{"other"} }},
		{"missing-group-id", func(c *apppolicy.TrustedContext) { c.Groups[0].ID = "" }},
		{"foreign-group", func(c *apppolicy.TrustedContext) { c.Groups[0].ApplicationID = "app-b" }},
		{"foreign-grant", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Resource.ApplicationID = "app-b" }},
		{"other-resource", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Resource.ID = "form-b" }},
		{"same-id-other-kind", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Resource.Kind = "view" }},
		{"no-parent-inheritance", func(c *apppolicy.TrustedContext) {
			c.Groups[0].Grants[1].Resource = apppolicy.ResourceRef{ApplicationID: "app-a", Kind: "group", ID: "parent"}
		}},
		{"unsupported-subordinate", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Rows = apppolicy.RowScope("subordinate") }},
		{"empty-row-scope", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Rows = "" }},
		{"empty-field-mask", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Fields = nil }},
		{"no-field-wildcard", func(c *apppolicy.TrustedContext) { c.Groups[0].Grants[1].Fields = []string{"*"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, r, row := fixture()
			tc.mutate(&ctx)
			if apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
				t.Fatal("unqualified or foreign grant must not authorize")
			}
		})
	}
	ctx, r, row := fixture()
	ctx.Groups = append(ctx.Groups, ctx.Groups[0])
	ctx.Groups[0].Grants = nil
	if !apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("empty group does not deny remaining grant")
	}
	ctx.Groups[1].MemberIDs = nil
	if apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("removing last effective membership revokes snapshot authorization")
	}
}

func TestMenuAndDataAreSeparateGates(t *testing.T) {
	ctx, r, row := fixture()
	ctx.Groups[0].Grants = ctx.Groups[0].Grants[:1]
	if !apppolicy.Allows(ctx, r, apppolicy.MenuEnter, apppolicy.RowFact{}, "") {
		t.Fatal("menu-only grant permits menu entry without a record")
	}
	if apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("menu entry is not data authorization")
	}
	if apppolicy.Allows(ctx, r, apppolicy.MenuEnter, row, "title") {
		t.Fatal("menu request must not be confused with a data field request")
	}
	ctx, r, row = fixture()
	ctx.Groups[0].Grants = ctx.Groups[0].Grants[1:]
	if apppolicy.Allows(ctx, r, apppolicy.MenuEnter, apppolicy.RowFact{}, "") {
		t.Fatal("data grant is not menu entry")
	}
	if !apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("data evaluator remains independent of menu evaluator")
	}
	ctx, r, _ = fixture()
	ctx.Groups[0].Grants[0].Rows = apppolicy.OwnRows
	if apppolicy.Allows(ctx, r, apppolicy.MenuEnter, apppolicy.RowFact{}, "") {
		t.Fatal("record own predicate cannot grant a menu without record scope")
	}
}

func TestNoApprovalEligibilityOrUnknownActionFromDataEdit(t *testing.T) {
	for _, action := range []apppolicy.Action{"approve", "unapprove", "uncomplete", "", "unknown"} {
		for _, subject := range []string{"ordinary", "bootstrap", "owner"} {
			t.Run(string(action)+"/"+subject, func(t *testing.T) {
				ctx, r, row := fixture()
				if subject == "bootstrap" {
					ctx.Actor.BootstrapAdmin = true
				}
				if subject == "owner" {
					ctx.Actor.ID = "owner-a"
				}
				if apppolicy.Allows(ctx, r, action, row, "amount") {
					t.Fatal("unsupported action is outside this core; app full is not task eligibility")
				}
			})
		}
	}
}

func TestAllowedFieldsStableUniqueProjectionAndInputPurity(t *testing.T) {
	ctx, r, row := fixture()
	requested := []string{"amount", "title", "title", "", "  ", "other"}
	if got := apppolicy.AllowedFields(ctx, r, apppolicy.DataRead, row, requested); !reflect.DeepEqual(got, []string{"title"}) {
		t.Fatalf("read projection = %v; want [title]", got)
	}
	ctx.Actor.ID = "owner-a"
	got := apppolicy.AllowedFields(ctx, r, apppolicy.DataEdit, row, requested)
	if !reflect.DeepEqual(got, []string{"amount", "title", "other"}) {
		t.Fatalf("owner fields = %v; want [amount title other]", got)
	}
	if len(got) > 0 {
		got[0] = "changed-output"
	}
	if !reflect.DeepEqual(requested, []string{"amount", "title", "title", "", "  ", "other"}) {
		t.Fatal("projection mutated or aliased requested fields")
	}
	if !reflect.DeepEqual(ctx.Groups[0].Grants[1].Fields, []string{"title"}) {
		t.Fatal("evaluator mutated policy snapshot")
	}
	for _, action := range []apppolicy.Action{apppolicy.MenuEnter, "approve"} {
		if len(apppolicy.AllowedFields(ctx, r, action, row, requested)) != 0 {
			t.Fatal("non-data action has no field projection")
		}
	}
	if apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "") {
		t.Fatal("empty data field must fail closed even for owner")
	}
}

func TestReplacedSnapshotsDoNotReuseEarlierAuthorization(t *testing.T) {
	ctx, r, row := fixture()
	if !apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("initial snapshot should allow title")
	}
	ctx.Groups = nil
	if apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("next supplied snapshot must not reuse earlier grants")
	}
	ctx, r, row = fixture()
	ctx.Actor.ID = "owner-a"
	ctx.Actor.CreateApp = false
	if !apppolicy.Allows(ctx, r, apppolicy.DataRead, row, "title") {
		t.Fatal("fixed owner does not depend on group membership or create grant")
	}
}
