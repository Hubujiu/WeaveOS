package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
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

func TestRestrictedRealRecordSearchMasksBeforeCountAndReusesContext(t *testing.T) {
	f := newRecordFixture(t)
	got, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || len(got.Items) != 2 || got.QueryVersion == "" {
		t.Fatalf("first page %+v", got)
	}
	for _, item := range got.Items {
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
