package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/identity"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func (s *Service) TrustedProxies() []string { return append([]string(nil), s.TrustedProxyHosts...) }

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	r, err = httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if err != nil {
		http.Error(w, "request initialization failed", http.StatusServiceUnavailable)
		return
	}
	if s.Personnel != nil && (r.URL.Path == "/api/v1/me/access" || strings.HasPrefix(r.URL.Path, "/api/v1/personnel/")) {
		s.Personnel.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/registrations" && r.Method == "POST" {
		s.register(w, r)
		return
	}
	if r.URL.Path == "/api/v1/sessions" && r.Method == "POST" {
		s.login(w, r)
		return
	}
	if r.URL.Path == "/api/v1/sessions/current" {
		a := s.authenticator()
		p, err := a.Authenticate(r, r.Method == "DELETE")
		if err != nil {
			if errors.Is(err, session.ErrUnauthorized) {
				if _, cookieErr := r.Cookie(session.SessionCookieName); cookieErr == nil {
					s.recordEvent(r, "session_invalid", "failure", "", "", "", "AUTH_UNAUTHENTICATED", "")
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
			s.recordEvent(r, "logout", "success", p.UserID, p.UserID, p.SessionRef, "", "")
			reply(w, r, 204, "OK", nil)
			return
		}
	}
	if r.URL.Path == "/api/v1/invitations" || strings.HasPrefix(r.URL.Path, "/api/v1/users/") {
		a := s.authenticator()
		p, err := a.Authenticate(r, true)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if r.URL.Path != "/api/v1/invitations" && !p.BootstrapAdmin {
			reply(w, r, 403, "COMMON_PERMISSION_DENIED", nil)
			return
		}
		if r.URL.Path == "/api/v1/invitations" && r.Method == "POST" {
			var in struct {
				QueryVersion json.RawMessage `json:"queryVersion"`
			}
			if !decode(w, r, &in) {
				return
			}
			version := ""
			if in.QueryVersion != nil && (string(in.QueryVersion) == "null" || json.Unmarshal(in.QueryVersion, &version) != nil || version == "") {
				reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
				return
			}
			s.createInvitation(w, r, p, version)
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

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	if !security.ValidSource(r, s.Origin) {
		s.fail(w, r, session.ErrForbidden)
		return
	}
	var in struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in, "account", "password") {
		return
	}
	result, err := s.application().Login(r.Context(), in.Account, in.Password, s.requestMetadata(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := session.IssueCookies(w, result.SID, result.CSRF); err != nil {
		s.fail(w, r, err)
		return
	}
	reply(w, r, 201, "OK", map[string]string{"id": result.User.ID, "account": result.User.Account})
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
	if !decode(w, r, &in, "account", "password", "invitationCode") {
		return
	}
	user, err := s.application().Register(r.Context(), in.Account, in.Password, in.Invitation, s.requestMetadata(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+user.ID)
	reply(w, r, 201, "OK", map[string]string{"id": user.ID, "account": user.Account})
}

func (s *Service) createInvitation(w http.ResponseWriter, r *http.Request, p session.Principal, version string) {
	meta := s.requestMetadata(r)
	meta.QueryVersion = version
	result, err := s.application().CreateInvitation(r.Context(), p, meta)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renewCommitted(w, r, p)
	w.Header().Set("Location", "/api/v1/invitations/"+result.ID)
	reply(w, r, 201, "OK", map[string]string{"id": result.ID, "invitationCode": result.Code})
}

func (s *Service) reset(w http.ResponseWriter, r *http.Request, p session.Principal) {
	target := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/password-reset")
	if err := s.application().ResetPassword(r.Context(), p, target, s.requestMetadata(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	if p.UserID == target {
		session.ClearCookies(w)
		_, _ = s.Sessions.Revoke(r.Context(), p.SID)
	} else {
		s.renewCommitted(w, r, p)
	}
	reply(w, r, 200, "OK", nil)
}

// Only called after an authorized write has acknowledged its database commit.
// Clearing browser cookies does not claim that Redis has revoked the session.
func (s *Service) renewCommitted(w http.ResponseWriter, r *http.Request, p session.Principal) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	a := s.authenticator()
	if err := a.Renew(ctx, w, r, p); err != nil {
		session.ClearCookies(w)
		if s.Logger != nil {
			s.Logger.Warn("session renewal failed after committed write", "request_id", httpserver.Metadata(r.Context()).RequestID)
		}
	}
}

func (s *Service) requestMetadata(r *http.Request) RequestMetadata {
	m := httpserver.Metadata(r.Context())
	return RequestMetadata{ClientIP: m.ClientIP, UserAgent: m.UserAgent, RequestID: m.RequestID}
}

// These audit events cannot undo an already completed logout or turn an invalid
// session into a valid one. Persistent audit work belongs to Application.
func (s *Service) recordEvent(r *http.Request, kind, outcome, actor, subject, ref, reason, fingerprint string) {
	app := s.application()
	if err := app.event(r.Context(), s.requestMetadata(r), kind, outcome, actor, subject, ref, reason, fingerprint); err != nil {
		app.alert()
	}
}
