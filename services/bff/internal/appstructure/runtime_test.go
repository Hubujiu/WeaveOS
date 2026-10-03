package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"strings"
	"testing"
)

func TestRecordReadResolvesLiveNonManagerInOneReadOnlyRR(t *testing.T) {
	owner, member := setup(t), setup(t)
	view, table := newForm(t, owner)
	text := field(t, owner, "text", "visible", map[string]any{"maxLength": nil})
	data(t, owner.call(t, "PUT", "/forms/"+view+"/definition", input(t, owner, 0, 0, text)), 200)
	c := context.Background()
	var group string
	if e := owner.owner.QueryRow(c, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'read scope') RETURNING id::text", owner.app).Scan(&group); e != nil {
		t.Fatal(e)
	}
	if _, e := owner.owner.Exec(c, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", owner.app, group, member.actor); e != nil {
		t.Fatal(e)
	}
	if _, e := owner.owner.Exec(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'menu.enter','all')", owner.app, group, view); e != nil {
		t.Fatal(e)
	}
	p := session.Principal{UserID: member.actor, Record: session.Record{AuthVersion: "1"}}
	a := &applications.Application{Pool: member.runtime}
	tx, facts, e := a.BeginRecordRead(c, p, owner.app, view)
	if e != nil {
		t.Fatalf("non-manager read must resolve real registered metadata: %v", e)
	}
	defer tx.Rollback(c)
	if facts.Actor.ID != member.actor || facts.App.OwnerUserID != owner.actor || facts.TableID != table || facts.SchemaVersion != 1 || len(facts.Grants) != 1 || facts.Grants[0].ResourceID != view {
		t.Fatalf("live snapshot facts: %+v", facts)
	}
	var isolation, mode string
	if e = tx.QueryRow(c, "SELECT current_setting('transaction_isolation'),current_setting('transaction_read_only')").Scan(&isolation, &mode); e != nil || isolation != "repeatable read" || mode != "on" {
		t.Fatal(isolation, mode, e)
	}
	if _, e = owner.owner.Exec(c, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", owner.app); e != nil {
		t.Fatal(e)
	}
	var revision int64
	if e = tx.QueryRow(c, "SELECT policy_revision FROM applications.apps WHERE id=$1", owner.app).Scan(&revision); e != nil || revision != facts.App.PolicyRevision {
		t.Fatal("RR escaped current snapshot", revision, e)
	}
	tx.Rollback(c)
	p.Record.AuthVersion = "2"
	if tx, _, e = a.BeginRecordRead(c, p, owner.app, view); !errors.Is(e, session.ErrUnauthorized) {
		if tx != nil {
			tx.Rollback(c)
		}
		t.Fatal("stale Session was not rejected", e)
	}
	p.Record.AuthVersion = "1"
	if tx, _, e = a.BeginRecordRead(c, p, member.app, view); !errors.Is(e, applications.ErrMissing) {
		if tx != nil {
			tx.Rollback(c)
		}
		t.Fatal("cross-app form resolved", e)
	}
}

func TestOwnerRuntimeHTTPUsesRealResourceAndSafeMinuteInput(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	expectError(t, f, "GET", "/forms/"+view+"/runtime", nil, 409, "APPLICATION_SCHEMA_NOT_READY")
	stamp := field(t, f, "datetime", nil, map[string]any{"precision": "minute"})
	money := field(t, f, "money", "1.235", map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, stamp, money)), 200)
	d := data(t, rootCall(t, f, "GET", "/api/v1/applications/"+f.app+"/forms/"+view+"/runtime", nil, f.actor), 200)
	fields := d["fields"].([]any)
	if len(fields) != 2 {
		t.Fatal(d)
	}
	byKind := map[string]map[string]any{}
	for _, raw := range fields {
		v := raw.(map[string]any)
		byKind[v["kind"].(string)] = v
		if _, ok := v["config"]; ok {
			t.Fatal("raw config leaked", v)
		}
	}
	if byKind["datetime"]["input"].(map[string]any)["timePrecision"] != "minute" || byKind["money"]["default"] != "1.24" {
		t.Fatal("runtime disagrees with stored canonical definition", d)
	}
	expectError(t, f, "GET", "/forms/"+uuid(t, f.owner)+"/runtime", nil, 404, "APPLICATION_NOT_FOUND")
}

func TestRuntimeProjectionPrunesHiddenFieldsDefaultsAndEmptyGroups(t *testing.T) {
	readID, createID, hiddenID := "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", "00000000-0000-0000-0000-000000000003"
	fields := []appfields.Field{{ID: readID, Name: "read", Kind: "text", Default: json.RawMessage(`"secret default"`), Config: json.RawMessage(`{}`)}, {ID: createID, Name: "input", Kind: "text", Default: json.RawMessage(`"allowed default"`), Config: json.RawMessage(`{}`)}, {ID: hiddenID, Name: "hidden secret name", Kind: "text", Config: json.RawMessage(`{}`)}}
	layout := []appfields.LayoutNode{{ID: readID, Kind: "group", Title: "keep", Children: []appfields.LayoutNode{{ID: createID, Kind: "field", FieldID: readID}}}, {ID: hiddenID, Kind: "group", Title: "drop", Children: []appfields.LayoutNode{{ID: readID, Kind: "field", FieldID: hiddenID}}}}
	facts := applications.RecordContext{SchemaReady: true, Fields: marshalRuntime(t, fields), Layout: marshalRuntime(t, layout)}
	access := RecordAccess{MenuEnter: true, Create: true, Read: "all", Edit: "none", History: "none", Fields: map[string]FieldAccess{readID: {Read: "own", Edit: "none"}, createID: {Read: "none", Create: true, Edit: "none"}}}
	v, e := ProjectRuntime(facts, access)
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Fields) != 2 || len(v.Layout) != 1 || v.Fields[0].Default != nil || string(v.Fields[1].Default) != `"allowed default"` || len(v.Fields[0].Query.Operators) != 0 {
		t.Fatalf("permission-safe runtime: %+v", v)
	}
	raw := string(marshalRuntime(t, v))
	if strings.Contains(raw, "hidden secret") || strings.Contains(raw, "secret default") || strings.Contains(raw, "config") {
		t.Fatal("hidden metadata escaped", raw)
	}
	access.Create = false
	access.Read = "none"
	access.Edit = "none"
	if _, e = ProjectRuntime(facts, access); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("menu-only must deny", e)
	}
}
func marshalRuntime(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
