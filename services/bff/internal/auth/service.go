package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/identity"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/invitation"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Service struct {
	Pool               *pgxpool.Pool
	Sessions           *session.Store
	Origin, AuditKeyID string
	AuditKey           []byte
	Logger             *slog.Logger
}

type requestIDKey struct{}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, w.Header().Get("X-Request-Id")))
	if r.URL.Path == "/api/v1/registrations" && r.Method == "POST" {
		s.register(w, r)
		return
	}
	if r.URL.Path == "/api/v1/sessions" && r.Method == "POST" {
		s.login(w, r)
		return
	}
	if r.URL.Path == "/api/v1/sessions/current" {
		a := session.Authenticator{Sessions: s.Sessions, DB: s.Pool, Origin: s.Origin}
		p, err := a.Authenticate(r, r.Method == "DELETE")
		if err != nil {
			if errors.Is(err, session.ErrUnauthorized) {
				if _, cookieErr := r.Cookie(session.SessionCookieName); cookieErr == nil {
					if s.event(r, "session_invalid", "failure", "", "", "", "AUTH_UNAUTHENTICATED", "") != nil {
						s.alert()
					}
				}
			}
			s.fail(w, r, err)
			return
		}
		ctx, err := identity.WithTrusted(r.Context(), identity.Context{SubjectID: p.UserID, SessionID: p.SessionRef})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		r = r.WithContext(ctx)
		if r.Method == "GET" || r.Method == "HEAD" {
			if err := a.Renew(r.Context(), w, r, p); err != nil {
				s.fail(w, r, err)
				return
			}
			reply(w, r, 200, "OK", map[string]string{"id": p.UserID, "account": p.Account})
			return
		}
		if r.Method == "DELETE" {
			if _, err := s.Sessions.Revoke(r.Context(), p.SID); err != nil {
				s.fail(w, r, err)
				return
			}
			session.ClearCookies(w)
			if s.event(r, "logout", "success", p.UserID, p.UserID, p.SessionRef, "", "") != nil {
				s.alert()
			}
			reply(w, r, 204, "OK", nil)
			return
		}
	}
	if r.URL.Path == "/api/v1/invitations" || strings.HasPrefix(r.URL.Path, "/api/v1/users/") {
		a := session.Authenticator{Sessions: s.Sessions, DB: s.Pool, Origin: s.Origin}
		p, err := a.Authenticate(r, true)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if !p.BootstrapAdmin {
			reply(w, r, 403, "COMMON_PERMISSION_DENIED", nil)
			return
		}
		if r.URL.Path == "/api/v1/invitations" && r.Method == "POST" {
			if !decode(w, r, &struct{}{}) {
				return
			}
			s.createInvitation(w, r, p)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/users/") && strings.HasSuffix(r.URL.Path, "/password-reset") && r.Method == "POST" {
			if !decode(w, r, &struct{}{}) {
				return
			}
			s.reset(w, r, p)
			return
		}
	}
	reply(w, r, 404, "API_NOT_FOUND", nil)
}
func (s *Service) Ready(ctx context.Context) error {
	if s == nil || s.Pool == nil || s.Sessions == nil {
		return errors.New("unwired")
	}
	if err := s.Pool.Ping(ctx); err != nil {
		return err
	}
	return s.Sessions.Ping(ctx)
}

