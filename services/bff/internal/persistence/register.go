package persistence

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvitationUnavailable = errors.New("invitation unavailable")
	ErrInvitationUsed        = fmt.Errorf("%w: already used", ErrInvitationUnavailable)
	ErrAccountTaken          = errors.New("account taken")
	ErrInvalidAccount        = errors.New("invalid account")
)

type User struct {
	ID      string
	Account string
}

type RegistrationInput struct {
	Account          string
	PasswordHash     string
	InvitationDigest []byte
	ClientIP         string
	UserAgent        string
	RequestID        string
}

type Store struct{ Pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

func (s *Store) Register(ctx context.Context, in RegistrationInput) (User, error) {
	account := strings.Trim(in.Account, " ")
	if account == "" || !utf8.ValidString(account) || utf8.RuneCountInString(account) > 254 || strings.Contains(account, " ") || strings.IndexFunc(account, unicode.IsControl) >= 0 {
		return User{}, ErrInvalidAccount
	}
	if len(in.InvitationDigest) != 32 || in.PasswordHash == "" || in.RequestID == "" || len(in.RequestID) > 128 {
		return User{}, errors.New("invalid registration storage input")
	}
	var clientIP *netip.Addr
	if in.ClientIP != "" {
		ip, err := netip.ParseAddr(in.ClientIP)
		if err != nil {
			return User{}, err
		}
		clientIP = &ip
	}
	userAgent := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, in.UserAgent)
	if utf8.RuneCountInString(userAgent) > 2048 {
		userAgent = string([]rune(userAgent)[:2048])
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err != nil {
		return User{}, err
	}
	queries := authsql.New(tx)
	inv, err := queries.LockInvitation(ctx, in.InvitationDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvitationUnavailable
	}
	if err != nil {
		return User{}, err
	}
	if inv.UsedBy.Valid {
		return User{}, ErrInvitationUsed
	}

	created, err := queries.CreateUser(ctx, account)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_users_account_key" {
			return User{}, ErrAccountTaken
		}
		return User{}, err
	}
	if err := queries.CreateCredential(ctx, authsql.CreateCredentialParams{UserID: created.ID, PasswordHash: in.PasswordHash}); err != nil {
		return User{}, err
	}
	if _, err := queries.ConsumeInvitation(ctx, authsql.ConsumeInvitationParams{ID: inv.ID, UsedBy: created.ID}); err != nil {
		return User{}, err
	}
	if err := queries.AppendRegistrationEvent(ctx, authsql.AppendRegistrationEventParams{
		SubjectUserID: created.ID, ClientIp: clientIP,
		UserAgent: pgtype.Text{String: userAgent, Valid: userAgent != ""}, RequestID: in.RequestID,
	}); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return User{ID: created.ID.String(), Account: created.Account}, nil
}
