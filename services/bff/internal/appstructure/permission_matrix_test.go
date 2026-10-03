package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rootCall(t *testing.T, f *fixture, method, path string, body any, expected string) *httptest.ResponseRecorder {
	t.Helper()
	adapter := &applications.Service{Application: &applications.Application{Pool: f.runtime}, Authenticator: f.service.Authenticator, Definitions: f.service}
	raw, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest(method, "https://weaveos.test"+path, strings.NewReader(string(raw)))
	r.Header.Set("Origin", "https://weaveos.test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", f.csrf)
	if expected != "" {
		r.Header.Set("X-Expected-Actor-Id", expected)
	}
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
	r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: f.csrf})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, r)
	return w
}

func TestActualMenuGranteeCannotManageChildDefinitionsOrEnumerateCandidates(t *testing.T) {
	owner, member := setup(t), setup(t)
	view, _ := newForm(t, owner)
	var group string
	if e := owner.owner.QueryRow(context.Background(), "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'registered actual menu group') RETURNING id::text", owner.app).Scan(&group); e != nil {
		t.Fatal(e)
	}
	for _, s := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", []any{owner.app, group, member.actor}},
		{"INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'application',$1,'menu.enter','all')", []any{owner.app, group}},
	} {
		if _, e := owner.owner.Exec(context.Background(), s.query, s.args...); e != nil {
			t.Fatal(e)
		}
	}
	data(t, rootCall(t, member, "GET", "/api/v1/applications/"+owner.app+"/access", nil, member.actor), 200)
	for _, path := range []string{"/structure", "/member-candidates", "/forms/" + view + "/definition"} {
		w := rootCall(t, member, "GET", "/api/v1/applications/"+owner.app+path, nil, member.actor)
		if w.Code != 403 || !strings.Contains(w.Body.String(), "APPLICATION_FORBIDDEN") {
			t.Fatalf("registered root menu must not inherit child management %s: %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestCreateOnlyExpectedActorGuardAndOwnerBootstrapManagement(t *testing.T) {
	creator, foreign := setup(t), setup(t)
	var identity string
	if e := creator.owner.QueryRow(context.Background(), "INSERT INTO personnel.identities(name) VALUES('v013-create-only') RETURNING id::text").Scan(&identity); e != nil {
		t.Fatal(e)
	}
	for _, s := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", []any{creator.actor, identity}},
		{"INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'applications.create')", []any{identity}},
	} {
		if _, e := creator.owner.Exec(context.Background(), s.query, s.args...); e != nil {
			t.Fatal(e)
		}
	}
	in := map[string]any{"name": "created by actual create-only", "operationId": uuid(t, creator.owner)}
	w := rootCall(t, creator, "POST", "/api/v1/applications", in, foreign.actor)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "AUTH_SESSION_CHANGED") {
		t.Fatalf("create boundary guard %d %s", w.Code, w.Body.String())
	}
	w = rootCall(t, creator, "POST", "/api/v1/applications", in, creator.actor)
	created := data(t, w, 201)
	app := created["id"].(string)
	data(t, rootCall(t, creator, "GET", "/api/v1/applications/"+app+"/structure", nil, creator.actor), 200)
	w = rootCall(t, creator, "GET", "/api/v1/applications/"+foreign.app+"/structure", nil, creator.actor)
	if w.Code != 403 {
		t.Fatalf("create-only cannot manage foreign app %d %s", w.Code, w.Body.String())
	}
	var root, version string
	e := creator.owner.QueryRow(context.Background(), "SELECT id::text,auth_version::text FROM auth.users WHERE is_bootstrap_admin AND status='active'").Scan(&root, &version)
	if e == pgx.ErrNoRows {
		e = creator.owner.QueryRow(context.Background(), "INSERT INTO auth.users(account,is_bootstrap_admin) VALUES('v013-root-'||gen_random_uuid(),true) RETURNING id::text,auth_version::text").Scan(&root, &version)
	}
	if e != nil {
		t.Fatal(e)
	}
	bootstrap := *creator
	bootstrap.actor = root
	bootstrap.sid, bootstrap.csrf, e = creator.session.Create(context.Background(), session.Record{UserID: root, AuthVersion: version, SessionRef: uuid(t, creator.owner)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { creator.session.Revoke(context.Background(), bootstrap.sid) })
	data(t, rootCall(t, &bootstrap, "GET", "/api/v1/applications/"+foreign.app+"/structure", nil, root), 200)
}
