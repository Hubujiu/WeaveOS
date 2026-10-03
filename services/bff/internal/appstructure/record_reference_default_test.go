package appstructure

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

// Frozen new-reference rule applies to actual INSERT defaults, including a
// defaults-only create mask. Retaining an existing reference is separate.
func TestTypedDefaultsOnlyCreateRejectsInactiveReferencesAtomically(t *testing.T) {
	for _, kind := range []string{"member", "department"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			f.service.Application.References = CurrentSources{}
			c := context.Background()
			var source string
			if kind == "member" {
				if e := f.owner.QueryRow(c, "INSERT INTO auth.users(account) VALUES('reference-default-'||gen_random_uuid()) RETURNING id::text").Scan(&source); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := f.owner.QueryRow(c, "INSERT INTO personnel.departments(parent_id,name) SELECT id,'reference default' FROM personnel.departments WHERE is_root RETURNING id::text").Scan(&source); e != nil {
					t.Fatal(e)
				}
			}
			view, table := newForm(t, f)
			ref := field(t, f, kind, source, map[string]any{})
			data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, ref)), 200)
			refID := ref["id"].(string)
			port := RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true, ActiveFieldIDs: []string{refID}}
			dml := RecordHistoryDML{History: RecordHistoryStore{}, Origin: "ordinary"}
			w := historyCommand(t, f, view, "record.create")
			op := recordOpID(t, w)
			id := uuid(t, f.owner)
			header, e := dml.Insert(c, w.Tx(), port, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{}}, []string{})
			if e != nil {
				t.Fatal("active omitted reference default must create", e)
			}
			historyComplete(t, w, header, op, 201)
			if e = w.Commit(c); e != nil {
				t.Fatal(e)
			}
			if kind == "member" {
				_, e = f.owner.Exec(c, "UPDATE auth.users SET status='disabled' WHERE id=$1", source)
			} else {
				_, e = f.owner.Exec(c, "DELETE FROM personnel.departments WHERE id=$1", source)
			}
			if e != nil {
				t.Fatal(e)
			}
			w = historyCommand(t, f, view, "record.create")
			op = recordOpID(t, w)
			rejectedID := uuid(t, f.owner)
			_, e = dml.Insert(c, w.Tx(), port, RecordCreate{OperationID: op, ID: rejectedID, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{}}, []string{})
			if !errors.Is(e, applications.ErrResourceInvalid) {
				w.Rollback(c)
				t.Fatalf("inactive omitted %s default must fail before durable insertion: %v", kind, e)
			}
			w.Rollback(c)
			for _, query := range []string{"SELECT count(*) FROM " + tablePhysical(table) + " WHERE id=$1", "SELECT count(*) FROM applications.record_change_events WHERE record_id=$1", "SELECT count(*) FROM applications.record_write_audit WHERE record_id=$1"} {
				var count int
				if e = f.owner.QueryRow(c, query, rejectedID).Scan(&count); e != nil || count != 0 {
					t.Fatal("rejected create left storage", query, count, e)
				}
			}
			var count int
			if e = f.owner.QueryRow(c, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&count); e != nil || count != 0 {
				t.Fatal("rejected create left operation", count, e)
			}
			// A stable old reference may remain on an already-created row.
			w = historyCommand(t, f, view, "record.edit")
			op = recordOpID(t, w)
			header, e = dml.UpdateCAS(c, w.Tx(), port, RecordEdit{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{refID: source}}, []string{refID})
			if e != nil {
				t.Fatal("same old inactive reference must retain", e)
			}
			historyComplete(t, w, header, op, 200)
			if e = w.Commit(c); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestNativeTypedCapabilityRejectsInactiveReferenceDefault(t *testing.T) {
	f := setup(t)
	f.service.Application.References = CurrentSources{}
	c := context.Background()
	var source string
	if e := f.owner.QueryRow(c, "INSERT INTO auth.users(account) VALUES('native-default-'||gen_random_uuid()) RETURNING id::text").Scan(&source); e != nil {
		t.Fatal(e)
	}
	view, table := newForm(t, f)
	ref := field(t, f, "member", source, map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, ref)), 200)
	if _, e := f.owner.Exec(c, "UPDATE auth.users SET status='disabled' WHERE id=$1", source); e != nil {
		t.Fatal(e)
	}
	var raw []byte
	e := f.runtime.QueryRow(c, "SELECT applications.apply_record_change($1,$2,$3,$4,$5,'insert',1,NULL,'{}'::jsonb)", f.app, view, table, uuid(t, f.owner), f.actor).Scan(&raw)
	var state *pgconn.PgError
	if !errors.As(e, &state) || state.Code != "23514" {
		t.Fatal("finite capability must reject implicit inactive references", e)
	}
}

