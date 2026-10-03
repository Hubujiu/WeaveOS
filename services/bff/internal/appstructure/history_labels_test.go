package appstructure

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHistoryLabelsUseCurrentTombstoneAndExplicitUnavailable(t *testing.T) {
	f := candidateFixture(t)
	view, table := newForm(t, f)
	c := context.Background()
	memberID := uuid(t, f.owner)
	if _, e := f.owner.Exec(c, "INSERT INTO auth.users(id,account) VALUES($1,'history-label-original')", memberID); e != nil {
		t.Fatal(e)
	}
	optionID := uuid(t, f.owner)
	member := field(t, f, "member", nil, map[string]any{})
	option := field(t, f, "single_select", nil, map[string]any{"options": []any{map[string]any{"id": optionID, "label": "original option"}}})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, member, option)), 200)
	id := uuid(t, f.owner)
	w := historyCommand(t, f, view, "record.create")
	op := recordOpID(t, w)
	mid, oid := member["id"].(string), option["id"].(string)
	header, e := (RecordDML{}).Insert(c, w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{mid: memberID, oid: optionID}}, []string{mid, oid})
	if e != nil {
		t.Fatal(e)
	}
	historyComplete(t, w, header, op, 201)
	if e = w.Commit(c); e != nil {
		t.Fatal(e)
	}
	// Owned synthetic source changes exercise the real source/tombstone hooks;
	// no historical values are rewritten and no runtime role gains source writes.
	if _, e = f.owner.Exec(c, "UPDATE auth.users SET account='history-label-current' WHERE id=$1", memberID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(c, `UPDATE applications.fields SET definition=jsonb_set(definition,'{config,options,0,label}','"current option"') WHERE id=$1`, oid); e != nil {
		t.Fatal(e)
	}
	read := func() map[string]map[string]any {
		r := rootCall(t, f, "GET", "/api/v1/applications/"+f.app+"/forms/"+view+"/records/"+id+"/history", nil, f.actor)
		d := data(t, r, 200)
		events := d["items"].([]any)
		if len(events) != 1 {
			t.Fatal("unexpected history", d)
		}
		result := map[string]map[string]any{}
		for _, raw := range events[0].(map[string]any)["changes"].([]any) {
			change := raw.(map[string]any)
			result[change["fieldId"].(string)] = change
		}
		return result
	}
	assertLabel := func(change map[string]any, id, label string, deleted, unavailable bool) {
		t.Helper()
		labels, ok := change["valueLabels"].(map[string]any)
		if !ok || len(labels) != 1 {
			t.Fatal("delta labels must only include its actual stable IDs", change)
		}
		value := labels[id].(map[string]any)
		var expected any = label
		if unavailable {
			expected = nil
		}
		if value["label"] != expected || value["deleted"] != deleted || value["labelUnavailable"] != unavailable {
			t.Fatalf("current/tombstone/unavailable label: %+v", value)
		}
	}
	changes := read()
	assertLabel(changes[mid], memberID, "history-label-current", false, false)
	assertLabel(changes[oid], optionID, "current option", false, false)
	if _, e = f.owner.Exec(c, "DELETE FROM auth.users WHERE id=$1", memberID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(c, `UPDATE applications.fields SET definition=jsonb_set(definition,'{config,options}','[]') WHERE id=$1`, oid); e != nil {
		t.Fatal(e)
	}
	changes = read()
	assertLabel(changes[mid], memberID, "history-label-current", true, false)
	assertLabel(changes[oid], optionID, "current option", true, false)
	if _, e = f.owner.Exec(c, "DELETE FROM applications.field_option_tombstones WHERE field_id=$1 AND option_id=$2", oid, optionID); e != nil {
		t.Fatal(e)
	}
	changes = read()
	assertLabel(changes[oid], optionID, "", false, true)
	// Label projection does not copy current names into canonical old/new values.
	b, _ := json.Marshal(changes[oid])
	if changes[oid]["after"] != optionID {
		t.Fatalf("canonical value rewritten %s", b)
	}
}
