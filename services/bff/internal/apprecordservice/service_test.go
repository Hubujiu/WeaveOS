package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type recordFixture struct {
	ctx                                                                               context.Context
	owner, runtime                                                                    *pgxpool.Pool
	actor, other, app, table, view, public, secret, reference, ownRecord, otherRecord string
	principal                                                                         session.Principal
	service                                                                           *Service
}

type failingAdvanceClient struct{ redis.UniversalClient }

func (f failingAdvanceClient) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	cmd.SetErr(errors.New("injected Redis Advance failure"))
	return cmd
}

func newRecordFixture(t *testing.T) recordFixture {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("migrated isolated PostgreSQL is required")
	}
	owner, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE auth_app")
		return err
	}
	runtime, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	opts, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	ids := make([]string, 10)
	for i := range ids {
		if err := owner.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	f := recordFixture{ctx: ctx, owner: owner, runtime: runtime, actor: ids[0], other: ids[1], app: ids[2], table: ids[3], view: ids[4], public: ids[5], secret: ids[6], reference: ids[9], ownRecord: ids[8], otherRecord: ids[7]}
	for _, u := range []string{f.actor, f.other} {
		if _, err := owner.Exec(ctx, "INSERT INTO auth.users(id,account) VALUES($1,$2)", u, "v015-consumer-"+u); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := owner.Exec(ctx, "INSERT INTO applications.apps(id,name,owner_user_id) VALUES($1,'consumer',$2)", f.app, f.other); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO applications.menu_resources VALUES($1,'application',$1)", f.app); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "SELECT applications.register_catalog_entry($1)", f.app); err != nil {
		t.Fatal(err)
	}
	fieldDefs := []map[string]any{
		{"id": f.public, "name": "Public", "kind": "text", "required": false, "config": map[string]any{}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}},
		{"id": f.secret, "name": "Secret", "kind": "text", "required": false, "config": map[string]any{}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}},
		{"id": f.reference, "name": "Member", "kind": "member", "required": false, "config": map[string]any{}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}},
	}
	fieldsJSON, _ := json.Marshal(fieldDefs)
	if _, err := owner.Exec(ctx, "INSERT INTO applications.logical_tables(id,app_id,name,position,schema_version,schema_ready,fields_json) VALUES($1,$2,'table',0,1,true,$3)", f.table, f.app, fieldsJSON); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO applications.form_views(id,app_id,table_id,name,position,view_version) VALUES($1,$2,$3,'form',0,1)", f.view, f.app, f.table); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO applications.menu_resources VALUES($1,'form',$2)", f.app, f.view); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "SELECT applications.apply_schema_change($1,$2,$3,'create_table',NULL,NULL)", f.other, f.app, f.table); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{f.public, f.secret, f.reference} {
		definition, _ := json.Marshal(fieldDefs[i])
		if _, err := owner.Exec(ctx, "INSERT INTO applications.fields(id,app_id,table_id,definition) VALUES($1,$2,$3,$4)", id, f.app, f.table, definition); err != nil {
			t.Fatal(err)
		}
		typeName := "text"
		if id == f.reference {
			typeName = "uuid"
		}
		physical, _ := json.Marshal(map[string]any{"ID": id, "Type": typeName, "Required": false})
		if _, err := owner.Exec(ctx, "SELECT applications.apply_schema_change($1,$2,$3,'add_column',NULL,$4)", f.other, f.app, f.table, physical); err != nil {
			t.Fatalf("field %d: %v", i, err)
		}
	}
	group := ids[7]
	if _, err := owner.Exec(ctx, "INSERT INTO applications.permission_groups(id,app_id,name) VALUES($1,$2,'readers')", group, f.app); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", f.app, group, f.actor); err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct {
		action, scope string
		fields        []string
	}{{"menu.enter", "all", nil}, {"data.read", "all", []string{f.public, f.reference}}, {"data.read", "own", []string{f.secret}}, {"data.create", "all", []string{f.public, f.reference}}, {"data.edit", "all", []string{f.public, f.reference}}} {
		var grant string
		if err := owner.QueryRow(ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,$4,$5) RETURNING id::text", f.app, group, f.view, g.action, g.scope).Scan(&grant); err != nil {
			t.Fatal(err)
		}
		for _, id := range g.fields {
			if _, err := owner.Exec(ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, id, f.table); err != nil {
				t.Fatal(err)
			}
		}
	}
	physical := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	cols := fmt.Sprintf("f_%s,f_%s,f_%s", strings.ReplaceAll(f.public, "-", ""), strings.ReplaceAll(f.secret, "-", ""), strings.ReplaceAll(f.reference, "-", ""))
	if _, err := owner.Exec(ctx, "INSERT INTO "+physical+"(id,created_by,"+cols+") VALUES($1,$2,'alpha','private alpha',$4),($3,$4,'beta','private beta',$4)", ids[8], f.actor, ids[7], f.other); err != nil {
		t.Fatal(err)
	}
	f.principal = session.Principal{UserID: f.actor, SessionRef: ids[8], Record: session.Record{AuthVersion: "1"}}
	f.service = New(runtime, client, "consumer-"+strings.ReplaceAll(f.app, "-", ""))
	f.service.Limits = appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}
	return f
}

