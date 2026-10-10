package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRootPrivatePresetsHTTPPreservesOldQueryProjectionAndBusinessState(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	base := "/api/v1/applications/" + f.app + "/forms/" + f.view
	definition := rootHTTPData(t, f.call(t, "GET", base+"/definition", "", nil), 200)
	var fields []map[string]any
	if json.Unmarshal(definition["fields"], &fields) != nil {
		t.Fatal("definition fixture")
	}
	second := f.id(t)
	fields = append(fields, map[string]any{"id": second, "name": "Remaining visible field", "kind": "text", "required": false, "default": nil, "config": map[string]any{"maxLength": nil}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}})
	raw, _ := json.Marshal(map[string]any{"operationId": f.id(t), "expectedSchemaVersion": 1, "expectedViewVersion": 1, "fields": fields, "layout": []any{}, "optionMappings": []any{}, "confirmationToken": nil})
	rootHTTPData(t, f.call(t, "PUT", base+"/definition", string(raw), nil), 200)
	query := `{"page":1,"pageSize":20,"filter":null,"sort":null}`
	first := rootHTTPData(t, f.call(t, "POST", base+"/records/search", query, nil), 200)
	token := rootHTTPString(t, first, "queryVersion")
	oldQuery := strings.TrimSuffix(query, "}") + `,"queryVersion":"` + token + `"}`
	if _, e := f.owner.Exec(f.ctx, `INSERT INTO personnel.table_presets(owner_id,view_key,name,slot,filter_json,hidden_column_ids) VALUES($1,'members','old-personnel-preset',1,NULL,'[]')`, f.actor); e != nil {
		t.Fatal(e)
	}
	snapshot := func() string {
		var value string
		e := f.owner.QueryRow(f.ctx, `SELECT jsonb_build_object(
 'app',(SELECT to_jsonb(a) FROM applications.apps a WHERE a.id=$1),
 'table',(SELECT to_jsonb(t) FROM applications.logical_tables t WHERE t.id=$2),
 'revisions',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.scope) FROM personnel.query_revisions r),
 'audit',(SELECT count(*) FROM auth.authentication_events),
 'recordAudit',(SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1),
 'personnel',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM personnel.table_presets p WHERE owner_id=$3))::text`, f.app, f.table, f.actor).Scan(&value)
		if e != nil {
			t.Fatal(e)
		}
		return value
	}
	before := snapshot()
	config := rootPresetRequest(t, f, "隐藏原字段").State
	config.HiddenColumnIDs = []string{f.field}
	op := f.id(t)
	bytes, _ := json.Marshal(config)
	body := `{"operationId":"` + op + `","expectedSchemaVersion":2,` + string(bytes[1:])
	created := rootHTTPData(t, f.call(t, "POST", rootPresetHTTPPath(f), body, nil), 201)
	id := rootHTTPString(t, created, "id")
	after := rootHTTPData(t, f.call(t, "POST", base+"/records/search", oldQuery, nil), 200)
	var rows []struct{ Values map[string]json.RawMessage }
	if json.Unmarshal(after["items"], &rows) != nil || len(rows) != 1 || string(rows[0].Values[f.field]) != `"original"` {
		t.Fatal("hidden field removed from complete authorized projection")
	}
	if current := snapshot(); current != before {
		t.Fatal("saving private preference changed business revision/audit/personnel preset", before, current)
	}
	update := strings.Replace(body, op, f.id(t), 1)
	update = strings.Replace(update, `"expectedSchemaVersion":2`, `"expectedSchemaVersion":2,"expectedVersion":1`, 1)
	rootHTTPData(t, f.call(t, "PUT", rootPresetHTTPPath(f)+"/"+id, update, nil), 200)
	deleted := f.call(t, "DELETE", rootPresetHTTPPath(f)+"/"+id+"?operationId="+f.id(t)+"&expectedVersion=2", "", nil)
	if deleted.Code != 204 {
		t.Fatal("delete", deleted.Code)
	}
	rootHTTPData(t, f.call(t, "POST", base+"/records/search", oldQuery, nil), 200)
	if current := snapshot(); current != before {
		t.Fatal("update/discard changed business state or old personnel presets")
	}
	change := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":2,"expectedRecordVersion":1,"changes":{"` + f.field + `":"changed"}}`
	rootHTTPData(t, f.call(t, "PATCH", base+"/records/"+f.record, change, nil), 200)
	conflict := f.call(t, "POST", base+"/records/search", oldQuery, nil)
	var env struct{ Code string }
	json.Unmarshal(conflict.Body.Bytes(), &env)
	if conflict.Code != 409 || env.Code != "APPLICATION_QUERY_CHANGED" {
		t.Fatal("original query change detection was bypassed", conflict.Code, env.Code)
	}
}
