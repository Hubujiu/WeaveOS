package appquery

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const fieldX = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const fieldN = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const fieldM = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

var fixtureFields = []Field{{fieldX, Text}, {fieldN, Number}, {fieldM, MultiSelect}}

func TestNestedTypedConditionsAndSetEqualityInRealPG(t *testing.T) {
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL 18 database required")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	filter := json.RawMessage(`{"operator":"and","children":[{"operator":"or","children":[{"fieldId":"` + fieldX + `","operator":"eq","value":"alpha"},{"fieldId":"` + fieldN + `","operator":"gt","value":"9.5"}]},{"fieldId":"` + fieldM + `","operator":"eq","value":["22222222-2222-4222-8222-222222222222","11111111-1111-4111-8111-111111111111","11111111-1111-4111-8111-111111111111"]}]}`)
	p, err := Compile(filter, json.RawMessage(`{"fieldId":"`+fieldN+`","direction":"asc"}`), fixtureFields, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.ReferencedFields, []string{fieldX, fieldN, fieldM}) || !strings.Contains(p.Order, "NULLS LAST") || !strings.Contains(p.Order, "id") {
		t.Fatalf("bad coverage/order: %+v", p)
	}
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_q (id uuid PRIMARY KEY, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, f_aaaaaaaaaaaa4aaa8aaaaaaaaaaaaaaa text, f_bbbbbbbbbbbb4bbb8bbbbbbbbbbbbbbb numeric, f_cccccccccccc4ccc8ccccccccccccccc uuid[])`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO v015_q VALUES
('11111111-1111-4111-8111-111111111111',now(),now(),'alpha',1,ARRAY['11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222']::uuid[]),
('22222222-2222-4222-8222-222222222222',now(),now(),'other',12,ARRAY['22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111']::uuid[]),
('33333333-3333-4333-8333-333333333333',now(),now(),'alpha',2,NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(ctx, `SELECT id::text FROM v015_q r WHERE `+p.Predicate+` ORDER BY `+p.Order, p.Arguments...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("SQL result %v want %v", ids, want)
	}
}

func TestCompilerRejectsWrongShapeAndUnsupportedSort(t *testing.T) {
	bad := []string{
		`{"operator":"and","children":[{"fieldId":"` + fieldX + `","operator":"gt","value":"x"}]}`,
		`{"operator":"and","operator":"or","children":[]}`,
		`{"operator":"and","children":[{"fieldId":"` + fieldN + `","operator":"eq","value":9.5}]}`,
		`{"operator":"and","children":[{"fieldId":"dddddddd-dddd-4ddd-8ddd-dddddddddddd","operator":"eq","value":"x"}]}`,
	}
	for _, raw := range bad {
		if _, err := Compile(json.RawMessage(raw), nil, fixtureFields, 1); err == nil {
			t.Errorf("accepted invalid %s", raw)
		}
	}
	if _, err := Compile(nil, json.RawMessage(`{"fieldId":"`+fieldX+`","direction":"asc"}`), fixtureFields, 1); err == nil {
		t.Error("text sort accepted")
	}
}
