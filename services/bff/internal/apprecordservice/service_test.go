package apprecordservice

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type recordFixture struct {
	ctx                                            context.Context
	owner, runtime                                 *pgxpool.Pool
	actor, other, app, table, view, public, secret string
	principal                                      session.Principal
	service                                        *Service
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
	ids := make([]string, 9)
	for i := range ids {
		if err := owner.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	f := recordFixture{ctx: ctx, owner: owner, runtime: runtime, actor: ids[0], other: ids[1], app: ids[2], table: ids[3], view: ids[4], public: ids[5], secret: ids[6]}
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
	for i, id := range []string{f.public, f.secret} {
		definition, _ := json.Marshal(fieldDefs[i])
		if _, err := owner.Exec(ctx, "INSERT INTO applications.fields(id,app_id,table_id,definition) VALUES($1,$2,$3,$4)", id, f.app, f.table, definition); err != nil {
			t.Fatal(err)
		}
		physical, _ := json.Marshal(map[string]any{"ID": id, "Type": "text", "Required": false})
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
	}{{"menu.enter", "all", nil}, {"data.read", "all", []string{f.public}}, {"data.read", "own", []string{f.secret}}} {
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
	cols := fmt.Sprintf("f_%s,f_%s", strings.ReplaceAll(f.public, "-", ""), strings.ReplaceAll(f.secret, "-", ""))
	if _, err := owner.Exec(ctx, "INSERT INTO "+physical+"(id,created_by,"+cols+") VALUES($1,$2,'alpha','private alpha'),($3,$4,'beta','private beta')", ids[8], f.actor, ids[7], f.other); err != nil {
		t.Fatal(err)
	}
	f.principal = session.Principal{UserID: f.actor, SessionRef: ids[8], Record: session.Record{AuthVersion: "1"}}
	policy := querycontext.Policy{Validate: func(m querycontext.Metadata) bool {
		return m.View != "" && json.Valid(m.Criteria) && json.Valid(m.Revision)
	}, Forward: func(_ string, a, b json.RawMessage) bool { return json.Valid(a) && json.Valid(b) }}
	f.service = &Service{Pool: runtime, Queries: querycontext.NewStore(client, "applications", "consumer-"+strings.ReplaceAll(f.app, "-", ""), policy)}
	return f
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
	}
	other, err := f.service.Search(f.ctx, f.principal, SearchRequest{AppID: f.app, ViewID: f.view, Page: 2, PageSize: 1, QueryVersion: got.QueryVersion})
	if err != nil {
		t.Fatal(err)
	}
	if other.Total != 2 || len(other.Items) != 1 || other.QueryVersion != got.QueryVersion {
		t.Fatalf("second page %+v", other)
	}
}
