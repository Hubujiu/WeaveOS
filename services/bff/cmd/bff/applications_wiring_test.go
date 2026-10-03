package main

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestB5CompositionWiresApplicationsWithSameSession(t *testing.T) {
	ctx := context.Background()
	cfg := config{DatabaseURL: os.Getenv("WEAVEOS_TEST_DATABASE_URL"), RedisURL: os.Getenv("WEAVEOS_TEST_REDIS_URL"), Origin: "https://weaveos.test", Generation: "b5-composition", AuditKeyID: "test", AuditKey: []byte("synthetic-audit-key-32-bytes-only!")}
	owner, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	var actor, op string
	err = owner.QueryRow(ctx, "SELECT id::text,gen_random_uuid()::text FROM auth.users WHERE is_bootstrap_admin").Scan(&actor, &op)
	if err == pgx.ErrNoRows {
		err = owner.QueryRow(ctx, "INSERT INTO auth.users(account,is_bootstrap_admin) VALUES('composition-root-'||gen_random_uuid(),true) RETURNING id::text,gen_random_uuid()::text").Scan(&actor, &op)
	}
	if err != nil {
		t.Fatal(err)
	}
	store := session.NewStore(cfg.RedisURL, cfg.Generation)
	defer store.Close()
	var version string
	if err := owner.QueryRow(ctx, "SELECT auth_version::text FROM auth.users WHERE id=$1", actor).Scan(&version); err != nil {
		t.Fatal(err)
	}
	sid, csrf, err := store.Create(ctx, session.Record{UserID: actor, SessionRef: op, AuthVersion: version})
	if err != nil {
		t.Fatal(err)
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
	w := request("POST", "/api/v1/applications", `{"name":"composition","operationId":"`+op+`"}`)
	if w.Code != 201 {
		t.Fatalf("production composition must expose frozen creation: %d %s", w.Code, w.Body.String())
	}
	var value struct{ Data struct{ ID string } }
	if json.Unmarshal(w.Body.Bytes(), &value) != nil || value.Data.ID == "" {
		t.Fatal("committed application result")
	}
	for _, path := range []string{"/api/v1/applications", "/api/v1/applications/" + value.Data.ID + "/access", "/api/v1/application-operations/" + op} {
		if w := request("GET", path, ""); w.Code != 200 {
			t.Fatalf("composed GET %s: %d %s", path, w.Code, w.Body.String())
		}
	}
}
