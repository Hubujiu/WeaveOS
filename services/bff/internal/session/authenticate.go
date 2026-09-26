package session

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
)

var ErrUnauthorized = errors.New("unauthenticated")
var ErrForbidden = errors.New("request forbidden")
var ErrUnavailable = errors.New("authentication dependency unavailable")

type Authenticator struct {
	Sessions *Store
	DB       authsql.DBTX
	Origin   string
}
type Principal struct {
	UserID, Account, SessionRef string
	BootstrapAdmin              bool
	Record                      Record
	SID                         string
}

func (a *Authenticator) Authenticate(r *http.Request, write bool) (Principal, error) {
	if a == nil || a.Sessions == nil || a.DB == nil {
		return Principal{}, ErrUnavailable
	}
	var sid string
	count := 0
	for _, c := range r.Cookies() {
		if c.Name == SessionCookieName {
			sid = c.Value
			count++
		}
	}
	if count != 1 {
		return Principal{}, ErrUnauthorized
	}
	record, err := a.Sessions.Load(r.Context(), sid)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, ErrUnavailable
	}
	var id pgtype.UUID
	if err := id.Scan(record.UserID); err != nil {
		return Principal{}, ErrUnauthorized
	}
	user, err := authsql.New(a.DB).GetCurrentUser(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, ErrUnavailable
	}
	if !security.CurrentUserMatchesSession(record.AuthVersion, user.Status, user.AuthVersion) {
		return Principal{}, ErrUnauthorized
	}
	if write && (!security.ValidSource(r, a.Origin) || !security.ValidCSRF(r, record.CSRFTokenHash)) {
		return Principal{}, ErrForbidden
	}
	return Principal{UserID: user.ID.String(), Account: user.Account, SessionRef: record.SessionRef, BootstrapAdmin: user.IsBootstrapAdmin, Record: record, SID: sid}, nil
}
func (a *Authenticator) Renew(ctx context.Context, w http.ResponseWriter, r *http.Request, p Principal) error {
	if a == nil || a.Sessions == nil {
		return ErrUnavailable
	}
	touched, err := a.Sessions.Touch(ctx, p.SID, p.Record)
	if err != nil {
		return ErrUnavailable
	}
	if !touched {
		return ErrUnauthorized
	}
	var csrf string
	count := 0
	for _, c := range r.Cookies() {
		if c.Name == CSRFCookieName {
			csrf = c.Value
			count++
		}
	}
	if count == 1 && security.MatchesCSRFToken(csrf, p.Record.CSRFTokenHash) {
		return IssueCookies(w, p.SID, csrf)
	}
	// Reads do not require CSRF. Only a bound CSRF Cookie may be renewed.
	http.SetCookie(w, cookie(SessionCookieName, p.SID, true, 3600))
	return nil
}
