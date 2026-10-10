package applications

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"testing"
)

func TestRootSharedRecordPolicyPreservesTrustedTupleBoundaries(t *testing.T) {
	const app = "11111111-1111-4111-8111-111111111111"
	const view = "22222222-2222-4222-8222-222222222222"
	const table = "33333333-3333-4333-8333-333333333333"
	const field = "44444444-4444-4444-8444-444444444444"
	facts := RecordContext{Actor: apppolicy.TrustedActor{ID: "actor"}, App: App{ID: app, OwnerUserID: "owner"}, ViewID: view, TableID: table, Grants: []Grant{
		{ResourceKind: "form", ResourceID: view, Action: "menu.enter"},
		{ResourceKind: "form", ResourceID: view, Action: "data.read", RowScope: "own", Fields: []string{field}},
		{ResourceKind: "form", ResourceID: "other", Action: "data.read", RowScope: "all", Fields: []string{field}},
		{ResourceKind: "directory", ResourceID: view, Action: "data.edit", RowScope: "all", Fields: []string{field}},
	}}
	p, menu := RecordPolicy(facts)
	if !menu || p.ActorID != "actor" || p.OwnerID != "owner" || p.AppID != app || p.ViewID != view || len(p.Grants) != 1 || p.VisibleScope() != appaccess.Own || p.FieldScope(appaccess.Edit, field) != appaccess.None {
		t.Fatalf("trusted tuples incorrectly combined: %+v menu=%v", p, menu)
	}
	facts.Grants = facts.Grants[1:]
	_, menu = RecordPolicy(facts)
	if menu {
		t.Fatal("data.read granted menu")
	}
	facts.Grants[0].RowScope = "subordinate"
	_, menu = RecordPolicy(facts)
	if menu {
		t.Fatal("invalid scope did not fail closed")
	}
	facts.Actor.ID = "owner"
	facts.Grants = nil
	p, menu = RecordPolicy(facts)
	if !menu || p.VisibleScope() != appaccess.All {
		t.Fatal("owner lost current real resource access")
	}
	facts.TableID = ""
	p, _ = RecordPolicy(facts)
	if p.VisibleScope() != appaccess.None {
		t.Fatal("owner bypassed resource existence")
	}
	facts.Actor = apppolicy.TrustedActor{ID: "root", BootstrapAdmin: true}
	facts.TableID = table
	p, menu = RecordPolicy(facts)
	if !menu || p.VisibleScope() != appaccess.All {
		t.Fatal("trusted bootstrap not reflected")
	}
}
