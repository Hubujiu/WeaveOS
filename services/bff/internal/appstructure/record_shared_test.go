package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestNonManagerRecordTransactionResolvesActualResourcesAndSession(t *testing.T) {
	owner, ordinary := setup(t), setup(t)
	view, table := newForm(t, owner)
	opts := applications.RecordWriteOptions{LockTimeout: time.Second, StatementTimeout: 5 * time.Second, OperationID: uuid(t, owner.owner), Kind: "record.create", Authorize: func(context.Context, pgx.Tx, applications.RecordContext) error { return applications.ErrDenied }}
	app := &applications.Application{Pool: ordinary.runtime}
	p := session.Principal{UserID: ordinary.actor, Record: session.Record{AuthVersion: "1"}}
	write, e := app.BeginRecordWrite(context.Background(), p, owner.app, view, opts)
	if e != nil {
		t.Fatalf("trusted ordinary Session must resolve real resource without manager gate: %v", e)
	}
	defer write.Rollback(context.Background())
	facts := write.Context()
	if facts.Actor.ID != ordinary.actor || facts.App.ID != owner.app || facts.TableID != table || facts.ViewID != view || facts.App.OwnerUserID != owner.actor {
		t.Fatalf("trusted current facts: %+v", facts)
	}
	if e := write.Claim(context.Background(), opts.OperationID, opts.Kind, opts.Fingerprint); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("disconnected ordinary action must deny before operation claim: %v", e)
	}
	write.Rollback(context.Background())
	p.Record.AuthVersion = "2"
	if write, e := app.BeginRecordWrite(context.Background(), p, owner.app, view, opts); !errors.Is(e, session.ErrUnauthorized) {
		if write != nil {
			write.Rollback(context.Background())
		}
		t.Fatalf("live Session version must be rechecked: %v", e)
	}
	p.Record.AuthVersion = "1"
	if write, e := app.BeginRecordWrite(context.Background(), p, ordinary.app, view, opts); !errors.Is(e, applications.ErrMissing) {
		if write != nil {
			write.Rollback(context.Background())
		}
		t.Fatalf("cross-app form must not resolve: %v", e)
	}
}

func TestDataGrantReplacementPersistsExactMasksAndRegisteredFormMenu(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	number := field(t, f, "money", nil, map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, number)), 200)
	base := "/api/v1/applications/" + f.app + "/permission-groups"
	group := data(t, rootCall(t, f, "POST", base, map[string]any{"name": "runtime group", "operationId": uuid(t, f.owner), "expectedPolicyRevision": 1}, f.actor), 201)["id"].(string)
	grants := []any{
		map[string]any{"resourceKind": "form", "resourceId": view, "action": "menu.enter", "rowScope": "all", "fields": []string{}},
		map[string]any{"resourceKind": "form", "resourceId": view, "action": "data.create", "rowScope": "all", "fields": []string{}},
		map[string]any{"resourceKind": "form", "resourceId": view, "action": "data.read", "rowScope": "own", "fields": []string{number["id"].(string)}},
		map[string]any{"resourceKind": "form", "resourceId": view, "action": "data.history", "rowScope": "own", "fields": []string{number["id"].(string)}},
	}
	data(t, rootCall(t, f, "PUT", base+"/"+group+"/grants", map[string]any{"grants": grants, "operationId": uuid(t, f.owner), "expectedPolicyRevision": 2}, f.actor), 200)
	stored := data(t, rootCall(t, f, "GET", base+"/"+group+"/grants", nil, f.actor), 200)["grants"].([]any)
	canonical := func(values []any) []string {
		t.Helper()
		result := make([]string, len(values))
		for i, value := range values {
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			result[i] = string(raw)
		}
		sort.Strings(result)
		return result
	}
	if got, want := canonical(stored), canonical(grants); !reflect.DeepEqual(got, want) {
		t.Fatalf("complete resource/action/row scope/field grants got %v, want %v", got, want)
	}

	foreign := map[string]any{"resourceKind": "form", "resourceId": view, "action": "data.edit", "rowScope": "all", "fields": []string{uuid(t, f.owner)}}
	w := rootCall(t, f, "PUT", base+"/"+group+"/grants", map[string]any{"grants": []any{foreign}, "operationId": uuid(t, f.owner), "expectedPolicyRevision": 3}, f.actor)
	if w.Code != 400 {
		t.Fatalf("unknown/cross-table field grant must reject %d %s", w.Code, w.Body.String())
	}
}

