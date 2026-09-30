package main

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCompositionWiresPersonnelAndOrdinaryInvitationQ25(t *testing.T) {
	ctx := context.Background()
	cfg := config{DatabaseURL: os.Getenv("WEAVEOS_TEST_DATABASE_URL"), RedisURL: os.Getenv("WEAVEOS_TEST_REDIS_URL"), Origin: "https://weaveos.test", Generation: "personnel-composition", AuditKeyID: "test", AuditKey: []byte("synthetic-audit-key-32-bytes-only!")}
	owner, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	var user, identity string
	if err := owner.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('composition-'||gen_random_uuid()) RETURNING id::text").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := owner.QueryRow(ctx, "INSERT INTO personnel.identities(name) VALUES('composition-manager') RETURNING id::text").Scan(&identity); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'personnel.manage')", identity); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", user, identity); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = owner.Exec(ctx, "DELETE FROM auth.authentication_events WHERE actor_user_id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM auth.invitations WHERE created_by=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.member_identities WHERE user_id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.member_configuration WHERE user_id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.identities WHERE id=$1", identity)
		_, _ = owner.Exec(ctx, "DELETE FROM auth.users WHERE id=$1", user)
	}()
	store := session.NewStore(cfg.RedisURL, cfg.Generation)
	defer store.Close()
	sid, csrf, err := store.Create(ctx, session.Record{UserID: user, SessionRef: "11111111-1111-4111-8111-111111111111", AuthVersion: "1"})
	if err != nil {
		t.Fatal("isolated Session required")
	}
	defer store.Revoke(ctx, sid)
	h, close, err := buildHandler(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, cfg.Origin+path, strings.NewReader(body))
		r.Header.Set("Origin", cfg.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	t.Run("own access", func(t *testing.T) {
		w := request("GET", "/api/v1/me/access", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"personnelManage":true`) || !strings.Contains(w.Body.String(), `"bootstrapAdmin":false`) {
			t.Fatalf("new module must be composed with same Session, got %d", w.Code)
		}
	})
	t.Run("ordinary invitation", func(t *testing.T) {
		w := request("POST", "/api/v1/invitations", "{}")
		if w.Code != 201 {
			t.Fatalf("qualified non-Root invite HTTP: %d", w.Code)
		}
	})
	t.Run("password reset remains Root", func(t *testing.T) {
		w := request("POST", "/api/v1/users/"+user+"/password-reset", "{}")
		if w.Code != 403 {
			t.Fatal("manage must not grant Root recovery")
		}
	})
}
