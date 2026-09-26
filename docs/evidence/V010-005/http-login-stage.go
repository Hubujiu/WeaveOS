package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type Service struct {
	Pool               *pgxpool.Pool
	Sessions           *session.Store
	Origin, AuditKeyID string
	AuditKey           []byte
	Logger             *slog.Logger
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/sessions" && r.Method == "POST" {
		s.login(w, r)
		return
	}
	if r.URL.Path == "/api/v1/sessions/current" {
		a := session.Authenticator{Sessions: s.Sessions, DB: s.Pool, Origin: s.Origin}
		_, err := a.Authenticate(r, r.Method == "DELETE")
		if err != nil {
			s.fail(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNotImplemented)
}
func (s *Service) Ready(context.Context) error { return errors.New("unwired") }

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
	if err := session.IssueCookies(w, sid, csrf); err != nil {
		s.fail(w, r, err)
		return
	}
	reply(w, r, 201, "OK", map[string]string{"id": user.ID.String(), "account": user.Account})
}
