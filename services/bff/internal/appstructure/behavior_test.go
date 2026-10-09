package appstructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func newForm(t *testing.T, f *fixture) (string, string) {
	t.Helper()
	d := data(t, f.call(t, "POST", "/forms", map[string]any{"operationId": uuid(t, f.owner), "name": "表单", "source": map[string]any{"kind": "new_table"}, "directoryId": nil, "position": 0, "expectedStructureVersion": 0}), 201)
	return d["form"].(map[string]any)["id"].(string), d["table"].(map[string]any)["id"].(string)
}
func field(t *testing.T, f *fixture, kind string, def any, config any) map[string]any {
	return map[string]any{"id": uuid(t, f.owner), "name": kind, "kind": kind, "required": false, "default": def, "config": config, "presentation": map[string]any{"helpText": nil}}
}
func input(t *testing.T, f *fixture, schema, view int, fields ...map[string]any) map[string]any {
	fs := []any{}
	for _, v := range fields {
		fs = append(fs, v)
	}
	return map[string]any{"operationId": uuid(t, f.owner), "expectedSchemaVersion": schema, "expectedViewVersion": view, "fields": fs, "layout": []any{}, "optionMappings": []any{}, "confirmationToken": nil}
}
func preflight(in map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"expectedSchemaVersion", "expectedViewVersion", "fields", "layout", "optionMappings"} {
		out[k] = in[k]
	}
	return out
}
func expectError(t *testing.T, f *fixture, method, path string, body any, status int, want string) {
	t.Helper()
	w := f.call(t, method, path, body)
	var envelope struct{ Code string }
	json.Unmarshal(w.Body.Bytes(), &envelope)
	if w.Code != status || envelope.Code != want {
		t.Fatalf("want %d %s got %d %s", status, want, w.Code, w.Body.String())
	}
}
func insertRow(t *testing.T, f *fixture, table string, values map[string]any) {
	t.Helper()
	cols := []string{"id", "created_by"}
	args := []any{uuid(t, f.owner), f.actor}
	for k, v := range values {
		cols = append(cols, columnPhysical(k))
		args = append(args, v)
	}
	slots := []string{}
	for i := range args {
		slots = append(slots, fmt.Sprintf("$%d", i+1))
	}
	_, e := f.owner.Exec(context.Background(), "INSERT INTO "+tablePhysical(table)+"("+strings.Join(cols, ",")+") VALUES("+strings.Join(slots, ",")+")", args...)
	if e != nil {
		t.Fatal(e)
	}
}
func TestExactStoredDefaultsAndLayoutOnlyPreservesRows(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", "'); DROP TABLE auth.users; --", map[string]any{"maxLength": nil})
	money := field(t, f, "money", "12345678901234567890.125", map[string]any{"roundingMode": "HALF_EVEN"})
	date := field(t, f, "date", "2000-02-29", map[string]any{})
	dt := field(t, f, "datetime", "1969-12-31T23:59:59.999999999Z", map[string]any{"precision": "millisecond"})
	boolean := field(t, f, "boolean", false, map[string]any{})
	in := input(t, f, 0, 0, text, money, date, dt, boolean)
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	insertRow(t, f, table, nil)
	var a, b, c, d string
	var flag bool
	query := "SELECT " + columnPhysical(text["id"].(string)) + "," + columnPhysical(money["id"].(string)) + "::text," + columnPhysical(date["id"].(string)) + "::text,to_char(" + columnPhysical(dt["id"].(string)) + " AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS.MS')," + columnPhysical(boolean["id"].(string)) + " FROM " + tablePhysical(table)
	if e := f.owner.QueryRow(context.Background(), query).Scan(&a, &b, &c, &d, &flag); e != nil || a != "'); DROP TABLE auth.users; --" || b != "12345678901234567890.12" || c != "2000-02-29" || d != "1969-12-31 23:59:59.999" || flag {
		t.Fatalf("exact default literals %q %q %q %q %v %v", a, b, c, d, flag, e)
	}
	in = input(t, f, 1, 1, text, money, date, dt, boolean)
	in["layout"] = []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": text["id"], "span": 4}}
	result := data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)["definition"].(map[string]any)
	if result["table"].(map[string]any)["schemaVersion"] != float64(1) || result["form"].(map[string]any)["viewVersion"] != float64(2) {
		t.Fatal("separate layout version")
	}
	var after string
	f.owner.QueryRow(context.Background(), "SELECT "+columnPhysical(money["id"].(string))+"::text FROM "+tablePhysical(table)).Scan(&after)
	if after != b {
		t.Fatal("layout rewrote business values")
	}
}
func TestNewReferenceDefaultFailsClosedWithoutSourceValidator(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	member := field(t, f, "member", uuid(t, f.owner), map[string]any{})
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, member), 503, "COMMON_SERVICE_UNAVAILABLE")
}
func TestNormalizedDefaultsReplaySameOperation(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	money := field(t, f, "money", nil, map[string]any{})
	in := input(t, f, 0, 0, money)
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	money["config"] = map[string]any{"precision": 38, "scale": 2, "roundingPlaces": 2, "roundingMode": "HALF_UP"}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	in["operationId"] = uuid(t, f.owner)
	in["expectedSchemaVersion"], in["expectedViewVersion"] = 1, 1
	d := data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)["definition"].(map[string]any)
	if d["table"].(map[string]any)["schemaVersion"] != float64(1) || d["form"].(map[string]any)["viewVersion"] != float64(1) {
		t.Fatal("no-op incremented versions")
	}
}
func TestOptionsMappingChangesRealValuesAndPreflightIsReadOnly(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	old, new := uuid(t, f.owner), uuid(t, f.owner)
	choice := field(t, f, "single_select", nil, map[string]any{"options": []any{map[string]any{"id": old, "label": "旧"}, map[string]any{"id": new, "label": "新"}}})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, choice)), 200)
	insertRow(t, f, table, map[string]any{choice["id"].(string): old})
	choice["config"] = map[string]any{"options": []any{map[string]any{"id": new, "label": "新"}}}
	in := input(t, f, 1, 1, choice)
	out := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	if out["saveAllowed"] != false {
		t.Fatal("used removed option accepted without explicit mapping")
	}
	in["optionMappings"] = []any{map[string]any{"fieldId": choice["id"], "fromOptionId": old, "toOptionId": new}}
	var count int
	f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&count)
	out = data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	if out["saveAllowed"] != true || out["confirmation"] == nil {
		t.Fatal("mapping preflight", out)
	}
	var afterCount int
	f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&afterCount)
	if count != afterCount {
		t.Fatal("preflight persisted ledger")
	}
	var oldValue string
	f.owner.QueryRow(context.Background(), "SELECT "+columnPhysical(choice["id"].(string))+"::text FROM "+tablePhysical(table)).Scan(&oldValue)
	if oldValue != old {
		t.Fatal("preview wrote business rows")
	}
	in["confirmationToken"] = out["confirmation"].(map[string]any)["token"]
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	var value string
	f.owner.QueryRow(context.Background(), "SELECT "+columnPhysical(choice["id"].(string))+"::text FROM "+tablePhysical(table)).Scan(&value)
	if value != new {
		t.Fatalf("real option column want %s got %s", new, value)
	}
}
func TestConfirmationSameCountChangeExpiryAndReplay(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	insertRow(t, f, table, map[string]any{text["id"].(string): "old"})
	in := input(t, f, 1, 1)
	path := "/forms/" + view + "/definition"
	out := data(t, f.call(t, "POST", path+"/preflight", preflight(in)), 200)
	in["confirmationToken"] = out["confirmation"].(map[string]any)["token"]
	f.owner.Exec(context.Background(), "UPDATE "+tablePhysical(table)+" SET "+columnPhysical(text["id"].(string))+"='changed'")
	expectError(t, f, "PUT", path, in, 409, "APPLICATION_SCHEMA_CONFIRMATION_STALE")
	out = data(t, f.call(t, "POST", path+"/preflight", preflight(in)), 200)
	in["confirmationToken"] = out["confirmation"].(map[string]any)["token"]
	data(t, f.call(t, "PUT", path, in), 200)
	f.service.Application.Now = func() time.Time { return time.Now().Add(time.Hour) }
	data(t, f.call(t, "PUT", path, in), 200)
}
func TestRequiredAndConversionFailureRollBack(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	insertRow(t, f, table, map[string]any{text["id"].(string): "not-a-number"})
	number := field(t, f, "number", nil, map[string]any{})
	number["id"] = text["id"]
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, number), 409, "APPLICATION_SCHEMA_CONVERSION_FAILED")
	required := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	required["required"] = true
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, text, required), 409, "APPLICATION_SCHEMA_REQUIRED_BACKFILL")
	var version int
	f.owner.QueryRow(context.Background(), "SELECT schema_version FROM applications.logical_tables WHERE id=$1", table).Scan(&version)
	if version != 1 {
		t.Fatal("failed save changed metadata")
	}
}
