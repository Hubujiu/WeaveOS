package appschema

import (
	"errors"
	"reflect"
	"testing"
)

const (
	tableID   = "00000000-0000-4000-8000-000000000001"
	fieldID1  = "00000000-0000-4000-8000-000000000011"
	fieldID2  = "00000000-0000-4000-8000-000000000012"
	fieldID3  = "00000000-0000-4000-8000-000000000013"
	tableName = "t_00000000000040008000000000000001"
	column1   = "f_00000000000040008000000000000011"
	column2   = "f_00000000000040008000000000000012"
	column3   = "f_00000000000040008000000000000013"
)

func textValue(s string) *Value { return &Value{Type: Text, Text: s} }
func boolValue(b bool) *Value   { return &Value{Type: Boolean, Boolean: b} }

// Oracle: PRD AP-FR-02 and B2 PLAN, not the implementation's SQL output.
func TestPlanStableIDsAndMetadataRename(t *testing.T) {
	before := []Field{{ID: fieldID1, Name: "旧名称", Type: Text}}
	after := []Field{{ID: fieldID1, Name: `新名称"; DROP SCHEMA auth CASCADE;--`, Type: Text}}
	p, err := BuildPlan(tableID, before, after)
	if err != nil {
		t.Fatalf("display-only rename must be plannable: %v", err)
	}
	if p.TableName != tableName || len(p.Changes) != 0 || !reflect.DeepEqual(p.Fields, after) {
		t.Fatalf("stable mapping and metadata-only rename required: %+v", p)
	}
	after[0].Name = "caller changed"
	if p.Fields[0].Name != `新名称"; DROP SCHEMA auth CASCADE;--` {
		t.Fatal("plan must own its field snapshot")
	}
}

func TestPlanTypedAddChangeAndDrop(t *testing.T) {
	p, err := BuildPlan(tableID, []Field{{ID: fieldID1, Type: Text}, {ID: fieldID3, Type: Boolean}},
		[]Field{{ID: fieldID2, Type: Boolean, Default: boolValue(true)}, {ID: fieldID1, Type: Boolean, Required: true}})
	if err != nil {
		t.Fatalf("confirmed typed schema diff must be planned: %v", err)
	}
	want := []struct {
		op  Operation
		col string
	}{{AddColumn, column2}, {AlterType, column1}, {AlterRequired, column1}, {DropColumn, column3}}
	if len(p.Changes) != len(want) {
		t.Fatalf("changes: %+v", p.Changes)
	}
	for i, w := range want {
		if p.Changes[i].Operation != w.op || p.Changes[i].Column != w.col {
			t.Fatalf("change %d: %+v", i, p.Changes[i])
		}
	}
}

func TestPlanRejectsUnsafeIDsAndUnfrozenPhysicalTypes(t *testing.T) {
	for _, id := range []string{"display name", `x";DROP TABLE auth.users;--`, "00000000-0000-4000-8000-00000000001", "00000000-0000-4000-8000-00000000001A"} {
		_, err := BuildPlan(id, nil, []Field{{ID: fieldID1, Type: Text}})
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("unsafe table ID %q must be rejected: %v", id, err)
		}
		_, err = BuildPlan(tableID, nil, []Field{{ID: id, Type: Text}})
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("unsafe field ID %q must be rejected: %v", id, err)
		}
	}
	for _, typ := range []Type{"jsonb", "numeric", "timestamp", "text);DROP SCHEMA auth CASCADE;--"} {
		_, err := BuildPlan(tableID, nil, []Field{{ID: fieldID1, Type: typ}})
		if !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("unregistered primitive %q must fail closed: %v", typ, err)
		}
	}
	_, err := BuildPlan(tableID, nil, []Field{{ID: fieldID1, Type: Text}, {ID: fieldID1, Type: Boolean}})
	if !errors.Is(err, ErrInvalid) {
		t.Error("duplicate stable IDs must be rejected")
	}
	_, err = BuildPlan(tableID, nil, []Field{{ID: fieldID1, Type: Boolean, Default: textValue("true")}})
	if !errors.Is(err, ErrInvalid) {
		t.Error("default must match the declared physical type")
	}
	_, err = BuildPlan(tableID, nil, []Field{{ID: fieldID1, Type: Text, Default: textValue("nul\x00byte")}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("PostgreSQL text cannot contain a NUL byte")
	}
}
