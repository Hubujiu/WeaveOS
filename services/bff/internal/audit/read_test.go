package audit

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func readerFixture(t *testing.T, l *pgxpool.Pool) (*Reader, *http.Request) {
	t.Helper()
	ctx := context.Background()
	_, err := l.Exec(ctx, "INSERT INTO auth.users(id,account,is_bootstrap_admin) VALUES ('20000000-0000-4000-8000-000000000001','SyntheticAdmin',true)")
	if err != nil {
		t.Fatal(err)
	}
	store := session.NewStore(os.Getenv("WEAVEOS_TEST_REDIS_URL"), "audit_test")
	t.Cleanup(func() { _ = store.Close() })
	sid, _, err := store.Create(ctx, session.Record{UserID: "20000000-0000-4000-8000-000000000001", SessionRef: "30000000-0000-4000-8000-000000000001", AuthVersion: "1"})
	if err != nil {
		t.Fatal("isolated Redis unavailable")
	}
	t.Cleanup(func() { _, _ = store.Revoke(ctx, sid) })
	request := httptest.NewRequest("GET", "https://localhost/audit-cli", nil)
	request.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
	return &Reader{Authentication: &session.Authenticator{Sessions: store, DB: l, Origin: "https://localhost"}, Live: l}, request
}

func TestAuditReadDeniesOrdinaryDisabledStaleAndAnonymous(t *testing.T) {
	l, _ := clean(t)
	reader, request := readerFixture(t, l)
	for _, query := range []string{
		"UPDATE auth.users SET is_bootstrap_admin=false",
		"UPDATE auth.users SET is_bootstrap_admin=true,status='disabled'",
		"UPDATE auth.users SET status='active',auth_version=auth_version+1",
	} {
		if _, err := l.Exec(context.Background(), query); err != nil {
			t.Fatal(err)
		}
		if events, err := reader.List(request, 10); err == nil || len(events) != 0 {
			t.Error("unqualified current identity must not read audit")
		}
	}
	anonymous := httptest.NewRequest("GET", "https://localhost/audit-cli", nil)
	if events, err := reader.List(anonymous, 10); err == nil || len(events) != 0 {
		t.Error("anonymous must not read audit")
	}
}
