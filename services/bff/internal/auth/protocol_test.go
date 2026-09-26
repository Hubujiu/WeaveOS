package auth

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// ADR-002 API-02/14/17: strict write DTOs, real HTTP status, same-origin writes.
func TestHTTPProtocolRejectsMalformedWriteDTOs(t *testing.T) {
	a := setup(t)
	a.user(t, "member", false)
	for _, tc := range []struct {
		name, body, media, source string
		status                    int
	}{
		{"unknown", "{\"account\":\"member\",\"password\":\"Aa1!\",\"isBootstrapAdmin\":true}", "application/json", origin, 400},
		{"trailing", "{\"account\":\"member\",\"password\":\"Aa1!\"} {}", "application/json", origin, 400},
		{"duplicate", "{\"account\":\"member\",\"account\":\"member\",\"password\":\"Aa1!\"}", "application/json", origin, 400},
		{"null", "{\"account\":null,\"password\":\"Aa1!\"}", "application/json", origin, 400},
		{"wrong_media", "{\"account\":\"member\",\"password\":\"Aa1!\"}", "text/plain", origin, 415},
		{"foreign_source", "{\"account\":\"member\",\"password\":\"Aa1!\"}", "application/json", "https://evil.example", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", origin+"/api/v1/sessions", strings.NewReader(tc.body))
			r.Header.Set("Origin", tc.source)
			r.Header.Set("Content-Type", tc.media)
			w := httptest.NewRecorder()
			w.Header().Set("X-Request-Id", "protocol-request")
			a.service.ServeHTTP(w, r)
			checkStatus(t, w, tc.status)
			if len(w.Result().Cookies()) != 0 {
				t.Fatal("invalid write issued Cookie")
			}
		})
	}
}
func TestAuditRequestIDMatchesResponseAndUnknownAccountHMAC(t *testing.T) {
	a := setup(t)
	r := httptest.NewRequest("POST", origin+"/api/v1/sessions", strings.NewReader(`{"account":"unique-unknown","password":"Aa1!"}`))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	w.Header().Set("X-Request-Id", "unique-server-request-id")
	a.service.ServeHTTP(w, r)
	checkStatus(t, w, 401)
	var body struct {
		Meta struct {
			RequestID string `json:"requestId"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := a.pool.QueryRow(context.Background(), "SELECT request_id FROM auth.authentication_events ORDER BY occurred_at DESC LIMIT 1").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != body.Meta.RequestID || stored != "unique-server-request-id" {
		t.Fatal("audit and response must share trusted request ID")
	}
}
func TestResetAndInvitationAuditFailureRollBack(t *testing.T) {
	a := setup(t)
	a.user(t, "admin", true)
	id := a.user(t, "target", false)
	admin := a.login(t, "admin")
	ctx := context.Background()
	var beforeHash string
	var beforeVersion int64
	if err := a.pool.QueryRow(ctx, "SELECT c.password_hash,u.auth_version FROM auth.users u JOIN auth.password_credentials c ON c.user_id=u.id WHERE u.id=$1", id).Scan(&beforeHash, &beforeVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := a.pool.Exec(ctx, "ALTER TABLE auth.authentication_events ADD CONSTRAINT auth_test_deny CHECK(false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.pool.Exec(ctx, "ALTER TABLE auth.authentication_events DROP CONSTRAINT IF EXISTS auth_test_deny")
	})
	checkStatus(t, a.request("POST", "/api/v1/users/"+id+"/password-reset", map[string]string{}, admin, nil), 503)
	var afterHash string
	var afterVersion int64
	a.pool.QueryRow(ctx, "SELECT c.password_hash,u.auth_version FROM auth.users u JOIN auth.password_credentials c ON c.user_id=u.id WHERE u.id=$1", id).Scan(&afterHash, &afterVersion)
	if beforeHash != afterHash || beforeVersion != afterVersion {
		t.Fatal("audit failure committed credential/version mutation")
	}
	checkStatus(t, a.request("POST", "/api/v1/invitations", map[string]string{}, admin, nil), 503)
	var count int
	a.pool.QueryRow(ctx, "SELECT count(*) FROM auth.invitations").Scan(&count)
	if count != 0 {
		t.Fatal("audit failure committed invitation")
	}
}