func TestRecordSourcesPreserveMultiDepartmentEdgesAndDeletedIdentity(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	var member, label, root string
	if e := f.owner.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('source-registry-'||gen_random_uuid()) RETURNING id::text,account").Scan(&member, &label); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(ctx, "SELECT id::text FROM personnel.departments WHERE is_root").Scan(&root); e != nil {
		t.Fatal(e)
	}
	depts := []string{}
	for i := 0; i < 2; i++ {
		var dept string
		if e := f.owner.QueryRow(ctx, "INSERT INTO personnel.departments(parent_id,name) VALUES($1,'source department') RETURNING id::text", root).Scan(&dept); e != nil {
			t.Fatal(e)
		}
		depts = append(depts, dept)
		if _, e := f.owner.Exec(ctx, "INSERT INTO personnel.department_members(user_id,department_id) VALUES($1,$2)", member, dept); e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if e := f.runtime.QueryRow(ctx, "SELECT count(*) FROM applications.member_department_sources WHERE member_id=$1", member).Scan(&count); e != nil || count != 2 {
		t.Fatal("actual multiple department edges required", count, e)
	}
	var before, after int64
	if e := f.runtime.QueryRow(ctx, "SELECT revision FROM applications.reference_source_revision WHERE singleton").Scan(&before); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", member); e != nil {
		t.Fatal(e)
	}
	if e := f.runtime.QueryRow(ctx, "SELECT revision FROM applications.reference_source_revision WHERE singleton").Scan(&after); e != nil || after != before {
		t.Fatal("non-display auth version change must not change source projection", before, after, e)
	}
	if _, e := f.owner.Exec(ctx, "DELETE FROM personnel.department_members WHERE user_id=$1", member); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(ctx, "DELETE FROM auth.users WHERE id=$1", member); e != nil {
		t.Fatal(e)
	}
	var preserved, status string
	if e := f.runtime.QueryRow(ctx, "SELECT label,status FROM applications.member_sources WHERE id=$1", member).Scan(&preserved, &status); e != nil || preserved != label || status != "deleted" {
		t.Fatal("deleted source loses stable ID/last label", preserved, status, e)
	}
	if _, e := f.owner.Exec(ctx, "INSERT INTO auth.users(id,account) VALUES($1,$2)", member, label); e == nil {
		t.Fatal("deleted source UUID reused")
	}
	for _, dept := range depts {
		if _, e := f.owner.Exec(ctx, "DELETE FROM personnel.departments WHERE id=$1", dept); e != nil {
			t.Fatal(e)
		}
	}
	if e := f.runtime.QueryRow(ctx, "SELECT count(*) FROM applications.department_sources WHERE id=ANY($1::uuid[]) AND status='deleted'", depts).Scan(&count); e != nil || count != 2 {
		t.Fatal("department tombstones required", count, e)
	}
}

func TestNativeRecordCapabilityRejectsNonCanonicalAndSystemKeys(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	money := field(t, f, "money", nil, map[string]any{})
	stamp := field(t, f, "datetime", nil, map[string]any{"precision": "minute"})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, money, stamp)), 200)
	bad := []map[string]any{
		{"created_by": f.actor}, {"record_version": "99"}, {money["id"].(string): 1.2}, {money["id"].(string): "1.235"}, {money["id"].(string): "-0.00"},
		{stamp["id"].(string): "2026-10-03T00:00:01Z"}, {uuid(t, f.owner): "secret"},
	}
	for _, values := range bad {
		raw, e := json.Marshal(values)
		if e != nil {
			t.Fatal(e)
		}
		var out []byte
		e = f.runtime.QueryRow(context.Background(), "SELECT applications.apply_record_change($1,$2,$3,$4,$5,'insert',1,NULL,$6::jsonb)", f.app, view, table, uuid(t, f.owner), f.actor, raw).Scan(&out)
		var pgerr *pgconn.PgError
		if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("finite DB capability must reject noncanonical/system keys with validation, got %v", e)
		}
	}
}

