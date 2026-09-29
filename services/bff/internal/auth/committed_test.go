package auth

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type commitFaultKey struct{}

func redisTransportFault(t *testing.T, rawURL string) (string, func()) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	target := u.Host
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	u.Host = listener.Addr().String()
	var mu sync.Mutex
	var connections []net.Conn
	var stopped bool
	stop := func() {
		mu.Lock()
		defer mu.Unlock()
		stopped = true
		_ = listener.Close()
		for _, c := range connections {
			_ = c.Close()
		}
	}
	t.Cleanup(stop)
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			server, err := net.Dial("tcp", target)
			if err != nil {
				_ = client.Close()
				continue
			}
			mu.Lock()
			if stopped {
				_ = client.Close()
				_ = server.Close()
				mu.Unlock()
				return
			}
			connections = append(connections, client, server)
			mu.Unlock()
			go func() { _, _ = io.Copy(server, client); _ = server.Close() }()
			go func() { _, _ = io.Copy(client, server); _ = client.Close() }()
		}
	}()
	return u.String(), stop
}

type commitFault struct {
	after    func()
	observed bool
}

func (f *commitFault) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, commitFaultKey{}, strings.EqualFold(strings.TrimSpace(d.SQL), "commit"))
}
func (f *commitFault) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if commit, _ := ctx.Value(commitFaultKey{}).(bool); commit && d.Err == nil {
		f.observed = true
		f.after()
	}
}

// PRD FR-04 / ADR-005 D8 (Q19): a real committed write must retain its result.
// The pgx tracer injects the Redis fault after PostgreSQL acknowledges COMMIT;
// neither the application, transaction, nor Redis renewal is mocked.
func TestCommittedWriteSurvivesRenewalFailure(t *testing.T) {
	for _, operation := range []string{"invitation", "reset"} {
		for _, fault := range []string{"revoked", "disconnected"} {
			t.Run(operation+"/"+fault, func(t *testing.T) {
				a := setup(t)
				var disconnect func()
				if fault == "disconnected" {
					proxied, stop := redisTransportFault(t, os.Getenv("WEAVEOS_TEST_REDIS_URL"))
					_ = a.service.Sessions.Close()
					a.service.Sessions = session.NewStore(proxied, "auth-http-tests")
					t.Cleanup(func() { _ = a.service.Sessions.Close() })
					disconnect = stop
				}
				a.user(t, "admin", true)
				target := a.user(t, "member", false)
				member := a.login(t, "member")
				cookies := a.login(t, "admin")
				var sid string
				for _, c := range cookies {
					if c.Name == session.SessionCookieName {
						sid = c.Value
					}
				}
				barrier := &commitFault{after: func() {
					if fault == "revoked" {
						ok, err := a.service.Sessions.Revoke(context.Background(), sid)
						if err != nil || !ok {
							t.Fatal("failed to revoke real session at commit barrier")
						}
					} else {
						disconnect()
					}
				}}
				cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
				if err != nil {
					t.Fatal(err)
				}
				cfg.ConnConfig.Tracer = barrier
				pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer pool.Close()
				a.service.Pool = pool
				path, want := "/api/v1/invitations", http.StatusCreated
				if operation == "reset" {
					path, want = "/api/v1/users/"+target+"/password-reset", http.StatusOK
				}
				w := a.request("POST", path, map[string]string{}, cookies, nil)
				if !barrier.observed {
					t.Fatal("real PostgreSQL COMMIT was not acknowledged")
				}
				var count int
				if operation == "invitation" {
					err = a.pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.invitations").Scan(&count)
				} else {
					err = a.pool.QueryRow(context.Background(), "SELECT auth_version-1 FROM auth.users WHERE id=$1", target).Scan(&count)
				}
				if err != nil || count != 1 {
					t.Fatal("business result was not committed exactly once")
				}
				checkStatus(t, w, want)
				cleared := map[string]bool{}
				for _, c := range w.Result().Cookies() {
					cleared[c.Name] = c.MaxAge < 0 && c.Value == "" && c.Secure
				}
				if !cleared[session.SessionCookieName] || !cleared[session.CSRFCookieName] {
					t.Error("both browser cookies must be cleared without renewal")
				}
				if !strings.Contains(a.logs.String(), "session renewal failed after committed write") {
					t.Error("missing post-commit diagnostic")
				}
				for _, c := range cookies {
					if strings.Contains(a.logs.String(), c.Value) {
						t.Error("diagnostic leaked a cookie")
					}
				}
				if operation == "invitation" {
					var body struct {
						Data struct {
							ID   string `json:"id"`
							Code string `json:"invitationCode"`
						}
					}
					if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Data.Code == "" || w.Header().Get("Location") != "/api/v1/invitations/"+body.Data.ID {
						t.Fatal("committed invitation was not delivered")
					}
					if strings.Contains(a.logs.String(), body.Data.Code) {
						t.Error("diagnostic leaked invitation")
					}
				}
				if fault == "disconnected" {
					checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 503)
				} else {
					checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 401)
					if operation == "reset" {
						checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, member, nil), 401)
					}
				}
			})
		}
	}
}
