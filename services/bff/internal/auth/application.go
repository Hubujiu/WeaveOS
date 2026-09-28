package auth

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/invitation"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Application owns the local authentication use cases. It never reads HTTP
// requests or writes responses; the Web adapter translates those separately.
type Application struct {
	Pool       *pgxpool.Pool
	Sessions   *session.Store
	AuditKeyID string
	AuditKey   []byte
	Logger     *slog.Logger
}

// RequestMetadata contains audit facts supplied by the trusted Web boundary.
type RequestMetadata struct {
	ClientIP  string
	UserAgent string
	RequestID string
}

type UserResult struct{ ID, Account string }

// LoginResult is internal session material, not a response DTO. The Web adapter
// issues SID and CSRF through cookies and only serializes User.
type LoginResult struct {
	User      UserResult
	SID, CSRF string
}

type InvitationResult struct{ ID, Code string }

// Failure carries public application semantics without choosing an HTTP status.
// Underlying database errors never become its public text.
type Failure struct {
	Code       string
	Violations []violation
}

func (e *Failure) Error() string { return e.Code }

func (a *Application) Register(ctx context.Context, account, password, code string, meta RequestMetadata) (UserResult, error) {
	if err := validateCredentials(account, password, &code); err != nil {
		return UserResult{}, err
	}
	digest, err := invitation.Digest(code)
	if err != nil {
		return UserResult{}, &Failure{Code: "INVITATION_INVALID"}
	}
	hash, err := hashPassword(password)
	if err != nil {
		return UserResult{}, err
	}
	user, err := persistence.New(a.Pool).Register(ctx, persistence.RegistrationInput{
		Account: account, PasswordHash: hash, InvitationDigest: digest[:],
		ClientIP: meta.ClientIP, UserAgent: meta.UserAgent, RequestID: meta.RequestID,
	})
	if err == nil {
		return UserResult{ID: user.ID, Account: user.Account}, nil
	}
	failure := registrationFailure(err)
	outcome := "error"
	if failure.Code == "USER_ACCOUNT_ALREADY_EXISTS" || failure.Code == "INVITATION_ALREADY_USED" || failure.Code == "INVITATION_INVALID" {
		outcome = "failure"
	}
	subject, fingerprint := "", ""
	if existing, lookupErr := authsql.New(a.Pool).GetLoginRecord(ctx, strings.Trim(account, " ")); lookupErr == nil {
		subject = existing.ID.String()
	} else {
		fingerprint = a.accountFingerprint(account)
	}
	if auditErr := a.event(ctx, meta, "register", outcome, "", subject, "", failure.Code, fingerprint); auditErr != nil {
		a.alert()
		return UserResult{}, &Failure{Code: "COMMON_SERVICE_UNAVAILABLE"}
	}
	return UserResult{}, failure
}

func registrationFailure(err error) *Failure {
	code := "COMMON_SERVICE_UNAVAILABLE"
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && len(databaseError.Code) >= 2 {
		switch databaseError.Code[:2] {
		case "08", "28", "53", "57", "58":
		default:
			code = "COMMON_INTERNAL_ERROR"
		}
	}
	switch {
	case errors.Is(err, persistence.ErrAccountTaken):
		code = "USER_ACCOUNT_ALREADY_EXISTS"
	case errors.Is(err, persistence.ErrInvitationUsed):
		code = "INVITATION_ALREADY_USED"
	case errors.Is(err, persistence.ErrInvitationUnavailable):
		code = "INVITATION_INVALID"
	}
	return &Failure{Code: code}
}

func (a *Application) Login(ctx context.Context, account, password string, meta RequestMetadata) (LoginResult, error) {
	if err := validateCredentials(account, password, nil); err != nil {
		return LoginResult{}, err
	}
	q := authsql.New(a.Pool)
	user, err := q.GetLoginRecord(ctx, strings.Trim(account, " "))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!verifyPassword(password, user.PasswordHash) || user.Status != "active") {
		subject, fingerprint := "", ""
		if err == nil {
			subject = user.ID.String()
		} else {
			fingerprint = a.accountFingerprint(account)
		}
		if err := a.event(ctx, meta, "login", "failure", "", subject, "", "AUTH_INVALID_CREDENTIALS", fingerprint); err != nil {
			a.alert()
			return LoginResult{}, err
		}
		return LoginResult{}, &Failure{Code: "AUTH_INVALID_CREDENTIALS"}
	}
	if err != nil {
		return LoginResult{}, err
	}
	current, err := q.GetCurrentUser(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if current.Status != "active" || current.AuthVersion != user.AuthVersion {
		return LoginResult{}, &Failure{Code: "AUTH_INVALID_CREDENTIALS"}
	}
	ref, err := uuid()
	if err != nil {
		return LoginResult{}, err
	}
	sid, csrf, err := a.Sessions.Create(ctx, session.Record{UserID: user.ID.String(), SessionRef: ref, AuthVersion: strconv.FormatInt(user.AuthVersion, 10)})
	if err != nil {
		return LoginResult{}, err
	}
	if err := a.event(ctx, meta, "login", "success", "", user.ID.String(), ref, "", ""); err != nil {
		_, _ = a.Sessions.Revoke(context.Background(), sid)
		a.alert()
		return LoginResult{}, err
	}
	return LoginResult{User: UserResult{ID: user.ID.String(), Account: user.Account}, SID: sid, CSRF: csrf}, nil
}