func TestNativeRecordCapabilityRequiresFiniteOperationAndSchemaCAS(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0)), 200)
	for _, args := range []struct {
		operation any
		schema    any
	}{{nil, int64(1)}, {"insert", nil}} {
		var out []byte
		e := f.runtime.QueryRow(context.Background(), "SELECT applications.apply_record_change($1,$2,$3,$4,$5,$6,$7,1,'{}'::jsonb)", f.app, view, table, uuid(t, f.owner), f.actor, args.operation, args.schema).Scan(&out)
		var pgerr *pgconn.PgError
		if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("nullable operation/schema must not bypass finite CAS capability: %v", e)
		}
	}
}

func TestRecordSharedMigrationRelationsAreReal(t *testing.T) {
	f := setup(t)
	for _, name := range []string{"record_drafts", "record_write_audit", "record_command_fences", "member_sources", "department_sources", "member_department_sources", "reference_source_revision"} {
		var exists bool
		if e := f.owner.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", "applications."+name).Scan(&exists); e != nil {
			t.Fatal(e)
		}
		if !exists {
			t.Errorf("frozen shared relation %s is absent", name)
		}
	}
}

func TestControlledRecordDMLWritesNativeValuesAndProtectsSystemAndCAS(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	money := field(t, f, "money", nil, map[string]any{})
	text := field(t, f, "text", "default literal", map[string]any{"maxLength": 20})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, money, text)), 200)
	resource := RecordTable{AppID: f.app, TableID: table, ViewID: view, Namespace: "appdata", SchemaVersion: 1, Ready: true, ActiveFieldIDs: []string{money["id"].(string), text["id"].(string)}}
	ctx := context.Background()
	tx, e := f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	id := uuid(t, f.owner)
	native := RecordDML{}
	header, e := native.Insert(ctx, tx, resource, RecordCreate{OperationID: uuid(t, f.owner), ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{money["id"].(string): "1.20"}}, []string{money["id"].(string)})
	if e != nil {
		tx.Rollback(ctx)
		var diagnostic []byte
		values, _ := json.Marshal(map[string]any{money["id"].(string): "1.20"})
		direct := f.runtime.QueryRow(ctx, "SELECT applications.apply_record_change($1,$2,$3,$4,$5,'insert',1,NULL,$6::jsonb)", f.app, view, table, id, f.actor, values).Scan(&diagnostic)
		t.Fatalf("real restricted native insert must work: %v; isolated direct diagnostic %v", e, direct)
	}
	if header.ID != id || header.CreatedBy != f.actor || header.RecordVersion != 1 || header.CreatedAt.IsZero() || !header.CreatedAt.Equal(header.UpdatedAt) {
		t.Fatalf("protected create header %+v", header)
	}
	var amount, label string
	if e := tx.QueryRow(ctx, "SELECT "+columnPhysical(money["id"].(string))+"::text,"+columnPhysical(text["id"].(string))+" FROM "+tablePhysical(table)+" WHERE id=$1", id).Scan(&amount, &label); e != nil || amount != "1.20" || label != "default literal" {
		t.Fatal("typed values/defaults", amount, label, e)
	}
	next, e := native.UpdateCAS(ctx, tx, resource, RecordEdit{OperationID: uuid(t, f.owner), ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{money["id"].(string): "-2.50"}}, []string{money["id"].(string)})
	if e != nil || next.RecordVersion != 2 || next.CreatedBy != f.actor || !next.CreatedAt.Equal(header.CreatedAt) {
		t.Fatal("native update must preserve creator and increment CAS", next, e)
	}
	locked, e := native.LockHeader(ctx, tx, resource, id)
	if e != nil || locked.RecordVersion != 2 {
		t.Fatal("controlled row lock must work without broad UPDATE grant", locked, e)
	}
	if _, e = native.UpdateCAS(ctx, tx, resource, RecordEdit{OperationID: uuid(t, f.owner), ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{}}, []string{}); e == nil {
		t.Fatal("stale CAS must reject")
	}
}