func reply(w http.ResponseWriter, r *http.Request, status int, code string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method == "HEAD" || status == 204 {
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{code, code, data, map[string]string{"requestId": w.Header().Get("X-Request-Id")}})
}
func (s *Service) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, code := 503, "COMMON_SERVICE_UNAVAILABLE"
	if errors.Is(err, session.ErrUnauthorized) {
		status, code = 401, "AUTH_UNAUTHENTICATED"
	}
	if errors.Is(err, session.ErrForbidden) {
		status, code = 403, "COMMON_CSRF_REJECTED"
	}
	reply(w, r, status, code, nil)
}
func uuid() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	if !security.ValidSource(r, s.Origin) {
		s.fail(w, r, session.ErrForbidden)
		return
	}
	var in struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !accountValid(strings.Trim(in.Account, " ")) || in.Password == "" {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return
	}
	q := authsql.New(s.Pool)
	user, err := q.GetLoginRecord(r.Context(), strings.Trim(in.Account, " "))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!verifyPassword(in.Password, user.PasswordHash) || user.Status != "active") {
		subject, fingerprint := "", ""
		if err == nil {
			subject = user.ID.String()
		} else {
			mac := hmac.New(sha256.New, s.AuditKey)
			_, _ = mac.Write([]byte(strings.Trim(in.Account, " ")))
			fingerprint = s.AuditKeyID + ":" + hex.EncodeToString(mac.Sum(nil))
		}
		if err := s.event(r, "login", "failure", "", subject, "", "AUTH_INVALID_CREDENTIALS", fingerprint); err != nil {
			s.alert()
			s.fail(w, r, err)
			return
		}
		reply(w, r, 401, "AUTH_INVALID_CREDENTIALS", nil)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	current, err := q.GetCurrentUser(r.Context(), user.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if current.Status != "active" || current.AuthVersion != user.AuthVersion {
		reply(w, r, 401, "AUTH_INVALID_CREDENTIALS", nil)
		return
	}
	ref, err := uuid()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	sid, csrf, err := s.Sessions.Create(r.Context(), session.Record{UserID: user.ID.String(), SessionRef: ref, AuthVersion: strconv.FormatInt(user.AuthVersion, 10)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.event(r, "login", "success", "", user.ID.String(), ref, "", ""); err != nil {
		_, _ = s.Sessions.Revoke(context.Background(), sid)
		s.alert()
		s.fail(w, r, err)
		return
	}
	if err := session.IssueCookies(w, sid, csrf); err != nil {
		s.fail(w, r, err)
		return
	}
	reply(w, r, 201, "OK", map[string]string{"id": user.ID.String(), "account": user.Account})
}

func accountValid(account string) bool {
	return account != "" && utf8.ValidString(account) && utf8.RuneCountInString(account) <= 254 && !strings.Contains(account, " ") && strings.IndexFunc(account, unicode.IsControl) < 0
}
func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	if !security.ValidSource(r, s.Origin) {
		s.fail(w, r, session.ErrForbidden)
		return
	}
	var in struct {
		Account    string `json:"account"`
		Password   string `json:"password"`
		Invitation string `json:"invitationCode"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !accountValid(strings.Trim(in.Account, " ")) || !validPassword(in.Password) {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return
	}
	digest, err := invitation.Digest(in.Invitation)
	if err != nil {
		reply(w, r, 400, "INVITATION_INVALID", nil)
		return
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	user, err := persistence.New(s.Pool).Register(r.Context(), persistence.RegistrationInput{Account: in.Account, PasswordHash: hash, InvitationDigest: digest[:], ClientIP: host, UserAgent: r.UserAgent(), RequestID: w.Header().Get("X-Request-Id")})
	if err != nil {
		status, code, outcome := 503, "COMMON_SERVICE_UNAVAILABLE", "error"
		switch {
		case errors.Is(err, persistence.ErrAccountTaken):
			status, code, outcome = 409, "USER_ACCOUNT_ALREADY_EXISTS", "failure"
		case errors.Is(err, persistence.ErrInvitationUsed):
			status, code, outcome = 409, "INVITATION_ALREADY_USED", "failure"
		case errors.Is(err, persistence.ErrInvitationUnavailable):
			status, code, outcome = 400, "INVITATION_INVALID", "failure"
		}
		account := strings.Trim(in.Account, " ")
		subject, fingerprint := "", ""
		if existing, e := authsql.New(s.Pool).GetLoginRecord(r.Context(), account); e == nil {
			subject = existing.ID.String()
		} else {
			mac := hmac.New(sha256.New, s.AuditKey)
			_, _ = mac.Write([]byte(account))
			fingerprint = s.AuditKeyID + ":" + hex.EncodeToString(mac.Sum(nil))
		}
		if s.event(r, "register", outcome, "", subject, "", code, fingerprint) != nil {
			s.alert()
			reply(w, r, 503, "COMMON_SERVICE_UNAVAILABLE", nil)
			return
		}
		reply(w, r, status, code, nil)
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+user.ID)
	reply(w, r, 201, "OK", map[string]string{"id": user.ID, "account": user.Account})
}

func (s *Service) createInvitation(w http.ResponseWriter, r *http.Request, p session.Principal) {
	code, err := invitation.Generate()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tx, err := s.Pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := authsql.New(tx)
	users, err := q.LockAdminUsers(r.Context(), []pgtype.UUID{uuidValue(p.UserID)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(users) != 1 || users[0].Status != "active" || !users[0].IsBootstrapAdmin || strconv.FormatInt(users[0].AuthVersion, 10) != p.Record.AuthVersion {
		reply(w, r, 403, "COMMON_PERMISSION_DENIED", nil)
		return
	}
	id, err := q.CreateAdminInvitation(r.Context(), authsql.CreateAdminInvitationParams{CodeHash: code.Digest[:], CreatedBy: uuidValue(p.UserID)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := q.AppendAuthEvent(r.Context(), s.eventParams(r, "invitation_created", "success", p.UserID, "", p.SessionRef, "", "")); err != nil {
		s.alert()
		s.fail(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		s.fail(w, r, err)
		return
	}
	a := session.Authenticator{Sessions: s.Sessions, DB: s.Pool, Origin: s.Origin}
	if err := a.Renew(r.Context(), w, r, p); err != nil {
		s.fail(w, r, err)
		return
	}
	reply(w, r, 201, "OK", map[string]string{"id": id.String(), "invitationCode": code.Value})
}
func (s *Service) alert() {
	if s.Logger != nil {
		s.Logger.Error("authentication audit persistence failed")
	}
}
func uuidValue(id string) pgtype.UUID {
	var v pgtype.UUID
	if id != "" {
		_ = v.Scan(id)
	}
	return v
}
func textValue(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func (s *Service) event(r *http.Request, kind, outcome, actor, subject, ref, reason, fingerprint string) error {
	return authsql.New(s.Pool).AppendAuthEvent(r.Context(), s.eventParams(r, kind, outcome, actor, subject, ref, reason, fingerprint))
}
func (s *Service) eventParams(r *http.Request, kind, outcome, actor, subject, ref, reason, fingerprint string) authsql.AppendAuthEventParams {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	var ip *netip.Addr
	if addr, err := netip.ParseAddr(host); err == nil {
		ip = &addr
	}
	ua := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, r.UserAgent())
	if utf8.RuneCountInString(ua) > 2048 {
		ua = string([]rune(ua)[:2048])
	}
	requestID, _ := r.Context().Value(requestIDKey{}).(string)
	return authsql.AppendAuthEventParams{EventType: kind, Outcome: outcome, ActorUserID: uuidValue(actor), SubjectUserID: uuidValue(subject), AccountFingerprint: textValue(fingerprint), ClientIp: ip, UserAgent: textValue(ua), SessionRef: uuidValue(ref), ReasonCode: textValue(reason), RequestID: requestID}
}

func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		reply(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return false
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	seen := map[string]bool{}
	for d.More() {
		k, err := d.Token()
		name, ok := k.(string)
		if err != nil || !ok || seen[name] {
			reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || string(value) == "null" {
			reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
			return false
		}
	}
	if _, err := d.Token(); err != nil {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	if _, err := d.Token(); err != io.EOF {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	return true
}

func (s *Service) reset(w http.ResponseWriter, r *http.Request, p session.Principal) {
	target := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/password-reset")
	id := uuidValue(target)
	if !id.Valid || id.String() != target {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return
	}
	hash, err := hashPassword("Abc@123456")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tx, err := s.Pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := authsql.New(tx)
	users, err := q.LockAdminUsers(r.Context(), []pgtype.UUID{uuidValue(p.UserID), id})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	actorValid, targetFound := false, false
	for _, u := range users {
		if u.ID.String() == p.UserID {
			actorValid = u.Status == "active" && u.IsBootstrapAdmin && strconv.FormatInt(u.AuthVersion, 10) == p.Record.AuthVersion
		}
		if u.ID.String() == target {
			targetFound = true
		}
	}
	if !actorValid {
		reply(w, r, 403, "COMMON_PERMISSION_DENIED", nil)
		return
	}
	if !targetFound {
		reply(w, r, 404, "USER_NOT_FOUND", nil)
		return
	}
	rows, err := q.UpdateResetCredential(r.Context(), authsql.UpdateResetCredentialParams{UserID: id, PasswordHash: hash})
	if err != nil || rows != 1 {
		s.fail(w, r, errors.New("credential mutation unavailable"))
		return
	}
	rows, err = q.BumpAuthVersion(r.Context(), id)
	if err != nil || rows != 1 {
		s.fail(w, r, errors.New("version mutation unavailable"))
		return
	}
	if err := q.AppendAuthEvent(r.Context(), s.eventParams(r, "password_reset", "success", p.UserID, target, p.SessionRef, "", "")); err != nil {
		s.alert()
		s.fail(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		s.fail(w, r, err)
		return
	}
	if p.UserID == target {
		session.ClearCookies(w)
		_, _ = s.Sessions.Revoke(r.Context(), p.SID)
	} else {
		a := session.Authenticator{Sessions: s.Sessions, DB: s.Pool, Origin: s.Origin}
		if err := a.Renew(r.Context(), w, r, p); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	reply(w, r, 200, "OK", nil)
}
