package personnel

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestQ36FilterTypedBoundsAndCanonical(t *testing.T) {
	valid := []string{
		`{"operator":"and","children":[{"field":"account","operator":"eq","value":"Alice"}]}`,
		`{"operator":"or","children":[{"field":"status","operator":"eq","value":"active"},{"operator":"and","children":[{"field":"personnelManage","operator":"neq","value":false}]}]}`,
		`{"operator":"and","children":[{"field":"identityIds","operator":"neq","value":"00000000-0000-4000-8000-000000000001"}]}`,
	}
	for _, raw := range valid {
		p, err := CompileFilter("members", json.RawMessage(raw), 3)
		if err != nil || !json.Valid(p.Canonical) || p.Predicate == "" {
			t.Fatalf("valid filter rejected: %v", err)
		}
	}
	leaf := `{"field":"account","operator":"eq","value":"Alice"}`
	group := func(child string) string { return `{"operator":"and","children":[` + child + `]}` }
	invalid := []string{`null`, `{}`, `{"operator":"and","children":[]}`, group(`{"field":"account","operator":"gt","value":"A"}`), group(`{"field":"status","operator":"eq","value":"root"}`), group(`{"field":"identityIds","operator":"neq","value":null}`), group(`{"field":"personnelManage","operator":"eq","value":"true"}`), group(`{"field":"account","operator":"eq","value":"a","value":"b"}`), group(`{"field":"account","operator":"eq","value":"a","secret":true}`), group(group(group(group(leaf)))), group(strings.TrimSuffix(strings.Repeat(leaf+",", 21), ",")), group(`{"field":"account","operator":"eq","value":"` + strings.Repeat("x", 16384) + `"}`)}
	for _, raw := range invalid {
		if _, err := CompileFilter("members", json.RawMessage(raw), 1); err == nil {
			t.Errorf("invalid filter accepted: %.120s", raw)
		}
	}
	a, _ := CompileFilter("members", json.RawMessage(valid[0]), 1)
	b, err := CompileFilter("members", json.RawMessage(` { "children" : [ {"value":"\u0041lice","operator":"eq","field":"account"} ],"operator":"and" } `), 1)
	if err != nil || string(a.Canonical) != string(b.Canonical) {
		t.Fatalf("equivalent JSON must normalize identically: %s / %s %v", a.Canonical, b.Canonical, err)
	}
}

func TestQ36FilterRealSQLNullRelationsAndInjection(t *testing.T) {
	ctx := context.Background()
	c, err := pgx.Connect(ctx, os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	const uid = "00000000-0000-4000-8000-000000000001"
	const rows = `WITH q(id,account,identity_ids) AS (VALUES (1,'Alice'::text,ARRAY['` + uid + `']::uuid[]),(2,'alice',ARRAY[]::uuid[]),(3,NULL,ARRAY[]::uuid[])) SELECT id FROM q WHERE `
	for _, tc := range []struct {
		field, op string
		value     any
		want      []int
	}{{"account", "eq", "Alice", []int{1}}, {"account", "neq", "Alice", []int{2}}, {"account", "eq", nil, []int{3}}, {"account", "neq", nil, []int{1, 2}}, {"identityIds", "neq", uid, []int{2, 3}}, {"account", "eq", "' OR TRUE --", []int{}}} {
		raw, _ := json.Marshal(map[string]any{"operator": "and", "children": []any{map[string]any{"field": tc.field, "operator": tc.op, "value": tc.value}}})
		p, err := CompileFilter("members", raw, 1)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		result, err := c.Query(ctx, rows+p.Predicate+" ORDER BY id", p.Arguments...)
		if err != nil {
			t.Fatal(err)
		}
		got := []int{}
		for result.Next() {
			var id int
			if err := result.Scan(&id); err != nil {
				t.Fatal(err)
			}
			got = append(got, id)
		}
		err = result.Err()
		result.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %s: %v want %v", tc.field, tc.op, got, tc.want)
		}
	}
}

func TestQ36FilterTimeDSTAndPrecision(t *testing.T) {
	p, err := CompileFilter("events", json.RawMessage(`{"operator":"and","children":[{"field":"occurredAt","operator":"eq","value":{"date":"2026-03-08","timeZone":"America/New_York"}}]}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Arguments) != 2 {
		t.Fatalf("day needs two absolute boundaries: %v", p.Arguments)
	}
	a, b := p.Arguments[0].(time.Time), p.Arguments[1].(time.Time)
	if a.Format(time.RFC3339) != "2026-03-08T05:00:00Z" || b.Sub(a) != 23*time.Hour {
		t.Fatalf("DST local day incorrect: %v %v", a, b)
	}
	for _, value := range []string{`"2026-01-01T00:00:00.1234567Z"`, `{"date":"2026-02-30","timeZone":"UTC"}`, `{"date":"2026-10-01","timeZone":"Not/AZone"}`} {
		if _, err := CompileFilter("events", json.RawMessage(`{"operator":"and","children":[{"field":"occurredAt","operator":"eq","value":`+value+`}]}`), 1); err == nil {
			t.Errorf("bad time accepted: %s", value)
		}
	}
}