func TestRestrictedRecordCreateReplaysMinimumAndUsesControlledDML(t *testing.T) {
	f := newRecordFixture(t)
	var operation string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&operation); err != nil {
		t.Fatal(err)
	}
	req := CreateRequest{AppID: f.app, ViewID: f.view, OperationID: operation, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "created"}}
	meta := applications.Metadata{RequestID: "v015-restricted-create"}
	got, err := f.service.Create(f.ctx, f.principal, req, meta)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.OperationID != operation || got.RecordVersion != 1 || got.SchemaVersion != 1 {
		t.Fatalf("result %+v", got)
	}
	replay, err := f.service.Create(f.ctx, f.principal, req, meta)
	if err != nil || replay != got {
		t.Fatalf("same key replay %+v %v", replay, err)
	}
	var count int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1 AND operation_id=$2", f.app, operation).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count %d %v", count, err)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM "+relation+" WHERE id=$1", got.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("typed row count %d %v", count, err)
	}
}

func TestRestrictedCreateNotReadyAfterLiveAuthorization(t *testing.T) {
	t.Run("authorized", func(t *testing.T) {
		f := newRecordFixture(t)
		if _, err := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET schema_ready=false WHERE app_id=$1 AND id=$2", f.app, f.table); err != nil {
			t.Fatal(err)
		}
		var op string
		if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
			t.Fatal(err)
		}
		_, err := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "pending"}}, applications.Metadata{RequestID: "v015-not-ready"})
		var code *appstructure.Error
		if !errors.As(err, &code) || code.Code != "APPLICATION_SCHEMA_NOT_READY" {
			t.Fatalf("authorized unsaved schema: %v", err)
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		f := newRecordFixture(t)
		if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.create')", f.app); err != nil {
			t.Fatal(err)
		}
		if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.create'", f.app); err != nil {
			t.Fatal(err)
		}
		if _, err := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET schema_ready=false WHERE app_id=$1 AND id=$2", f.app, f.table); err != nil {
			t.Fatal(err)
		}
		var op string
		if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
			t.Fatal(err)
		}
		_, err := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "pending"}}, applications.Metadata{RequestID: "v015-not-ready-denied"})
		if !errors.Is(err, applications.ErrDenied) {
			t.Fatalf("unauthorized schema status leaked: %v", err)
		}
	})
}

func TestRestrictedEditCASAndPendingFence(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	created, err := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "created"}}, applications.Metadata{RequestID: "v015-edit-create"})
	if err != nil {
		t.Fatal(err)
	}
	var editOp string
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&editOp); err != nil {
		t.Fatal(err)
	}
	req := EditRequest{AppID: f.app, ViewID: f.view, RecordID: created.ID, OperationID: editOp, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "edited"}}
	changed, err := f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-edit"})
	if err != nil || changed.RecordVersion != 2 || changed.ID != created.ID {
		t.Fatalf("edit %+v %v", changed, err)
	}
	var staleOp string
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&staleOp); err != nil {
		t.Fatal(err)
	}
	req.OperationID = staleOp
	if _, err = f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-stale"}); !errors.Is(err, apprecords.ErrConflict) {
		t.Fatalf("stale CAS: %v", err)
	}
	if _, err = f.owner.Exec(f.ctx, "INSERT INTO applications.record_command_fences(app_id,table_id,record_id,command_id,state,expected_record_version) VALUES($1,$2,$3,gen_random_uuid(),'pending',2)", f.app, f.table, created.ID); err != nil {
		t.Fatal(err)
	}
	var fencedOp string
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&fencedOp); err != nil {
		t.Fatal(err)
	}
	req.OperationID = fencedOp
	req.ExpectedRecordVersion = 2
	if _, err = f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-fenced"}); err == nil {
		t.Fatal("pending command fence bypassed")
	} else {
		var code *appstructure.Error
		if !errors.As(err, &code) || code.Code != "APPLICATION_RECORD_FENCED" {
			t.Fatalf("fence: %v", err)
		}
	}
}

func TestRestrictedConcurrentEditsCommitOneVersionAndAudit(t *testing.T) {
	f := newRecordFixture(t)
	var keys [2]string
	for i := range keys {
		if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&keys[i]); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := range keys {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: keys[i], ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: fmt.Sprintf("winner-%d", i)}}, applications.Metadata{RequestID: fmt.Sprintf("v015-concurrent-%d", i)})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, apprecords.ErrConflict):
			conflict++
		default:
			t.Fatalf("concurrent edit error: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("expected one success and one CAS conflict; got %d/%d", success, conflict)
	}
	var version int64
	var audits int
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	if err := f.runtime.QueryRow(f.ctx, "SELECT record_version FROM "+relation+" WHERE id=$1", f.ownRecord).Scan(&version); err != nil || version != 2 {
		t.Fatalf("version %d %v", version, err)
	}
	if err := f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1", f.app).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit count %d %v", audits, err)
	}
}

