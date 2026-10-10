package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func rootStructureDeleteBody(op string, structure, resource int64) string {
	return fmt.Sprintf(`{"operationId":%q,"expectedStructureVersion":%d,"expectedResourceVersion":%d}`, op, structure, resource)
}
func TestRootStructureDeletionHTTPRoutingAndSharedData(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	base := "/api/v1/applications/" + f.app
	// Create the second view after schema initialization; a new view starts at 0.
	d := rootHTTPData(t, f.call(t, "POST", base+"/forms", `{"operationId":"`+f.id(t)+`","name":"Other","source":{"kind":"existing_table","tableId":"`+f.table+`"},"directoryId":null,"position":0,"expectedStructureVersion":1}`, nil), 201)
	var v struct{ ID string }
	if json.Unmarshal(d["form"], &v) != nil || v.ID == "" {
		t.Fatal("second view setup")
	}
	op := f.id(t)
	got := rootHTTPData(t, f.call(t, "POST", base+"/forms/"+f.view+"/deletion", rootStructureDeleteBody(op, 2, 1), nil), 200)
	if len(got) != 6 || string(got["deleted"]) != "true" || rootHTTPString(t, got, "resourceKind") != "form" {
		t.Fatal("six-key receipt", got)
	}
	rootHTTPData(t, f.call(t, "GET", base+"/forms/"+v.ID+"/records/"+f.record, "", nil), 200)
	rootHTTPData(t, f.call(t, "GET", base+"/forms/"+v.ID+"/records/"+f.record+"/history", "", nil), 200)
	rootHTTPData(t, f.call(t, "GET", "/api/v1/application-operations/"+op, "", nil), 200)
}

func TestRootStructureDeletionHTTPEmptyApplicationRoute(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	app := f.id(t)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.apps(id,name,owner_user_id) VALUES($1,'Empty',$2)", app, f.actor); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.menu_resources VALUES($1,'application',$1)", app); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "SELECT applications.register_catalog_entry($1)", app); e != nil {
		t.Fatal(e)
	}
	rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+app+"/deletion", rootStructureDeleteBody(f.id(t), 0, 1), nil), 200)
}
