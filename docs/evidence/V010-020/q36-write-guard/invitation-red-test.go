package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

func TestQ36InvitationOptionalQueryContext(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.user(t, "query-admin", true)
	target := f.user(t, "query-target", false)
	cookies := f.login(t, "query-admin")
	people := &personnel.Application{Pool: f.pool, Queries: personnel.NewQueryContextStore(f.client, "invitation-query-tests")}
	f.service.Personnel = &personnel.Service{Application: people, Authenticator: f.service.authenticator()}
	f.service.InvitationBegin = func(ctx context.Context, p session.Principal, qv string) (pgx.Tx, error) {
		tx, err := people.BeginQueryWrite(ctx, p, qv)
		for _, entry := range []struct {
			err  error
			code string
		}{{personnel.ErrQueryChanged, "COMMON_QUERY_CHANGED"}, {personnel.ErrQueryContextExpired, "COMMON_QUERY_CONTEXT_EXPIRED"}, {personnel.ErrDenied, "COMMON_PERMISSION_DENIED"}} {
			if errors.Is(err, entry.err) {
				return nil, &Failure{Code: entry.code}
			}
		}
		return tx, err
	}
	baseline := f.request("POST", "/api/v1/personnel/members/search", map[string]any{"search": "query-target"}, cookies, nil)
	checkStatus(t, baseline, 200)
	var b struct{ Data struct{ QueryVersion string } }
	if err := json.Unmarshal(baseline.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	// Independent invitation and an unrelated user change do not invalidate original target query.
	checkStatus(t, f.request("POST", "/api/v1/invitations", map[string]any{}, cookies, nil), 201)
	f.user(t, "unrelated-user", false)
	checkStatus(t, f.request("POST", "/api/v1/invitations", map[string]any{"queryVersion": b.Data.QueryVersion}, cookies, nil), 201)
	if _, err := f.pool.Exec(ctx, "UPDATE auth.users SET account='query-renamed' WHERE id=$1", target); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct{ qv, code string }{{b.Data.QueryVersion, "COMMON_QUERY_CHANGED"}, {strings.Repeat("x", 43), "COMMON_QUERY_CONTEXT_EXPIRED"}} {
		w := f.request("POST", "/api/v1/invitations", map[string]any{"queryVersion": v.qv}, cookies, nil)
		checkStatus(t, w, 409)
		if !strings.Contains(w.Body.String(), v.code) || strings.Contains(w.Body.String(), "invitationCode") {
			t.Fatal("exact context error, no invitation material")
		}
	}
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM auth.invitations").Scan(&count); err != nil || count != 2 {
		t.Fatal("rejected query must not insert invitation")
	}
	checkStatus(t, f.request("POST", "/api/v1/invitations", map[string]any{"queryVersion": ""}, cookies, nil), 400)
}
