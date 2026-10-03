package appstructure

import (
	"context"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestNumericPolicyChangeKeepsConfiguredDefault(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	money := field(t, f, "money", "1.00", map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, money)), 200)
	money["config"] = map[string]any{"precision": 8, "scale": 2}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, money)), 200)
	insertRow(t, f, table, nil)
	var value *string
	e := f.owner.QueryRow(context.Background(), "SELECT "+columnPhysical(money["id"].(string))+"::text FROM "+tablePhysical(table)).Scan(&value)
	if e != nil || value == nil || *value != "1.00" {
		t.Fatal("same-type numeric policy lost configured physical default", value, e)
	}
}
func TestLayoutOnlyNeverDeduplicatesExistingMultiColumn(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	opt := uuid(t, f.owner)
	choice := field(t, f, "multi_select", nil, map[string]any{"options": []any{map[string]any{"id": opt, "label": "A"}}})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, choice)), 200)
	insertRow(t, f, table, map[string]any{choice["id"].(string): []string{opt, opt}})
	var revision int
	f.owner.QueryRow(context.Background(), "SELECT data_revision FROM applications.logical_tables WHERE id=$1", table).Scan(&revision)
	in := input(t, f, 1, 1, choice)
	in["layout"] = []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": choice["id"], "span": 6}}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	var length, after int
	e := f.owner.QueryRow(context.Background(), "SELECT cardinality("+columnPhysical(choice["id"].(string))+"),(SELECT data_revision FROM applications.logical_tables WHERE id=$1) FROM "+tablePhysical(table), table).Scan(&length, &after)
	if e != nil || length != 2 || after != revision {
		t.Fatal("layout-only rewrote multi values", length, revision, after, e)
	}
}
func TestExplicitMultiMappingDeduplicatesRealUUIDArray(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	old, next := uuid(t, f.owner), uuid(t, f.owner)
	choice := field(t, f, "multi_select", nil, map[string]any{"options": []any{map[string]any{"id": old, "label": "旧"}, map[string]any{"id": next, "label": "新"}}})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, choice)), 200)
	insertRow(t, f, table, map[string]any{choice["id"].(string): []string{old, old, next}})
	choice["config"] = map[string]any{"options": []any{map[string]any{"id": next, "label": "新"}}}
	in := input(t, f, 1, 1, choice)
	in["optionMappings"] = []any{map[string]any{"fieldId": choice["id"], "fromOptionId": old, "toOptionId": next}}
	out := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	in["confirmationToken"] = out["confirmation"].(map[string]any)["token"]
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	var ids []string
	e := f.owner.QueryRow(context.Background(), "SELECT "+columnPhysical(choice["id"].(string))+" FROM "+tablePhysical(table)).Scan(&ids)
	if e != nil || len(ids) != 1 || ids[0] != next {
		t.Fatal("mapped set-valued UUID column", ids, e)
	}
}
func TestExplicitZeroSpanAndUnknownMappingsReject(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	in := input(t, f, 0, 0, text)
	in["layout"] = []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": text["id"], "span": 0}}
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 400, "COMMON_VALIDATION_FAILED")
	in["layout"] = []any{}
	in["optionMappings"] = []any{map[string]any{"fieldId": uuid(t, f.owner), "fromOptionId": uuid(t, f.owner), "toOptionId": nil}}
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 400, "COMMON_VALIDATION_FAILED")
}
func TestTimestampToTextUsesCanonicalUTC(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	dt := field(t, f, "datetime", nil, map[string]any{"precision": "millisecond"})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, dt)), 200)
	insertRow(t, f, table, map[string]any{dt["id"].(string): "2000-02-29T01:02:03.123Z"})
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	text["id"] = dt["id"]
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, text)), 200)
	var value string
	f.owner.QueryRow(context.Background(), "SELECT "+columnPhysical(dt["id"].(string))+" FROM "+tablePhysical(table)).Scan(&value)
	if value != "2000-02-29T01:02:03.123Z" {
		t.Fatal("scalar text serialization must remain canonical UTC", value)
	}
}
func TestRealRestrictedRoleCannotAlterDropOrExecuteViaPublic(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0)), 200)
	for _, sql := range []string{"ALTER TABLE " + tablePhysical(table) + " ADD COLUMN escaped text", "DROP TABLE " + tablePhysical(table)} {
		if _, e := f.runtime.Exec(context.Background(), sql); e == nil {
			t.Fatal("arbitrary runtime DDL allowed")
		}
	}
	var publics int
	e := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL aclexplode(p.proacl) a WHERE n.nspname='applications' AND p.proname IN ('apply_schema_change','apply_option_mapping') AND a.grantee=0 AND a.privilege_type='EXECUTE'").Scan(&publics)
	if e != nil || publics != 0 {
		t.Fatal("PUBLIC can call DDL", publics, e)
	}
	_, e = f.runtime.Exec(context.Background(), "SELECT applications.apply_schema_change($1,$2,$3,'execute_sql',NULL,NULL)", f.actor, f.app, table)
	if e == nil {
		t.Fatal("nonallowlisted DDL operation accepted")
	}
	_ = pgx.ErrNoRows
}
