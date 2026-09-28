package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Application is the in-process authentication boundary, independent of HTTP.
// This initial declaration deliberately has no behavior: tests must first fail.
type Application struct {
	Pool       *pgxpool.Pool
	Sessions   *session.Store
	AuditKeyID string
	AuditKey   []byte
	Logger     *slog.Logger
}

type RequestMetadata struct {
	ClientIP  string
	UserAgent string
	RequestID string
}

type UserResult struct{ ID, Account string }
type LoginResult struct {
	User      UserResult
	SID, CSRF string
}
type InvitationResult struct{ ID, Code string }

type Failure struct{ Code string }

func (e *Failure) Error() string { return e.Code }

func (a *Application) Register(ctx context.Context, account, password, code string, meta RequestMetadata) (UserResult, error) {
	return UserResult{}, errors.New("authentication application not implemented")
}
func (a *Application) Login(ctx context.Context, account, password string, meta RequestMetadata) (LoginResult, error) {
	return LoginResult{}, errors.New("authentication application not implemented")
}
func (a *Application) CreateInvitation(ctx context.Context, actor session.Principal, meta RequestMetadata) (InvitationResult, error) {
	return InvitationResult{}, errors.New("authentication application not implemented")
}
func (a *Application) ResetPassword(ctx context.Context, actor session.Principal, target string, meta RequestMetadata) error {
	return errors.New("authentication application not implemented")
}
