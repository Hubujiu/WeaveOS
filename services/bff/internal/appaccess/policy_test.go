package appaccess

import (
	"reflect"
	"testing"
)

func TestCompleteGrantTuplesAndCreateEmptyMask(t *testing.T) {
	p := Policy{ActorID: "u", AppID: "a", OwnerID: "o", ViewID: "f", ResourceExists: true, Grants: []Grant{
		{AppID: "a", ViewID: "f", Action: Read, Scope: All, Fields: []string{"x"}},
		{AppID: "a", ViewID: "f", Action: Read, Scope: Own, Fields: []string{"y"}},
		{AppID: "a", ViewID: "f", Action: Edit, Scope: Own, Fields: []string{"x"}},
		{AppID: "a", ViewID: "f", Action: Create, Scope: All},
		{AppID: "other", ViewID: "f", Action: Edit, Scope: All, Fields: []string{"y"}},
		{AppID: "a", ViewID: "other", Action: Read, Scope: All, Fields: []string{"secret"}},
	}}
	if p.VisibleScope() != All || p.FieldScope(Read, "x") != All || p.FieldScope(Read, "y") != Own {
		t.Fatal("read row and field scopes must follow the complete tuple")
	}
	if p.CoversRead([]string{"y"}) || !p.CoversRead([]string{"x"}) {
		t.Fatal("filter/sort field scope must cover all visible rows")
	}
	if got := p.ReadFields("other", []string{"x", "y", "secret"}); !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("cross-app/view or own field leaked: %v", got)
	}
	if got := p.ReadFields("u", []string{"x", "y", "x"}); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatalf("own field mask/duplicate wrong: %v", got)
	}
	if !p.CanCreate(nil) || p.CanCreate([]string{"x"}) {
		t.Fatal("explicit empty create mask allows defaults only")
	}
	if !p.CanEdit("u", []string{"x"}) || p.CanEdit("other", []string{"x"}) || p.CanEdit("u", []string{"y"}) {
		t.Fatal("own edit must not borrow another action, scope or app")
	}
}

func TestNoReadableFieldNoRowAndRealResourceGate(t *testing.T) {
	p := Policy{ActorID: "u", AppID: "a", OwnerID: "o", ViewID: "f", ResourceExists: true,
		Grants: []Grant{{AppID: "a", ViewID: "f", Action: Read, Scope: All}}}
	if p.VisibleScope() != None || len(p.ReadFields("u", []string{"x"})) != 0 {
		t.Fatal("empty read mask cannot contribute row to COUNT/page")
	}
	p.OwnerID = "u"
	if p.VisibleScope() != All || !p.CoversRead([]string{"x"}) || !p.CanCreate([]string{"x"}) {
		t.Fatal("actual owner has supported full capability")
	}
	p.ResourceExists = false
	if p.VisibleScope() != None || p.CanCreate(nil) || p.CanEdit("u", []string{"x"}) {
		t.Fatal("owner cannot bypass resource existence")
	}
}