func TestNativeReferenceGuardRequiresRealRegistryAndCounter(t *testing.T) {
	for _, missing := range []string{"registry", "counter"} {
		t.Run(missing, func(t *testing.T) {
			f := setup(t)
			f.service.Application.References = CurrentSources{}
			c := context.Background()
			var source string
			if e := f.owner.QueryRow(c, "INSERT INTO auth.users(account) VALUES('unavailable-default-'||gen_random_uuid()) RETURNING id::text").Scan(&source); e != nil {
				t.Fatal(e)
			}
			view, table := newForm(t, f)
			ref := field(t, f, "member", source, map[string]any{})
			data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, ref)), 200)
			tx, e := f.owner.Begin(c)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(c)
			if missing == "registry" {
				_, e = tx.Exec(c, "DELETE FROM applications.member_sources WHERE id=$1", source)
			} else {
				_, e = tx.Exec(c, "DELETE FROM applications.reference_source_revision WHERE singleton")
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = tx.Exec(c, "SET LOCAL ROLE auth_app"); e != nil {
				t.Fatal(e)
			}
			var raw []byte
			e = tx.QueryRow(c, "SELECT applications.apply_record_change($1,$2,$3,$4,$5,'insert',1,NULL,'{}'::jsonb)", f.app, view, table, uuid(t, f.owner), f.actor).Scan(&raw)
			var state *pgconn.PgError
			if !errors.As(e, &state) || state.Code != "W0004" {
				t.Fatal("missing source must be explicitly unavailable", missing, e)
			}
		})
	}
}

func TestNativeGuardValidatesActualPostgreSQLReferenceDefault(t *testing.T) {
	f := setup(t)
	f.service.Application.References = CurrentSources{}
	c := context.Background()
	view, table := newForm(t, f)
	ref := field(t, f, "member", f.actor, map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, ref)), 200)
	var inactive string
	if e := f.owner.QueryRow(c, "INSERT INTO auth.users(account,status) VALUES('physical-default-'||gen_random_uuid(),'disabled') RETURNING id::text").Scan(&inactive); e != nil {
		t.Fatal(e)
	}
	// Owned fault injection: metadata still advertises an active default while
	// the physical constant is inactive. The runtime role cannot alter this.
	if _, e := f.owner.Exec(c, "ALTER TABLE "+tablePhysical(table)+" ALTER COLUMN "+columnPhysical(ref["id"].(string))+" SET DEFAULT '"+inactive+"'::uuid"); e != nil {
		t.Fatal(e)
	}
	id := uuid(t, f.owner)
	var raw []byte
	e := f.runtime.QueryRow(c, "SELECT applications.apply_record_change($1,$2,$3,$4,$5,'insert',1,NULL,'{}'::jsonb)", f.app, view, table, id, f.actor).Scan(&raw)
	var state *pgconn.PgError
	if !errors.As(e, &state) || state.Code != "23514" {
		t.Fatal("actual PostgreSQL default must be validated", e)
	}
	var count int
	if e = f.owner.QueryRow(c, "SELECT count(*) FROM "+tablePhysical(table)+" WHERE id=$1", id).Scan(&count); e != nil || count != 0 {
		t.Fatal("failed physical default left row", count, e)
	}
}

func TestNativeNullableReferenceWithoutConstantCreatesNull(t *testing.T) {
	f := setup(t)
	c := context.Background()
	view, table := newForm(t, f)
	ref := field(t, f, "member", nil, map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, ref)), 200)
	if _, e := f.owner.Exec(c, "UPDATE applications.fields SET definition=definition-'default' WHERE app_id=$1 AND table_id=$2 AND id=$3", f.app, table, ref["id"]); e != nil {
		t.Fatal(e)
	}
	id := uuid(t, f.owner)
	var raw []byte
	if e := f.runtime.QueryRow(c, "SELECT applications.apply_record_change($1,$2,$3,$4,$5,'insert',1,NULL,'{}'::jsonb)", f.app, view, table, id, f.actor).Scan(&raw); e != nil {
		t.Fatal("nullable physical absence is not a new reference", e)
	}
	var isNull bool
	if e := f.runtime.QueryRow(c, "SELECT "+columnPhysical(ref["id"].(string))+" IS NULL FROM "+tablePhysical(table)+" WHERE id=$1", id).Scan(&isNull); e != nil || !isNull {
		t.Fatal(isNull, e)
	}
}