func TestRestrictedReferenceWriteRequiresActiveAuthoritativeSource(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	req := CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.reference: f.other}}
	got, err := f.service.Create(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-ref-active"})
	if err != nil || got.RecordVersion != 1 {
		t.Fatalf("active source rejected %+v %v", got, err)
	}
	if _, err = f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.other); err != nil {
		t.Fatal(err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	req.OperationID = op
	if _, err = f.service.Create(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-ref-disabled"}); err == nil {
		t.Fatal("disabled source accepted as new reference")
	}
}

func TestRestrictedOmittedReferenceDefaultMustStillBeActive(t *testing.T) {
	f := newRecordFixture(t)
	var source, op string
	for _, target := range []*string{&source, &op} {
		if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.owner.Exec(f.ctx, "INSERT INTO auth.users(id,account) VALUES($1,$2)", source, "default-source-"+source); err != nil {
		t.Fatal(err)
	}
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	physical, _ := json.Marshal(map[string]any{"ID": f.reference, "Type": "uuid", "Default": map[string]any{"Text": source}})
	if _, err = tx.Exec(f.ctx, "SELECT applications.apply_schema_change($1,$2,$3,'alter_default',NULL,$4::jsonb)", f.other, f.app, f.table, physical); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, "UPDATE applications.fields SET definition=jsonb_set(definition,'{default}',to_jsonb($2::text)) WHERE app_id=$1 AND id=$3", f.app, source, f.reference); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=jsonb_set(fields_json,'{2,default}',to_jsonb($3::text)),schema_version=2 WHERE app_id=$1 AND id=$2", f.app, f.table, source); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	// The actor can create a row but cannot explicitly write the reference
	// field; the server-side physical default must still be validated.
	if _, err = f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.create')", f.app, f.reference); err != nil {
		t.Fatal(err)
	}
	active, err := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 2, Values: map[string]any{}}, applications.Metadata{RequestID: "v015-active-default"})
	if err != nil || active.RecordVersion != 1 {
		t.Fatalf("active default failed %+v %v", active, err)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.reference, "-", "")}.Sanitize()
	var actual string
	if err = f.runtime.QueryRow(f.ctx, "SELECT "+column+"::text FROM "+relation+" WHERE id=$1", active.ID).Scan(&actual); err != nil || actual != source {
		t.Fatalf("physical default missing %s %v", actual, err)
	}
	if _, err = f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", source); err != nil {
		t.Fatal(err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	var before int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM "+relation).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 2, Values: map[string]any{}}, applications.Metadata{RequestID: "v015-disabled-default"}); !errors.Is(err, applications.ErrResourceInvalid) {
		t.Fatalf("disabled default inserted: %v", err)
	}
	var after, audit, confirmed int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM "+relation).Scan(&after); err != nil || after != before {
		t.Fatalf("failed default changed records %d/%d %v", before, after, err)
	}
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1", f.app).Scan(&audit); err != nil || audit != 1 {
		t.Fatalf("failed default wrote audit %d %v", audit, err)
	}
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2 AND result_json IS NOT NULL", f.actor, op).Scan(&confirmed); err != nil || confirmed != 0 {
		t.Fatalf("failed default confirmed operation %d %v", confirmed, err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	kept, err := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: active.ID, OperationID: op, ExpectedSchemaVersion: 2, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "updated"}}, applications.Metadata{RequestID: "v015-old-reference-kept"})
	if err != nil || kept.RecordVersion != 2 {
		t.Fatalf("unchanged old reference blocked %+v %v", kept, err)
	}
}

func TestRestrictedDefaultReferenceSerializesWithSourceDisable(t *testing.T) {
	f := newRecordFixture(t)
	var source, op string
	for _, target := range []*string{&source, &op} {
		if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.owner.Exec(f.ctx, "INSERT INTO auth.users(id,account) VALUES($1,$2)", source, "racing-default-"+source); err != nil {
		t.Fatal(err)
	}
	physical, _ := json.Marshal(map[string]any{"ID": f.reference, "Type": "uuid", "Default": map[string]any{"Text": source}})
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, "SELECT applications.apply_schema_change($1,$2,$3,'alter_default',NULL,$4::jsonb)", f.other, f.app, f.table, physical); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, "UPDATE applications.fields SET definition=jsonb_set(definition,'{default}',to_jsonb($2::text)) WHERE app_id=$1 AND id=$3", f.app, source, f.reference); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=jsonb_set(fields_json,'{2,default}',to_jsonb($3::text)),schema_version=2 WHERE app_id=$1 AND id=$2", f.app, f.table, source); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	// The real source trigger holds personnel revision locks until COMMIT.
	sourceTx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sourceTx.Rollback(f.ctx)
	if _, err = sourceTx.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", source); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, e := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 2, Values: map[string]any{}}, applications.Metadata{RequestID: "v015-source-default-race"})
		result <- e
	}()
	blocked := false
	for i := 0; i < 50; i++ {
		if err = f.owner.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%personnel.lock_query_revisions()%')`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("record write never waited for the source revision lock")
	}
	if err = sourceTx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; !errors.Is(err, applications.ErrResourceInvalid) {
		t.Fatalf("write used pre-disable reference default after source COMMIT: %v", err)
	}
	var count int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&count); err != nil || count != 0 {
		t.Fatalf("racing failed write claimed key %d %v", count, err)
	}
}

