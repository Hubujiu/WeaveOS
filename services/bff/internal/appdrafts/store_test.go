package appdrafts

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
)

const actor = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const app = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const table = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
const view = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
const draftID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
const x = "11111111-1111-4111-8111-111111111111"
const y = "22222222-2222-4222-8222-222222222222"

func TestRealPGPartialDraftRevocationAndExactConsume(t *testing.T) {
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
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_drafts (
 id uuid PRIMARY KEY,owner_user_id uuid NOT NULL,app_id uuid NOT NULL,table_id uuid NOT NULL,view_id uuid NOT NULL,
 target_record_id uuid, schema_version bigint NOT NULL,base_record_version bigint,draft_version bigint NOT NULL,
 values_json jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now())`)
	if err != nil {
		t.Fatal(err)
	}
	s := Store{Relation: pgx.Identifier{"pg_temp", "v015_drafts"}}
	access := Access{ActorID: actor, AppID: app, TableID: table, ViewID: view, ResourceAllowed: true, CurrentSchemaVersion: 1,
		Field: func(id string) FieldStatus {
			return FieldStatus{Exists: id == x || id == y, Writable: id == x || id == y, Kind: "text", MaxRunes: 256}
		}}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.CreateInTx(ctx, tx, access, Create{ID: draftID, SchemaVersion: 1, Values: Values{x: "1.", y: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.DraftVersion != 1 || d.Values[x] != "1." {
		t.Fatalf("incomplete input changed: %+v", d)
	}
	if _, err := s.UpdateInTx(ctx, tx, access, draftID, Update{ExpectedDraftVersion: 1, Changes: Values{x: true}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("text field accepted boolean shape: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	access.Field = func(id string) FieldStatus {
		return FieldStatus{Exists: id == x || id == y, Writable: id == x, Kind: "text", MaxRunes: 256}
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	d, err = s.GetInTx(ctx, tx, access, draftID)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := d.Values[y]; leaked || len(d.Conflicts) != 1 || d.Conflicts[0].Reason != "FIELD_PERMISSION_REVOKED" {
		t.Fatalf("revoked value leaked or conflict missing: %+v", d)
	}
	d, err = s.UpdateInTx(ctx, tx, access, draftID, Update{ExpectedDraftVersion: 1, Changes: Values{x: "2."}, RemoveFieldIDs: []string{y}})
	if err != nil {
		t.Fatal(err)
	}
	if d.DraftVersion != 2 || d.Values[x] != "2." || len(d.Conflicts) != 0 {
		t.Fatalf("patch did not preserve/remove correctly: %+v", d)
	}
	d, err = s.UpdateInTx(ctx, tx, access, draftID, Update{ExpectedDraftVersion: 2, Changes: Values{}, RemoveFieldIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if d.DraftVersion != 2 {
		t.Fatal("empty patch bumped version")
	}
	if err := s.ConsumeInTx(ctx, tx, access, draftID, 1, nil, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale consume %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeInTx(ctx, tx, access, draftID, 2, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetInTx(ctx, tx, access, draftID); err != nil {
		t.Fatalf("rollback lost draft: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRealPGOwnerCursorDraftList(t *testing.T) {
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
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_drafts (id uuid PRIMARY KEY,owner_user_id uuid NOT NULL,app_id uuid NOT NULL,table_id uuid NOT NULL,view_id uuid NOT NULL,target_record_id uuid,schema_version bigint NOT NULL,base_record_version bigint,draft_version bigint NOT NULL,values_json jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now())`)
	if err != nil {
		t.Fatal(err)
	}
	s := Store{Relation: pgx.Identifier{"pg_temp", "v015_drafts"}}
	a := Access{ActorID: actor, AppID: app, TableID: table, ViewID: view, ResourceAllowed: true, CurrentSchemaVersion: 1, Field: func(id string) FieldStatus {
		return FieldStatus{Exists: id == x, Writable: id == x, Kind: "text", MaxRunes: 256}
	}}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"11111111-1111-4111-8111-111111111112", "11111111-1111-4111-8111-111111111113", "11111111-1111-4111-8111-111111111114"} {
		if _, err := s.CreateInTx(ctx, tx, a, Create{ID: id, SchemaVersion: 1, Values: Values{x: "incomplete"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.ListInTx(ctx, tx, a, nil, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.NextToken == "" {
		t.Fatalf("first page %+v", first)
	}
	second, err := s.ListInTx(ctx, tx, a, nil, 2, first.NextToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.NextToken != "" {
		t.Fatalf("second page %+v", second)
	}
	// All three rows were created in one transaction, so updated_at is tied.
	// The documented descending UUID tie-break determines the exact two pages.
	gotIDs := []string{first.Items[0].ID, first.Items[1].ID, second.Items[0].ID}
	wantIDs := []string{"11111111-1111-4111-8111-111111111114", "11111111-1111-4111-8111-111111111113", "11111111-1111-4111-8111-111111111112"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("complete cursor sequence got %v, want %v", gotIDs, wantIDs)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
