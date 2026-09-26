package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
)

const origin = "https://app.example.test"

type testApp struct {
	service *Service
	pool    *pgxpool.Pool
	client  *redis.Client
	logs    *bytes.Buffer
}

func setup(t *testing.T) *testApp {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	rurl := os.Getenv("WEAVEOS_TEST_REDIS_URL")
	if dsn == "" || rurl == "" {
		t.Fatal("isolated PostgreSQL and Redis URLs required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "weaveos_") {
		t.Fatal("test database required")
	}
	if _, err := pool.Exec(ctx, "TRUNCATE auth.authentication_events,auth.invitations,auth.password_credentials,auth.users CASCADE"); err != nil {
		t.Fatal(err)
	}
	opt, err := redis.ParseURL(rurl)
	if err != nil || opt.DB != 15 {
		t.Fatal("isolated Redis DB15 required")
	}
	client := redis.NewClient(opt)
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	logs := new(bytes.Buffer)
	s := &Service{Pool: pool, Sessions: session.NewStore(rurl, "auth-http-tests"), Origin: origin, AuditKeyID: "test", AuditKey: bytes.Repeat([]byte{7}, 32), Logger: slog.New(slog.NewJSONHandler(logs, nil))}
	t.Cleanup(func() { client.FlushDB(ctx); client.Close(); pool.Close() })
	return &testApp{s, pool, client, logs}
}
func (a *testApp) user(t *testing.T, account string, admin bool) string {
	t.Helper()
	salt := []byte("test-salt16bytes")
	digest := argon2.IDKey([]byte("Aa1!"), salt, 2, 19456, 1, 32)
	phc := "$argon2id$v=19$m=19456,t=2,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(digest)
	var id string
	if err := a.pool.QueryRow(context.Background(), "INSERT INTO auth.users(account,is_bootstrap_admin) VALUES($1,$2) RETURNING id", account, admin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.pool.Exec(context.Background(), "INSERT INTO auth.password_credentials(user_id,password_hash) VALUES($1,$2)", id, phc); err != nil {
		t.Fatal(err)
	}
	return id
}
func (a *testApp) invitation(t *testing.T, creator string, n byte) string {
	t.Helper()
	raw := bytes.Repeat([]byte{n}, 32)
	hash := sha256.Sum256(raw)
	if _, err := a.pool.Exec(context.Background(), "INSERT INTO auth.invitations(code_hash,created_by) VALUES($1,$2)", hash[:], creator); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
func (a *testApp) request(method, path string, body any, cookies []*http.Cookie, header map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, origin+path, reader)
	r.RemoteAddr = "192.0.2.7:4321"
	r.Header.Set("Origin", origin)
	r.Header.Set("User-Agent", "test-browser")
	r.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		r.AddCookie(c)
		if c.Name == session.CSRFCookieName {
			r.Header.Set("X-CSRF-Token", c.Value)
		}
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	w.Header().Set("X-Request-Id", "independent-test-request")
	a.service.ServeHTTP(w, r)
	return w
}
func checkStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d want=%d (response omitted to protect credentials)", w.Code, want)
	}
}
func (a *testApp) login(t *testing.T, account string) []*http.Cookie {
	t.Helper()
	w := a.request("POST", "/api/v1/sessions", map[string]string{"account": account, "password": "Aa1!"}, nil, nil)
	checkStatus(t, w, 201)
	return w.Result().Cookies()
}
func TestRegistrationAtomicHTTP(t *testing.T) {
	a := setup(t)
	admin := a.user(t, "admin", true)
	code := a.invitation(t, admin, 1)
	body := map[string]string{"account": "Alice", "password": "Aa1!", "invitationCode": code}
	w := a.request("POST", "/api/v1/registrations", body, nil, nil)
	checkStatus(t, w, 201)
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("registration must not create Session")
	}
	body["account"] = "other"
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 409)
	body["invitationCode"] = a.invitation(t, admin, 2)
	body["account"] = "alice"
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 201)
	body["invitationCode"] = a.invitation(t, admin, 3)
	body["account"] = "Alice"
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 409)
	body["account"] = "after-conflict"
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 201)
	body["invitationCode"] = a.invitation(t, admin, 4)
	body["account"] = "space inside"
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 400)
	body["account"] = "Trimmed"
	body["password"] = "aa1!"
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 400)
	body["password"] = "Aa1!"
	body["account"] = "  Trimmed  "
	checkStatus(t, a.request("POST", "/api/v1/registrations", body, nil, nil), 201)
	a.login(t, "Trimmed")
}
func TestConcurrentInvitationHTTP(t *testing.T) {
	a := setup(t)
	admin := a.user(t, "admin", true)
	code := a.invitation(t, admin, 5)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, account := range []string{"race-a", "race-b"} {
		wg.Add(1)
		go func(account string) {
			defer wg.Done()
			statuses <- a.request("POST", "/api/v1/registrations", map[string]string{"account": account, "password": "Aa1!", "invitationCode": code}, nil, nil).Code
		}(account)
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("expected one success and one used-invitation conflict: %v", counts)
	}
}
func TestLoginCurrentLogoutHTTP(t *testing.T) {
	a := setup(t)
	a.user(t, "member", false)
	checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, nil, map[string]string{"X-User-Id": "admin"}), 401)
	for _, account := range []string{"member", "unknown"} {
		w := a.request("POST", "/api/v1/sessions", map[string]string{"account": account, "password": "wrong"}, nil, nil)
		checkStatus(t, w, 401)
		if len(w.Result().Cookies()) != 0 {
			t.Fatal("failed login issued Cookie")
		}
	}
	cookies := a.login(t, "member")
	if len(cookies) != 2 {
		t.Fatal("expected independent Session and CSRF cookies")
	}
	w := a.request("GET", "/api/v1/sessions/current", nil, cookies, nil)
	checkStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "csrfToken") {
		t.Fatal("JSON must not contain token")
	}
	checkStatus(t, a.request("DELETE", "/api/v1/sessions/current", nil, cookies, map[string]string{"X-CSRF-Token": ""}), 403)
	checkStatus(t, a.request("DELETE", "/api/v1/sessions/current", nil, cookies, map[string]string{"Origin": "https://evil.example"}), 403)
	w = a.request("DELETE", "/api/v1/sessions/current", nil, cookies, nil)
	checkStatus(t, w, 204)
	if w.Body.Len() != 0 {
		t.Fatal("204 must have no body")
	}
	checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 401)
}
func TestBootstrapInvitationResetAndRevocationHTTP(t *testing.T) {
	a := setup(t)
	a.user(t, "admin", true)
	id := a.user(t, "member", false)
	member := a.login(t, "member")
	admin := a.login(t, "admin")
	path := "/api/v1/users/" + id + "/password-reset"
	checkStatus(t, a.request("POST", "/api/v1/invitations", map[string]string{}, member, nil), 403)
	checkStatus(t, a.request("POST", path, map[string]string{}, member, nil), 403)
	w := a.request("POST", "/api/v1/invitations", map[string]string{}, admin, nil)
	checkStatus(t, w, 201)
	var envelope struct {
		Data struct {
			InvitationCode string `json:"invitationCode"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	code := envelope.Data.InvitationCode
	raw, err := base64.RawURLEncoding.Strict().DecodeString(code)
	if err != nil || len(raw) != 32 {
		t.Fatal("issued invitation is not canonical32")
	}
	checkStatus(t, a.request("POST", "/api/v1/registrations", map[string]string{"account": "issued", "password": "Aa1!", "invitationCode": code}, nil, nil), 201)
	w = a.request("POST", path, map[string]string{}, admin, nil)
	checkStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Abc@123456") {
		t.Fatal("reset response leaks password")
	}
	checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, member, nil), 401)
	checkStatus(t, a.request("POST", "/api/v1/sessions", map[string]string{"account": "member", "password": "Aa1!"}, nil, nil), 401)
	checkStatus(t, a.request("POST", "/api/v1/sessions", map[string]string{"account": "member", "password": "Abc@123456"}, nil, nil), 201)
	var h1, h2 string
	a.pool.QueryRow(context.Background(), "SELECT password_hash FROM auth.password_credentials WHERE user_id=$1", id).Scan(&h1)
	checkStatus(t, a.request("POST", path, map[string]string{}, admin, nil), 200)
	a.pool.QueryRow(context.Background(), "SELECT password_hash FROM auth.password_credentials WHERE user_id=$1", id).Scan(&h2)
	if h1 == h2 || h2 == "Abc@123456" || !strings.HasPrefix(h2, "$argon2id$") {
		t.Fatal("each reset must store a fresh salted hash")
	}
}
func TestAuditFailureFailsLoginClosedAndLogoutRemainsRevoked(t *testing.T) {
	a := setup(t)
	a.user(t, "member", false)
	cookies := a.login(t, "member")
	ctx := context.Background()
	if _, err := a.pool.Exec(ctx, "ALTER TABLE auth.authentication_events ADD CONSTRAINT auth_test_deny CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.pool.Exec(ctx, "ALTER TABLE auth.authentication_events DROP CONSTRAINT IF EXISTS auth_test_deny")
	})
	w := a.request("POST", "/api/v1/sessions", map[string]string{"account": "member", "password": "Aa1!"}, nil, nil)
	checkStatus(t, w, 503)
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("audit failure must not issue login Cookie")
	}
	keys, err := a.client.Keys(ctx, "ems:auth:session:auth-http-tests:v1:*").Result()
	if err != nil || len(keys) != 1 {
		t.Fatal("failed login left a usable new Session")
	}
	checkStatus(t, a.request("DELETE", "/api/v1/sessions/current", nil, cookies, nil), 204)
	checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 401)
	if !strings.Contains(a.logs.String(), "audit") {
		t.Fatal("audit failure must alert without secrets")
	}
}
func TestReadinessAndAuditPrivacyHTTP(t *testing.T) {
	a := setup(t)
	id := a.user(t, "member", false)
	cookies := a.login(t, "member")
	if err := a.service.Ready(context.Background()); err != nil {
		t.Fatal("real healthy dependencies must be ready")
	}
	checkStatus(t, a.request("POST", "/api/v1/sessions", map[string]string{"account": "unknown-sensitive", "password": "DoNotLog@123"}, nil, map[string]string{"X-Forwarded-For": "198.51.100.9"}), 401)
	var fingerprint, ip, ua string
	var count int
	err := a.pool.QueryRow(context.Background(), "SELECT account_fingerprint,client_ip::text,user_agent FROM auth.authentication_events WHERE event_type='login' AND subject_user_id IS NULL ORDER BY occurred_at DESC LIMIT 1").Scan(&fingerprint, &ip, &ua)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fingerprint, "test:") || len(fingerprint) != 69 || !strings.HasPrefix(ip, "192.0.2.7") || ua != "test-browser" {
		t.Fatal("audit identity/IP/device semantics incorrect")
	}
	a.pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE event_type='login' AND outcome='success' AND subject_user_id=$1 AND session_ref IS NOT NULL", id).Scan(&count)
	if count != 1 {
		t.Fatal("success audit must have independent Session reference")
	}
	if _, err := a.pool.Exec(context.Background(), "UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 401)
	checkStatus(t, a.request("POST", "/api/v1/sessions", map[string]string{"account": "member", "password": "Aa1!"}, nil, nil), 401)
	var auditJSON string
	a.pool.QueryRow(context.Background(), "SELECT json_agg(e)::text FROM auth.authentication_events e").Scan(&auditJSON)
	for _, secret := range []string{"Aa1!", "DoNotLog@123", "unknown-sensitive", cookies[0].Value, cookies[1].Value} {
		if strings.Contains(auditJSON+a.logs.String(), secret) {
			t.Fatal("audit leaked authentication input")
		}
	}
	a.pool.Close()
	if a.service.Ready(context.Background()) == nil {
		t.Fatal("closed PostgreSQL must not be ready")
	}
}
