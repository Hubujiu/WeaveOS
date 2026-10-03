package appstructure

import (
	"context"
	"encoding/json"
	auditmaintenance "github.com/Hubujiu/WeaveOS/services/bff/internal/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func physicalSuffix(id string) string { return strings.ReplaceAll(id, "-", "") }

func TestAllCommonKindsProduceTypedPGColumnsAndConstantDefaults(t *testing.T) {
	f := setup(t)
	f.service.Application.References = CurrentSources{}
	view, table := newForm(t, f)
	var department string
	if e := f.owner.QueryRow(context.Background(), "SELECT id::text FROM personnel.departments WHERE is_root LIMIT 1").Scan(&department); e != nil {
		t.Fatal(e)
	}
	option := uuid(t, f.owner)
	cases := []struct {
		kind               string
		value, config      any
		physical, expected string
	}{
		{"text", "literal text", map[string]any{"maxLength": nil}, "text", `"literal text"`},
		{"multiline", "line1\nline2", map[string]any{"maxLength": nil}, "text", `"line1\nline2"`},
		{"number", "123456789012345678901234567890.5", map[string]any{}, "numeric(38,0)", `"123456789012345678901234567891"`},
		{"money", "-1.005", map[string]any{"roundingMode": "HALF_EVEN"}, "numeric(38,2)", `"-1.00"`},
		{"date", "2000-02-29", map[string]any{}, "date", `"2000-02-29"`},
		{"datetime", "1969-12-31T23:59:59.999999Z", map[string]any{"precision": "minute"}, "timestamp with time zone", `"1969-12-31T23:59:00+00:00"`},
		{"single_select", option, map[string]any{"options": []any{map[string]any{"id": option, "label": "one"}}}, "uuid", `"` + option + `"`},
		{"multi_select", []any{option, option}, map[string]any{"options": []any{map[string]any{"id": option, "label": "one"}}}, "uuid[]", `["` + option + `"]`},
		{"boolean", false, map[string]any{}, "boolean", `false`},
		{"member", f.actor, map[string]any{}, "uuid", `"` + f.actor + `"`},
		{"department", department, map[string]any{}, "uuid", `"` + department + `"`},
	}
	fields := []map[string]any{}
	for _, test := range cases {
		fields = append(fields, field(t, f, test.kind, test.value, test.config))
	}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, fields...)), 200)
	insertRow(t, f, table, nil)
	if _, e := f.owner.Exec(context.Background(), "SET TIME ZONE 'UTC'"); e != nil {
		t.Fatal(e)
	}
	for i, test := range cases {
		id := fields[i]["id"].(string)
		var physical string
		if e := f.owner.QueryRow(context.Background(), "SELECT format_type(atttypid,atttypmod) FROM pg_attribute WHERE attrelid=to_regclass($1) AND attname=$2", "appdata.t_"+physicalSuffix(table), "f_"+physicalSuffix(id)).Scan(&physical); e != nil || physical != test.physical {
			t.Fatalf("%s native type: %q %v", test.kind, physical, e)
		}
		expression := columnPhysical(id)
		if test.kind == "number" || test.kind == "money" {
			expression += "::text"
		}
		var value []byte
		if e := f.owner.QueryRow(context.Background(), "SELECT to_jsonb("+expression+") FROM "+tablePhysical(table)).Scan(&value); e != nil || !jsonEqual(json.RawMessage(value), json.RawMessage(test.expected)) {
			t.Fatalf("%s independent exact default: %s want %s %v", test.kind, value, test.expected, e)
		}
	}
	var columns, jsonColumns int
	if e := f.owner.QueryRow(context.Background(), "SELECT count(*),count(*) FILTER(WHERE atttypid IN ('json'::regtype,'jsonb'::regtype)) FROM pg_attribute WHERE attrelid=to_regclass($1) AND attnum>0 AND NOT attisdropped", "appdata.t_"+physicalSuffix(table)).Scan(&columns, &jsonColumns); e != nil || columns != 16 || jsonColumns != 0 {
		t.Fatal("typed business rows plus five system columns", columns, jsonColumns, e)
	}
}

func TestNewStructureAuditArchivesWithActualMaintenanceRoles(t *testing.T) {
	f := setup(t)
	coldOwner, e := pgxpool.New(context.Background(), os.Getenv("WEAVEOS_TEST_ARCHIVE_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer coldOwner.Close()
	coldRoles, e := os.ReadFile("../../../../infra/runtime/cold-roles.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = coldOwner.Exec(context.Background(), string(coldRoles)); e != nil {
		t.Fatal(e)
	}
	view, _ := newForm(t, f)
	if _, e := f.owner.Exec(context.Background(), "UPDATE auth.authentication_events SET occurred_at=date_trunc('month',now())-interval '1 day' WHERE object_id=$1 AND event_type='application_structure_changed'", view); e != nil {
		t.Fatal(e)
	}
	rolePool := func(dsn string) *pgxpool.Pool {
		cfg, e := pgxpool.ParseConfig(dsn)
		if e != nil {
			t.Fatal(e)
		}
		cfg.AfterConnect = func(c context.Context, x *pgx.Conn) error { _, e := x.Exec(c, "SET ROLE auth_maintenance"); return e }
		pool, e := pgxpool.NewWithConfig(context.Background(), cfg)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	hot, cold := rolePool(os.Getenv("WEAVEOS_TEST_DATABASE_URL")), rolePool(os.Getenv("WEAVEOS_TEST_ARCHIVE_DATABASE_URL"))
	result, e := auditmaintenance.Maintain(context.Background(), hot, cold, time.Now())
	if e != nil || result.Archived < 1 {
		t.Fatal("cold4 recognizes new audit with actual role", result, e)
	}
	var exists bool
	if e := cold.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM archive.authentication_events WHERE object_id=$1 AND event_type='application_structure_changed' AND change_summary->>'appId'=$2)", view, f.app).Scan(&exists); e != nil || !exists {
		t.Fatal("new cold audit lost", exists, e)
	}
	if e := f.owner.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM auth.authentication_events WHERE object_id=$1 AND event_type='application_structure_changed')", view).Scan(&exists); e != nil || exists {
		t.Fatal("hot audit must move after durable cold commit", exists, e)
	}
}
