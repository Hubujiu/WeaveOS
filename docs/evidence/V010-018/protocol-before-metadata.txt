package auth

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRealHTTPSCookiesRestoreAndLogout(t *testing.T) {
	a := setup(t)
	a.user(t, "member", false)
	server := httptest.NewTLSServer(httpserver.NewHandler(a.service.Ready, a.service))
	defer server.Close()
	a.service.Origin = server.URL
	client := server.Client()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar = jar
	send := func(method, path, body string) *http.Response {
		t.Helper()
		r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Origin", server.URL)
		r.Header.Set("Content-Type", "application/json")
		for _, c := range jar.Cookies(r.URL) {
			if c.Name == "__Host-csrf" {
				r.Header.Set("X-CSRF-Token", c.Value)
			}
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := send("POST", "/api/v1/sessions", `{"account":"member","password":"Aa1!"}`)
	if res.StatusCode != 201 {
		t.Fatal("real HTTPS login must create Session")
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 2 {
		t.Fatal("real transport must issue two cookies")
	}
	for _, c := range cookies {
		if !c.Secure || c.Path != "/" || c.Domain != "" || c.MaxAge != 3600 || c.SameSite != http.SameSiteLaxMode {
			t.Fatal("Cookie security attributes incorrect")
		}
		if (c.Name == "__Host-session") != c.HttpOnly {
			t.Fatal("only Session cookie must be HttpOnly")
		}
	}
	res = send("GET", "/api/v1/sessions/current", "")
	if res.StatusCode != 200 {
		t.Fatal("HTTPS cookie jar must restore authenticated user")
	}
	res.Body.Close()
	res = send("DELETE", "/api/v1/sessions/current", "")
	if res.StatusCode != 204 {
		t.Fatal("bound HTTPS CSRF must permit logout")
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if len(b) != 0 {
		t.Fatal("204 must be empty on wire")
	}
	res = send("GET", "/api/v1/sessions/current", "")
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("logout cookies must be cleared and Session revoked")
	}
}

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

func TestUnknownAPIReturnsEnvelope404(t *testing.T) {
	a := setup(t)
	w := a.request("GET", "/api/v1/unknown", nil, nil, nil)
	checkStatus(t, w, 404)
	var envelope map[string]any
	if json.Unmarshal(w.Body.Bytes(), &envelope) != nil || len(envelope) != 4 || envelope["data"] != nil {
		t.Fatal("unknown API must return safe JSON envelope")
	}
}
func TestRevokedSessionAndFailedRegistrationProduceSanitizedAudit(t *testing.T) {
	a := setup(t)
	adminID := a.user(t, "admin", true)
	a.user(t, "member", false)
	cookies := a.login(t, "member")
	for _, c := range cookies {
		if c.Name == "__Host-session" {
			if _, err := a.service.Sessions.Revoke(context.Background(), c.Value); err != nil {
				t.Fatal(err)
			}
		}
	}
	checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 401)
	var count int
	if err := a.pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE event_type='session_invalid' AND outcome='failure'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("discovered revoked Session must produce an invalidation event")
	}
	code := a.invitation(t, adminID, 9)
	checkStatus(t, a.request("POST", "/api/v1/registrations", map[string]string{"account": "member", "password": "Aa1!", "invitationCode": code}, nil, nil), 409)
	if err := a.pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE event_type='register' AND outcome='failure' AND reason_code='USER_ACCOUNT_ALREADY_EXISTS'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("registration rollback must be followed by failure audit")
	}
}
