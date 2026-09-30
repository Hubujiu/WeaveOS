package personnel

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PRD R2-AC09–11: this application exists only inside this test. Its own
// resource owner is authoritative; a query appId cannot relabel a B resource.
func TestApplicationFixtureRealHTTPSCentralAndOwnedResourceBoundaries(t *testing.T) {
	f := setupWeb(t)
	ctx := context.Background()
	auth := session.Authenticator{Sessions: f.store, DB: f.app.Pool, Origin: "https://weaveos.test"}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "fixture" {
			w.WriteHeader(404)
			return
		}
		// App identity comes from this registered server route, never query/body.
		appID := parts[1]
		if appID != "test-A" && appID != "test-B" {
			w.WriteHeader(404)
			return
		}
		p, err := auth.Authenticate(r, false)
		if err != nil {
			w.WriteHeader(401)
			return
		}
		if err = f.app.AllowApplication(r.Context(), p, appID); err != nil {
			if errors.Is(err, ErrDenied) {
				w.WriteHeader(403)
			} else {
				w.WriteHeader(503)
			}
			return
		}
		if len(parts) == 4 {
			owned := map[string]string{"document-A": "test-A", "document-B": "test-B"}
			if owned[parts[3]] != appID {
				w.WriteHeader(404)
				return
			}
			// This fixture app grants A's resource to exactly one ordinary account.
			// Neither personnel.manage nor trusted Root bypasses this internal rule.
			if parts[3] != "document-A" || p.UserID != f.actor.UserID {
				w.WriteHeader(403)
				return
			}
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	var rootID, rootVersion string
	err := f.owner.QueryRow(ctx, "SELECT id::text,auth_version::text FROM auth.users WHERE is_bootstrap_admin").Scan(&rootID, &rootVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		err = f.owner.QueryRow(ctx, "INSERT INTO auth.users(account,is_bootstrap_admin) VALUES('fixture-root-'||gen_random_uuid(),true) RETURNING id::text,auth_version::text").Scan(&rootID, &rootVersion)
		t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM auth.users WHERE id=$1", rootID) })
	}
	if err != nil {
		t.Fatal(err)
	}
	rootSID, _, err := f.store.Create(ctx, session.Record{UserID: rootID, AuthVersion: rootVersion, SessionRef: "11111111-1111-4111-8111-111111111112"})
	if err != nil {
		t.Fatal(err)
	}
	defer f.store.Revoke(ctx, rootSID)
	request := func(path, sid string, want int) {
		t.Helper()
		r, _ := http.NewRequest("GET", server.URL+path, nil)
		if sid != "" {
			r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		}
		response, err := server.Client().Do(r)
		if err != nil {
			t.Fatal("real TLS fixture request failed")
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("fixture boundary %s: got %d want %d", path, response.StatusCode, want)
		}
	}
	request("/fixture/test-A/page", "", 401)
	for _, path := range []string{"/fixture/test-A/page", "/fixture/test-A/api/document-A", "/fixture/test-B/page"} {
		request(path, f.sid, 200)
	}
	request("/fixture/test-A/api/document-B?appId=test-A", f.sid, 404)
	request("/fixture/test-B/api/document-B", f.sid, 403)
	request("/fixture/test-A/api/document-A", rootSID, 403)
	if _, err = f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='app.test.B'", f.template); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/fixture/test-B/page", "/fixture/test-B/api/document-B?appId=test-A"} {
		request(path, f.sid, 403)
	}
	request("/fixture/test-A/page", f.sid, 200)
	if _, err = f.owner.Exec(ctx, "UPDATE personnel.permission_catalog SET name='renamed A' WHERE code='app.test.A'"); err != nil {
		t.Fatal(err)
	}
	request("/fixture/test-A/api/document-A", f.sid, 200)
	if _, err = f.owner.Exec(ctx, "INSERT INTO personnel.permission_catalog(code,name,category,app_id) VALUES('app.test.C','new C','application','test-C') ON CONFLICT(code) DO UPDATE SET enabled=true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.permission_catalog WHERE code='app.test.C'") })
	if err = f.app.AllowApplication(ctx, f.actor, "test-C"); !errors.Is(err, ErrDenied) {
		t.Fatal("new application must not grant an existing ordinary identity")
	}
	if _, err = f.owner.Exec(ctx, "UPDATE personnel.permission_catalog SET enabled=false WHERE code='app.test.A'"); err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{f.sid, rootSID} {
		for _, path := range []string{"/fixture/test-A/page", "/fixture/test-A/api/document-A"} {
			request(path, sid, 403)
		}
	}
	if _, err = f.store.Revoke(ctx, f.sid); err != nil {
		t.Fatal(err)
	}
	request("/fixture/test-A/page", f.sid, 401)
}
