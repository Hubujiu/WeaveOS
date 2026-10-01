package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service is the existing Web adapter/composition entry point. Its field-based
// setup remains compatible with the host and integration fixtures. Business
// operations live in Application, not in its HTTP methods.
type Service struct {
	Pool                 *pgxpool.Pool
	Sessions             *session.Store
	Origin, AuditKeyID   string
	AuditKey             []byte
	Logger               *slog.Logger
	TrustedProxyHosts    []string
	Personnel            http.Handler
	InvitationAuthorizer func(context.Context, pgx.Tx, session.Principal) error
	InvitationBegin      func(context.Context, session.Principal, string) (pgx.Tx, error)
}

func (s *Service) application() *Application {
	return &Application{Pool: s.Pool, Sessions: s.Sessions, AuditKeyID: s.AuditKeyID, AuditKey: s.AuditKey, Logger: s.Logger, InvitationAuthorizer: s.InvitationAuthorizer, InvitationBegin: s.InvitationBegin}
}

func (s *Service) authenticator() session.Authenticator {
	return session.Authenticator{Sessions: s.Sessions, DB: s.Pool, Origin: s.Origin}
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
