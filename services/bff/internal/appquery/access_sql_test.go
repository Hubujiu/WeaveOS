package appquery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/jackc/pgx/v5"
)

func TestRealPGRowAndFieldMaskBeforeProjection(t *testing.T) {
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_masks(id uuid PRIMARY KEY,created_by uuid NOT NULL,f_aaaaaaaaaaaa4aaa8aaaaaaaaaaaaaaa text,f_bbbbbbbbbbbb4bbb8bbbbbbbbbbbbbbb numeric)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO v015_masks VALUES
('11111111-1111-4111-8111-111111111111','aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','public-own',42),
('22222222-2222-4222-8222-222222222222','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','public-other',99)`)
	if err != nil {
		t.Fatal(err)
	}
	p := appaccess.Policy{ActorID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", OwnerID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", ResourceExists: true, Grants: []appaccess.Grant{
		{AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Action: appaccess.Read, Scope: appaccess.All, Fields: []string{fieldX}},
		{AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Action: appaccess.Read, Scope: appaccess.Own, Fields: []string{fieldN}},
	}}
	query := func(p appaccess.Policy) map[string]map[string]any {
		fragments, e := CompileAccess(p, fixtureFields[:2], 1)
		if e != nil {
			t.Fatal(e)
		}
		rows, e := db.Query(ctx, `SELECT r.id::text,`+fragments.Projection+` FROM v015_masks r WHERE `+fragments.RowPredicate+` ORDER BY r.id`, fragments.Arguments...)
		if e != nil {
			t.Fatal(e)
		}
		defer rows.Close()
		out := map[string]map[string]any{}
		for rows.Next() {
			var id string
			var raw []byte
			if e = rows.Scan(&id, &raw); e != nil {
				t.Fatal(e)
			}
			var v map[string]any
			if e = json.Unmarshal(raw, &v); e != nil {
				t.Fatal(e)
			}
			out[id] = v
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		return out
	}
	all := query(p)
	if len(all) != 2 {
		t.Fatalf("all scope rows %v", all)
	}
	if all["11111111-1111-4111-8111-111111111111"][fieldN] != "42" {
		t.Fatalf("own numeric not serialized as wire string: %v", all)
	}
	if _, leak := all["22222222-2222-4222-8222-222222222222"][fieldN]; leak {
		t.Fatalf("secret own field leaked to other row: %v", all)
	}
	search, e := CompileSearch(p, fixtureFields[:2], json.RawMessage(`{"operator":"and","children":[{"fieldId":"`+fieldX+`","operator":"eq","value":"public-other"}]}`), nil, 1)
	if e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM v015_masks r WHERE `+search.Access.RowPredicate+` AND `+search.CountFilter.Predicate, search.CountArguments...).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("authorized filter count=%d", count)
	}
	p.Grants = p.Grants[1:]
	own := query(p)
	if len(own) != 1 {
		t.Fatalf("own scope count leaked rows: %v", own)
	}
	ownSearch, e := CompileSearch(p, fixtureFields[:2], json.RawMessage(`{"operator":"and","children":[{"fieldId":"`+fieldN+`","operator":"gt","value":"10"}]}`), nil, 1)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(ctx, `SELECT count(*) FROM v015_masks r WHERE `+ownSearch.Access.RowPredicate+` AND `+ownSearch.CountFilter.Predicate, ownSearch.CountArguments...).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("own filter count=%d", count)
	}
}

func TestCompileSearchRejectsSecretFilterBeforeSQL(t *testing.T) {
	p := appaccess.Policy{ActorID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", OwnerID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", ResourceExists: true, Grants: []appaccess.Grant{
		{AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Action: appaccess.Read, Scope: appaccess.All, Fields: []string{fieldX}},
		{AppID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ViewID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Action: appaccess.Read, Scope: appaccess.Own, Fields: []string{fieldN}},
	}}
	secret := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + fieldN + `","operator":"gt","value":"1"}]}`)
	if _, err := CompileSearch(p, fixtureFields[:2], secret, nil, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("secret predicate accepted: %v", err)
	}
	if _, err := CompileSearch(p, fixtureFields[:2], nil, json.RawMessage(`{"fieldId":"`+fieldN+`","direction":"asc"}`), 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("secret sort accepted: %v", err)
	}
	public := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + fieldX + `","operator":"eq","value":"a"}]}`)
	plan, err := CompileSearch(p, fixtureFields[:2], public, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Arguments) != 2 || plan.Arguments[0] != p.ActorID || plan.Arguments[1] != "a" {
		t.Fatalf("bind alignment wrong: %+v", plan)
	}
}
