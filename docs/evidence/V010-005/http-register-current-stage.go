package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
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

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			s.fail(w, r, err)
			return
		}
		if r.Method == "GET" || r.Method == "HEAD" {
			if err := a.Renew(r.Context(), w, r, p); err != nil {
				s.fail(w, r, err)
				return
			}
			reply(w, r, 200, "OK", map[string]string{"id": p.UserID, "account": p.Account})
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
	}
	w.WriteHeader(http.StatusNotImplemented)
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
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.Trim(in.Account, " ") == "" || in.Password == "" {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return
	}
	q := authsql.New(s.Pool)
	user, err := q.GetLoginRecord(r.Context(), strings.Trim(in.Account, " "))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!verifyPassword(in.Password, user.PasswordHash) || user.Status != "active") {
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
	if json.NewDecoder(r.Body).Decode(&in) != nil || !accountValid(strings.Trim(in.Account, " ")) || !validPassword(in.Password) {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(in.Invitation)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != in.Invitation {
		reply(w, r, 400, "INVITATION_INVALID", nil)
		return
	}
	digest := sha256.Sum256(raw)
	hash, err := hashPassword(in.Password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	user, err := persistence.New(s.Pool).Register(r.Context(), persistence.RegistrationInput{Account: in.Account, PasswordHash: hash, InvitationDigest: digest[:], ClientIP: host, UserAgent: r.UserAgent(), RequestID: w.Header().Get("X-Request-Id")})
	switch {
	case errors.Is(err, persistence.ErrAccountTaken):
		reply(w, r, 409, "USER_ACCOUNT_ALREADY_EXISTS", nil)
	case errors.Is(err, persistence.ErrInvitationUnavailable):
		reply(w, r, 400, "INVITATION_INVALID", nil)
	case err != nil:
		s.fail(w, r, err)
	default:
		w.Header().Set("Location", "/api/v1/users/"+user.ID)
		reply(w, r, 201, "OK", map[string]string{"id": user.ID, "account": user.Account})
	}
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
	return authsql.New(s.Pool).AppendAuthEvent(r.Context(), authsql.AppendAuthEventParams{EventType: kind, Outcome: outcome, ActorUserID: uuidValue(actor), SubjectUserID: uuidValue(subject), AccountFingerprint: textValue(fingerprint), ClientIp: ip, UserAgent: textValue(ua), SessionRef: uuidValue(ref), ReasonCode: textValue(reason), RequestID: "independent-test-request"})
}