func TestRestrictedDraftCreateIsExplicitAndDoesNotMutateRecord(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	req := DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, SchemaVersion: 1, Values: map[string]any{f.public: "unfinished"}}
	draft, err := f.service.CreateDraft(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-draft"})
	if err != nil || draft.ID == "" || draft.DraftVersion != 1 || draft.Values[f.public] != "unfinished" {
		t.Fatalf("explicit draft %+v %v", draft, err)
	}
	var count int
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM "+relation).Scan(&count); err != nil || count != 2 {
		t.Fatalf("draft changed formal rows %d %v", count, err)
	}
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1", f.app).Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft wrote record audit %d %v", count, err)
	}
}

func TestRestrictedDraftSubmitConsumesWithRecordAndOperation(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, SchemaVersion: 1, Values: map[string]any{f.public: "ready"}}, applications.Metadata{RequestID: "v015-draft-submit"})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	created, err := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "ready"}, DraftRef: &DraftRef{ID: draft.ID, DraftVersion: draft.DraftVersion}}, applications.Metadata{RequestID: "v015-record-submit"})
	if err != nil || created.RecordVersion != 1 {
		t.Fatalf("submitted %+v %v", created, err)
	}
	var count int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_drafts WHERE id=$1", draft.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft not consumed atomically %d %v", count, err)
	}
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2 AND result_json IS NOT NULL", f.actor, op).Scan(&count); err != nil || count != 1 {
		t.Fatalf("operation not complete %d %v", count, err)
	}
}

func TestRestrictedDraftPatchNoopAndDiscardLedger(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, SchemaVersion: 1, Values: map[string]any{f.public: "unfinished"}}, applications.Metadata{RequestID: "v015-draft-patch"})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	updated, err := f.service.UpdateDraft(f.ctx, f.principal, DraftUpdateRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: op, ExpectedDraftVersion: 1, Changes: map[string]any{f.public: "ready"}}, applications.Metadata{RequestID: "v015-patch"})
	if err != nil || updated.DraftVersion != 2 || updated.Values[f.public] != "ready" {
		t.Fatalf("patch %+v %v", updated, err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	noop, err := f.service.UpdateDraft(f.ctx, f.principal, DraftUpdateRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: op, ExpectedDraftVersion: 2, Changes: map[string]any{}, RemoveFieldIDs: []string{}}, applications.Metadata{RequestID: "v015-noop"})
	if err != nil || noop.DraftVersion != 2 {
		t.Fatalf("no-op %+v %v", noop, err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	if err = f.service.DiscardDraft(f.ctx, f.principal, DraftDiscardRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: op, ExpectedDraftVersion: 2}, applications.Metadata{RequestID: "v015-discard"}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_drafts WHERE id=$1", draft.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft remains %d %v", count, err)
	}
	var status int
	if err = f.runtime.QueryRow(f.ctx, "SELECT http_status FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&status); err != nil || status != 204 {
		t.Fatalf("discard ledger %d %v", status, err)
	}
}

