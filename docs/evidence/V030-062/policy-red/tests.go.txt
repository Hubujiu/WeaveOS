package appaccess

import (
	"reflect"
	"testing"
)

func TestRootHistoryCompleteTupleIntersection(t *testing.T) {
	p := Policy{ActorID: "u", AppID: "a", OwnerID: "o", ViewID: "v", ResourceExists: true, Grants: []Grant{
		{AppID: "a", ViewID: "v", Action: Read, Scope: All, Fields: []string{"public", "read-only"}},
		{AppID: "a", ViewID: "v", Action: Read, Scope: Own, Fields: []string{"own"}},
		{AppID: "a", ViewID: "v", Action: History, Scope: Own, Fields: []string{"public"}},
		{AppID: "a", ViewID: "v", Action: History, Scope: All, Fields: []string{"own", "history-only"}},
		{AppID: "foreign", ViewID: "v", Action: History, Scope: All, Fields: []string{"read-only"}},
		{AppID: "a", ViewID: "foreign", Action: History, Scope: All, Fields: []string{"read-only"}},
	}}
	if p.HistoryScope() != All || p.FieldScope(History, "public") != Own || p.FieldScope(History, "own") != All {
		t.Fatal("history must preserve its own full tuples")
	}
	ids := []string{"public", "own", "read-only", "history-only", "own", ""}
	if got := p.HistoryFields("u", ids); !reflect.DeepEqual(got, []string{"public", "own"}) {
		t.Fatalf("own intersection: %v", got)
	}
	if got := p.HistoryFields("other", ids); len(got) != 0 {
		t.Fatalf("all scope borrowed another tuple: %v", got)
	}
	if p.FieldScope(Edit, "public") != None || p.CanEdit("u", []string{"public"}) {
		t.Fatal("history accidentally granted edit")
	}
}

func TestRootHistoryEmptyMasksAndLiveResource(t *testing.T) {
	p := Policy{ActorID: "u", AppID: "a", OwnerID: "o", ViewID: "v", ResourceExists: true, Grants: []Grant{
		{AppID: "a", ViewID: "v", Action: Read, Scope: All, Fields: []string{"x"}},
		{AppID: "a", ViewID: "v", Action: History, Scope: All},
	}}
	if p.HistoryScope() != None || len(p.HistoryFields("u", []string{"x"})) != 0 {
		t.Fatal("empty history mask granted access")
	}
	p.OwnerID = "u"
	if p.HistoryScope() != All || !reflect.DeepEqual(p.HistoryFields("other", []string{"x", "x", "y"}), []string{"x", "y"}) {
		t.Fatal("owner historical projection missing")
	}
	p.OwnerID = "o"
	p.BootstrapAdmin = true
	if !reflect.DeepEqual(p.HistoryFields("other", []string{"x"}), []string{"x"}) {
		t.Fatal("bootstrap projection missing")
	}
	p.ResourceExists = false
	if p.HistoryScope() != None || len(p.HistoryFields("u", []string{"x"})) != 0 {
		t.Fatal("bootstrap bypassed missing resource")
	}
}
