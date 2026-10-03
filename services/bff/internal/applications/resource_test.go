package applications

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"
)

func TestB5ResourceOwnershipClosedGrantAndConfigurationReads(t *testing.T) {
	f := fixture(t, true)
	app, _ := f.create(t)
	foreign, _ := f.create(t)
	base := "/api/v1/applications/" + app
	g := data(t, f.call("POST", base+"/permission-groups", map[string]any{"name": "资源组", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201)
	gid := value(t, g, "id")
	gp := base + "/permission-groups/" + gid
	for _, part := range []string{"", "/" + gid + "/members", "/" + gid + "/grants"} {
		data(t, f.call("GET", base+"/permission-groups"+part, nil), 200)
	}
	for _, grant := range []map[string]any{{"resourceKind": "application", "resourceId": foreign, "action": "menu.enter", "rowScope": "all", "fields": []string{}}, {"resourceKind": "application", "resourceId": f.uuid(t), "action": "menu.enter", "rowScope": "all", "fields": []string{}}, {"resourceKind": "application", "resourceId": app, "action": "data.read", "rowScope": "all", "fields": []string{}}, {"resourceKind": "view", "resourceId": app, "action": "menu.enter", "rowScope": "all", "fields": []string{}}, {"resourceKind": "application", "resourceId": app, "action": "menu.enter", "rowScope": "all", "fields": []string{f.uuid(t)}}, {"resourceKind": "application", "resourceId": app, "action": "menu.enter", "rowScope": "all"}} {
		w := f.call("PUT", gp+"/grants", map[string]any{"grants": []any{grant}, "operationId": f.operation(t), "expectedPolicyRevision": 2})
		code := "APPLICATION_RESOURCE_INVALID"
		if _, hasFields := grant["fields"]; !hasFields {
			code = "COMMON_INVALID_ARGUMENT"
		}
		if w.Code != 400 || !strings.Contains(w.Body.String(), code) {
			t.Fatalf("unregistered/cross-app/data/field grant must reject: %d %s", w.Code, w.Body.String())
		}
	}
	for _, list := range []string{"null", "missing"} {
		body := `{"operationId":"` + f.operation(t) + `","expectedPolicyRevision":2`
		if list == "null" {
			body += `,"memberIds":null`
		}
		body += "}"
		if w := f.request("PUT", gp+"/members", body, true, true); w.Code != 400 {
			t.Fatal("missing/null replacement must not clear members")
		}
	}
	body := `{"grants":[{"resourceKind":"application","resourceId":"` + app + `","action":"menu.enter","action":"menu.enter","rowScope":"all","fields":[]}],"operationId":"` + f.operation(t) + `","expectedPolicyRevision":2}`
	if w := f.request("PUT", gp+"/grants", body, true, true); w.Code != 400 {
		t.Fatal("nested duplicate grant keys must reject")
	}
	if w := f.call("PUT", "/api/v1/applications/"+foreign+"/permission-groups/"+gid+"/members", map[string]any{"memberIds": []string{}, "operationId": f.operation(t), "expectedPolicyRevision": 1}); w.Code != 404 {
		t.Fatal("group route must bind actual same-app owner")
	}
	var rev int
	if err := f.owner.QueryRow(context.Background(), "SELECT policy_revision FROM applications.apps WHERE id=$1", app).Scan(&rev); err != nil || rev != 2 {
		t.Fatal("all rejected grants must preserve policy revision")
	}
	member := fixture(t, false)
	data(t, f.call("PUT", gp+"/members", map[string]any{"memberIds": []string{member.actor}, "operationId": f.operation(t), "expectedPolicyRevision": 2}), 200)
	if _, err := f.owner.Exec(context.Background(), "UPDATE auth.users SET status='disabled' WHERE id=$1", member.actor); err != nil {
		t.Fatal(err)
	}
	if w := member.call("GET", base+"/access", nil); w.Code != 401 {
		t.Fatal("members admitted while active must lose access after disable")
	}
	// Identical non-replay full replacement is still a policy change.
	d := data(t, f.call("PUT", gp+"/members", map[string]any{"memberIds": []string{member.actor}, "operationId": f.operation(t), "expectedPolicyRevision": 3}), 200)
	if revision(t, d) != 4 {
		t.Fatal("identical new-key replacement increments policy revision")
	}
	data(t, f.call("PUT", gp, map[string]any{"name": "禁用组", "enabled": false, "operationId": f.operation(t), "expectedPolicyRevision": 4}), 200)
	if _, err := f.owner.Exec(context.Background(), "UPDATE personnel.permission_catalog SET enabled=false WHERE app_id=$1", app); err != nil {
		t.Fatal(err)
	}
	if w := f.call("GET", base+"/access", nil); w.Code != 403 {
		t.Fatal("Bootstrap still requires actual enabled catalog")
	}
	list := data(t, f.call("GET", "/api/v1/applications", nil), 200)
	if strings.Contains(string(list["items"]), app) {
		t.Fatal("disabled catalog app must disappear from discovery")
	}
}
func TestB5SQLSameAppForeignKeysFixedOwnerAndCatalogPrivilege(t *testing.T) {
	f := fixture(t, true)
	app, _ := f.create(t)
	foreign, _ := f.create(t)
	g := data(t, f.call("POST", "/api/v1/applications/"+app+"/permission-groups", map[string]any{"name": "FK", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201)
	gid := value(t, g, "id")
	ctx := context.Background()
	for _, q := range []struct {
		sql  string
		args []any
		code string
	}{{"INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", []any{foreign, gid, f.actor}, "23503"}, {"INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'application',$1,'menu.enter','all')", []any{foreign, gid}, "23503"}, {"INSERT INTO applications.menu_resources(app_id,resource_kind,resource_id) VALUES($1,'application',$2)", []any{app, foreign}, "23514"}, {"UPDATE applications.apps SET owner_user_id=$2 WHERE id=$1", []any{app, f.actor}, "42501"}, {"UPDATE personnel.permission_catalog SET name='forged' WHERE app_id=$1", []any{app}, "42501"}} {
		_, err := f.runtime.Exec(ctx, q.sql, q.args...)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != q.code {
			t.Fatalf("actual auth_app must enforce %s: %v", q.code, err)
		}
	}
	other := fixture(t, false)
	_, err := f.owner.Exec(ctx, "UPDATE applications.apps SET owner_user_id=$2 WHERE id=$1", app, other.actor)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" {
		t.Fatal("even owner SQL cannot transfer immutable application owner")
	}
	if _, err := f.runtime.Exec(ctx, "SELECT applications.register_catalog_entry($1)", app); err != nil {
		t.Fatal("identical catalog registration is idempotent", err)
	}
	var definer bool
	var settings []string
	if err := f.owner.QueryRow(ctx, "SELECT prosecdef,proconfig FROM pg_proc WHERE oid='applications.register_catalog_entry(uuid)'::regprocedure").Scan(&definer, &settings); err != nil || !definer || len(settings) != 1 || settings[0] != "search_path=pg_catalog" {
		t.Fatal("catalog registration has exact fixed SECURITY DEFINER search path")
	}
	for _, role := range []string{"auth_reader", "auth_maintenance", "auth_backup"} {
		var permitted bool
		if err := f.owner.QueryRow(ctx, "SELECT has_function_privilege($1,'applications.register_catalog_entry(uuid)','EXECUTE')", role).Scan(&permitted); err != nil || permitted {
			t.Fatal("only runtime can invoke catalog registrar")
		}
	}
	var stored []byte
	if err := f.owner.QueryRow(ctx, "SELECT result_json FROM applications.operations WHERE actor_user_id=$1 ORDER BY created_at DESC LIMIT 1", f.actor).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if json.Unmarshal(stored, &fields) != nil {
		t.Fatal("durable result JSON")
	}
	for _, key := range []string{"cookie", "session", "headers", "csrf", "password"} {
		if _, exists := fields[key]; exists {
			t.Fatal("operation must store only safe minimal result")
		}
	}
}