func TestRestrictedDraftExplicitRemovalAfterSchemaAndBaseConflict(t *testing.T) {
	f := newRecordFixture(t)
	op := func() string {
		t.Helper()
		var id string
		if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	base := int64(1)
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op(), SchemaVersion: 1, TargetRecordID: &f.ownRecord, BaseRecordVersion: &base, Values: map[string]any{f.reference: f.other, f.public: "safe"}}, applications.Metadata{RequestID: "v015-conflict-create"})
	if err != nil {
		t.Fatal(err)
	}
	// Fixture performs the physical drop and catalog update as one approved
	// schema-change transaction, leaving the draft at its original binding.
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2", f.app, f.reference); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(map[string]any{"ID": f.reference, "Type": "uuid"})
	if _, err = tx.Exec(f.ctx, "SELECT applications.apply_schema_change($1,$2,$3,'drop_column',$4::jsonb,NULL)", f.other, f.app, f.table, before); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, "UPDATE applications.fields SET removed=true WHERE app_id=$1 AND table_id=$2 AND id=$3", f.app, f.table, f.reference); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=fields_json - 2,schema_version=2 WHERE app_id=$1 AND id=$2", f.app, f.table); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	// The bound row has independently advanced, so both bindings conflict.
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	if _, err = f.owner.Exec(f.ctx, "UPDATE "+relation+" SET record_version=record_version+1 WHERE id=$1", f.ownRecord); err != nil {
		t.Fatal(err)
	}
	read, err := f.service.GetDraft(f.ctx, f.principal, f.app, f.view, draft.ID)
	if err != nil || !read.HasConflicts || read.SchemaVersion != 1 || read.BaseRecordVersion == nil || *read.BaseRecordVersion != 1 {
		t.Fatalf("old binding lost %+v %v", read, err)
	}
	wantConflict := map[string]bool{"SCHEMA_CHANGED": false, "BASE_RECORD_CHANGED": false, "FIELD_REMOVED": false}
	for _, conflict := range read.Conflicts {
		if _, ok := wantConflict[conflict.Reason]; ok {
			wantConflict[conflict.Reason] = true
		}
	}
	for reason, found := range wantConflict {
		if !found {
			t.Fatalf("missing %s conflict %+v", reason, read.Conflicts)
		}
	}
	if _, leaked := read.Values[f.reference]; leaked {
		t.Fatalf("removed value leaked %+v", read)
	}
	if _, err = f.service.UpdateDraft(f.ctx, f.principal, DraftUpdateRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: op(), ExpectedDraftVersion: 1, Changes: map[string]any{f.public: "new"}}, applications.Metadata{RequestID: "v015-conflict-change"}); !errors.Is(err, appdrafts.ErrBaseConflict) {
		t.Fatalf("stale binding accepted new value: %v", err)
	}
	key := op()
	cleaned, err := f.service.UpdateDraft(f.ctx, f.principal, DraftUpdateRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: key, ExpectedDraftVersion: 1, Changes: map[string]any{}, RemoveFieldIDs: []string{f.reference}}, applications.Metadata{RequestID: "v015-conflict-remove"})
	if err != nil || cleaned.DraftVersion != 2 || cleaned.SchemaVersion != 1 || !cleaned.HasConflicts || cleaned.BaseRecordVersion == nil || *cleaned.BaseRecordVersion != 1 {
		t.Fatalf("explicit cleanup %+v %v", cleaned, err)
	}
	if _, leaked := cleaned.Values[f.reference]; leaked {
		t.Fatalf("cleanup exposed removed value %+v", cleaned)
	}
	var payload []byte
	if err = f.runtime.QueryRow(f.ctx, "SELECT values_json FROM applications.record_drafts WHERE id=$1", draft.ID).Scan(&payload); err != nil || strings.Contains(string(payload), f.reference) {
		t.Fatalf("old key remains %s %v", payload, err)
	}
	var operations, audits int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2 AND result_json IS NOT NULL", f.actor, key).Scan(&operations); err != nil || operations != 1 {
		t.Fatalf("cleanup operation %d %v", operations, err)
	}
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1", f.app).Scan(&audits); err != nil || audits != 0 {
		t.Fatalf("cleanup wrote formal audit %d %v", audits, err)
	}
	missingKey := op()
	if _, err = f.service.UpdateDraft(f.ctx, f.principal, DraftUpdateRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: missingKey, ExpectedDraftVersion: 2, RemoveFieldIDs: []string{f.reference}}, applications.Metadata{RequestID: "v015-nonexistent-remove"}); !errors.Is(err, appdrafts.ErrInvalid) {
		t.Fatalf("removed a key absent from owner draft: %v", err)
	}
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, missingKey).Scan(&operations); err != nil || operations != 0 {
		t.Fatalf("missing-key cleanup confirmed %d %v", operations, err)
	}
}

func TestRestrictedDraftExplicitRemovalAfterFieldGrantRevoke(t *testing.T) {
	f := newRecordFixture(t)
	var key string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&key); err != nil {
		t.Fatal(err)
	}
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: key, SchemaVersion: 1, Values: map[string]any{f.public: "hidden"}}, applications.Metadata{RequestID: "v015-revoke-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action IN ('data.create','data.edit'))", f.app, f.public); err != nil {
		t.Fatal(err)
	}
	read, err := f.service.GetDraft(f.ctx, f.principal, f.app, f.view, draft.ID)
	if err != nil || !read.HasConflicts {
		t.Fatalf("revoked draft conflict %+v %v", read, err)
	}
	if len(read.Conflicts) != 1 || read.Conflicts[0].Reason != "FIELD_PERMISSION_REVOKED" {
		t.Fatalf("revoked field conflict %+v", read.Conflicts)
	}
	if _, leaked := read.Values[f.public]; leaked {
		t.Fatalf("revoked value leaked %+v", read)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&key); err != nil {
		t.Fatal(err)
	}
	cleaned, err := f.service.UpdateDraft(f.ctx, f.principal, DraftUpdateRequest{AppID: f.app, ViewID: f.view, DraftID: draft.ID, OperationID: key, ExpectedDraftVersion: 1, RemoveFieldIDs: []string{f.public}}, applications.Metadata{RequestID: "v015-revoke-remove"})
	if err != nil || cleaned.DraftVersion != 2 || cleaned.HasConflicts || len(cleaned.Values) != 0 {
		t.Fatalf("revoked key cleanup %+v %v", cleaned, err)
	}
}

func TestRestrictedDraftReadOwnerCursorAndRecordDetail(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, SchemaVersion: 1, Values: map[string]any{f.public: "private draft"}}, applications.Metadata{RequestID: "v015-draft-read"})
	if err != nil {
		t.Fatal(err)
	}
	read, err := f.service.GetDraft(f.ctx, f.principal, f.app, f.view, draft.ID)
	if err != nil || read.Values[f.public] != "private draft" {
		t.Fatalf("draft read %+v %v", read, err)
	}
	page, err := f.service.ListDrafts(f.ctx, f.principal, DraftListRequest{AppID: f.app, ViewID: f.view, PageSize: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != draft.ID {
		t.Fatalf("draft list %+v %v", page, err)
	}
	other := session.Principal{UserID: f.other, SessionRef: f.other, Record: session.Record{AuthVersion: "1"}}
	if _, err = f.service.GetDraft(f.ctx, other, f.app, f.view, draft.ID); !errors.Is(err, appdrafts.ErrMissing) {
		t.Fatalf("owner read foreign draft: %v", err)
	}
	record, err := f.service.GetRecord(f.ctx, f.principal, f.app, f.view, f.otherRecord)
	if err != nil || record.Values[f.public] != "beta" {
		t.Fatalf("record detail %+v %v", record, err)
	}
	if record.AppID != f.app || record.TableID != f.table || record.ViewID != f.view || record.SchemaVersion != 1 {
		t.Fatalf("detail identity/control mismatch %+v", record)
	}
	if _, ok := record.Values[f.secret]; ok {
		t.Fatalf("record detail leaked own-only field %+v", record)
	}
}

func TestRestrictedBoundDraftListUsesEditGrantForItsTarget(t *testing.T) {
	f := newRecordFixture(t)
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	base := int64(1)
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, SchemaVersion: 1, TargetRecordID: &f.ownRecord, BaseRecordVersion: &base, Values: map[string]any{f.public: "next"}}, applications.Metadata{RequestID: "v015-bound-list"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.create')", f.app); err != nil {
		t.Fatal(err)
	}
	if _, err = f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.create'", f.app); err != nil {
		t.Fatal(err)
	}
	page, err := f.service.ListDrafts(f.ctx, f.principal, DraftListRequest{AppID: f.app, ViewID: f.view, PageSize: 5})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != draft.ID || page.Items[0].HasConflicts {
		t.Fatalf("bound draft list %+v %v", page, err)
	}
}

func TestRestrictedQueryOnlyObservableChangesInvalidate(t *testing.T) {
	f := newRecordFixture(t)
	filter := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + f.public + `","operator":"eq","value":"alpha"}]}`)
	request := SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, Filter: filter}
	first, err := f.service.Search(f.ctx, f.principal, request)
	if err != nil || first.Total != 1 {
		t.Fatalf("initial %+v %v", first, err)
	}
	request.QueryVersion = first.QueryVersion
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	if _, err = f.owner.Exec(f.ctx, "UPDATE "+relation+" SET "+column+"='beta changed',record_version=record_version+1,updated_at=clock_timestamp() WHERE id=$1", f.otherRecord); err != nil {
		t.Fatal(err)
	}
	same, err := f.service.Search(f.ctx, f.principal, request)
	if err != nil || same.Total != 1 || same.QueryVersion != first.QueryVersion {
		t.Fatalf("unrelated row forced refresh %+v %v", same, err)
	}
	if _, err = f.owner.Exec(f.ctx, "UPDATE auth.users SET account='renamed-reference-'||id::text WHERE id=$1", f.other); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Search(f.ctx, f.principal, request); !errors.Is(err, querycontext.ErrChanged) {
		t.Fatalf("visible reference rename must invalidate old P: %v", err)
	}
}

func TestRestrictedSavedQuickSearchRemovedFieldVsRevokedPermission(t *testing.T) {
	t.Run("removed", func(t *testing.T) {
		f := newRecordFixture(t)
		quick := json.RawMessage(`{"term":"alpha","fieldIds":["` + f.public + `"]}`)
		req := SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, QuickSearch: quick}
		first, err := f.service.Search(f.ctx, f.principal, req)
		if err != nil || first.Total != 1 {
			t.Fatalf("initial quick search %+v %v", first, err)
		}
		req.QueryVersion = first.QueryVersion
		tx, err := f.owner.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if _, err = tx.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2", f.app, f.public); err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(map[string]any{"ID": f.public, "Type": "text"})
		if _, err = tx.Exec(f.ctx, "SELECT applications.apply_schema_change($1,$2,$3,'drop_column',$4::jsonb,NULL)", f.other, f.app, f.table, before); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(f.ctx, "UPDATE applications.fields SET removed=true WHERE app_id=$1 AND table_id=$2 AND id=$3", f.app, f.table, f.public); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=fields_json - 0,schema_version=2 WHERE app_id=$1 AND id=$2", f.app, f.table); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if _, err = f.service.Search(f.ctx, f.principal, req); !errors.Is(err, querycontext.ErrChanged) {
			t.Fatalf("removed saved quick field: %v", err)
		}
	})
	t.Run("revoked", func(t *testing.T) {
		f := newRecordFixture(t)
		quick := json.RawMessage(`{"term":"alpha","fieldIds":["` + f.public + `"]}`)
		req := SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, QuickSearch: quick}
		first, err := f.service.Search(f.ctx, f.principal, req)
		if err != nil {
			t.Fatal(err)
		}
		req.QueryVersion = first.QueryVersion
		if _, err = f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all')", f.app, f.public); err != nil {
			t.Fatal(err)
		}
		if _, err = f.service.Search(f.ctx, f.principal, req); !errors.Is(err, appquery.ErrForbidden) {
			t.Fatalf("revoked saved quick field: %v", err)
		}
	})
	t.Run("invalid incoming", func(t *testing.T) {
		f := newRecordFixture(t)
		if _, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, QuickSearch: json.RawMessage(`{"fieldIds":["` + f.public + `"]}`)}); !errors.Is(err, appquery.ErrInvalid) {
			t.Fatalf("invalid incoming quick search: %v", err)
		}
	})
}

func TestRestrictedReferencePredicateMissingRegistryIsUnavailable(t *testing.T) {
	f := newRecordFixture(t)
	filter := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + f.reference + `","operator":"eq","value":"` + f.other + `"}]}`)
	req := SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 2, Filter: filter}
	if _, err := f.owner.Exec(f.ctx, "UPDATE applications.member_sources SET status='deleted' WHERE id=$1", f.other); err != nil {
		t.Fatal(err)
	}
	tombstone, err := f.service.Search(f.ctx, f.principal, req)
	if err != nil || tombstone.Total != 2 {
		t.Fatalf("tombstone must remain searchable %+v %v", tombstone, err)
	}
	for _, row := range tombstone.Items {
		if display, ok := row.ReferenceDisplays[f.reference][f.other]; !ok || !display.Deleted {
			t.Fatalf("tombstone label missing %+v", row)
		}
	}
	if _, err = f.owner.Exec(f.ctx, "DELETE FROM applications.member_sources WHERE id=$1", f.other); err != nil {
		t.Fatal(err)
	}
	if result, err := f.service.Search(f.ctx, f.principal, req); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing source silently reduced COUNT to %d: %v", result.Total, err)
	}
}

func TestRestrictedListWriteChecksSavedProjectionBeforeClaim(t *testing.T) {
	f := newRecordFixture(t)
	filter := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + f.public + `","operator":"eq","value":"alpha"}]}`)
	initial, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, Filter: filter})
	if err != nil || initial.Total != 1 {
		t.Fatalf("initial %+v %v", initial, err)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	if _, err = f.owner.Exec(f.ctx, "UPDATE "+relation+" SET "+column+"='alpha',record_version=record_version+1 WHERE id=$1", f.otherRecord); err != nil {
		t.Fatal(err)
	}
	var op string
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	req := EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: op, QueryVersion: initial.QueryVersion, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "edited"}}
	if _, err = f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-list-change"}); !errors.Is(err, querycontext.ErrChanged) {
		t.Fatalf("changed saved projection accepted: %v", err)
	}
	var count int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&count); err != nil || count != 0 {
		t.Fatalf("operation claimed after changed projection: %d %v", count, err)
	}
	var value string
	if err = f.runtime.QueryRow(f.ctx, "SELECT "+column+" FROM "+relation+" WHERE id=$1", f.ownRecord).Scan(&value); err != nil || value != "alpha" {
		t.Fatalf("record mutated after changed projection: %q %v", value, err)
	}
	// A different row can change while the saved authorized P stays equal.
	if _, err = f.owner.Exec(f.ctx, "UPDATE "+relation+" SET "+column+"='beta',record_version=record_version+1 WHERE id=$1", f.otherRecord); err != nil {
		t.Fatal(err)
	}
	changed, err := f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-list-unchanged"})
	if err != nil || changed.RecordVersion != 2 {
		t.Fatalf("irrelevant revision blocked write %+v %v", changed, err)
	}
	// The original key recovers the confirmed result after its list token
	// becomes stale due to the write itself.
	replay, err := f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "v015-list-unchanged"})
	if err != nil || replay != changed {
		t.Fatalf("original key replay %+v %v", replay, err)
	}
}

func TestRestrictedAdvanceFailureOnlyReplaysConfirmedOriginalKey(t *testing.T) {
	f := newRecordFixture(t)
	filter := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + f.public + `","operator":"eq","value":"alpha"}]}`)
	first, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, Filter: filter})
	if err != nil || first.Total != 1 {
		t.Fatalf("baseline %+v %v", first, err)
	}
	var op string
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	confirmedReq := EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.otherRecord, OperationID: op, QueryVersion: first.QueryVersion, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "beta2"}}
	confirmed, err := f.service.Edit(f.ctx, f.principal, confirmedReq, applications.Metadata{RequestID: "v015-confirmed-before-advance-fault"})
	if err != nil || confirmed.RecordVersion != 2 {
		t.Fatalf("confirmed setup %+v %v", confirmed, err)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	if _, err = f.owner.Exec(f.ctx, "UPDATE "+relation+" SET "+column+"='beta3',record_version=record_version+1 WHERE id=$1", f.otherRecord); err != nil {
		t.Fatal(err)
	}
	opts, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })
	fault := New(f.runtime, failingAdvanceClient{client}, "consumer-"+strings.ReplaceAll(f.app, "-", ""))
	fault.Limits = f.service.Limits
	replayed, err := fault.Edit(f.ctx, f.principal, confirmedReq, applications.Metadata{RequestID: "v015-confirmed-before-advance-fault"})
	if err != nil || replayed != confirmed {
		t.Fatalf("confirmed original key recovery %+v %v", replayed, err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	unconfirmedReq := EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.otherRecord, OperationID: op, QueryVersion: first.QueryVersion, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 3, Changes: map[string]any{f.public: "should-not-write"}}
	if _, err = fault.Edit(f.ctx, f.principal, unconfirmedReq, applications.Metadata{RequestID: "v015-unconfirmed-advance-fault"}); err == nil || err.Error() != "injected Redis Advance failure" {
		t.Fatalf("new business mutation after receipt failure: %v", err)
	}
	var count int
	if err = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unconfirmed key claimed %d %v", count, err)
	}
	var value string
	if err = f.runtime.QueryRow(f.ctx, "SELECT "+column+" FROM "+relation+" WHERE id=$1", f.otherRecord).Scan(&value); err != nil || value != "beta3" {
		t.Fatalf("unconfirmed mutation executed %q %v", value, err)
	}
	if _, err = f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor); err != nil {
		t.Fatal(err)
	}
	if _, err = fault.Edit(f.ctx, f.principal, confirmedReq, applications.Metadata{RequestID: "v015-inactive-replay"}); !errors.Is(err, session.ErrUnauthorized) {
		t.Fatalf("inactive Session recovered operation: %v", err)
	}
}

func TestRestrictedRealRecordSearchMasksBeforeCountAndReusesContext(t *testing.T) {
	f := newRecordFixture(t)
	got, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || len(got.Items) != 2 || got.QueryVersion == "" {
		t.Fatalf("first page %+v", got)
	}
	if got.Page != 1 || got.PageSize != 2 || string(got.Sort) != "null" {
		t.Fatalf("page controls %+v", got)
	}
	for _, item := range got.Items {
		if item.AppID != f.app || item.TableID != f.table || item.ViewID != f.view || item.SchemaVersion != 1 {
			t.Fatalf("page item identity/control mismatch %+v", item)
		}
		if item.CreatedBy == f.actor {
			if item.Values[f.public] != "alpha" || item.Values[f.secret] != "private alpha" {
				t.Fatalf("own row %+v", item)
			}
		} else if item.CreatedBy == f.other {
			if item.Values[f.public] != "beta" {
				t.Fatalf("other row %+v", item)
			}
			if _, leaked := item.Values[f.secret]; leaked {
				t.Fatalf("own-only secret leaked %+v", item)
			}
		} else {
			t.Fatalf("unknown creator %+v", item)
		}
		if display, ok := item.ReferenceDisplays[f.reference][f.other]; !ok || display.Label != "v015-consumer-"+f.other || display.Deleted {
			t.Fatalf("reference display missing %+v", item)
		}
	}
	other, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 2, PageSize: 1, QueryVersion: got.QueryVersion})
	if err != nil {
		t.Fatal(err)
	}
	if other.Total != 2 || len(other.Items) != 1 || other.QueryVersion != got.QueryVersion {
		t.Fatalf("second page %+v", other)
	}
	quick := json.RawMessage(`{"term":"BETA","fieldIds":["` + f.public + `"]}`)
	filtered, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 1, QueryVersion: got.QueryVersion, QuickSearch: quick})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].Values[f.public] != "beta" {
		t.Fatalf("literal quick search %+v %v", filtered, err)
	}
}
