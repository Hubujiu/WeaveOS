package persistence

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrBootstrapCollision = errors.New("bootstrap account collision")

type BootstrapSeedInput struct {
	Account      string
	PasswordHash string
	RequestID    string
}

func (s *Store) SeedBootstrap(ctx context.Context, in BootstrapSeedInput) (User, error) {
	account := strings.Trim(in.Account, " ")
	if account == "" || !utf8.ValidString(account) || utf8.RuneCountInString(account) > 254 || strings.Contains(account, " ") || strings.IndexFunc(account, unicode.IsControl) >= 0 {
		return User{}, ErrInvalidAccount
	}
	if in.PasswordHash == "" || in.RequestID == "" || len(in.RequestID) > 128 {
		return User{}, errors.New("invalid bootstrap seed input")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Serialize trusted initializers; the unique index remains the final constraint.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7765301003)"); err != nil {
		return User{}, err
	}
	var existing User
	var hasCredential bool
	err = tx.QueryRow(ctx, `SELECT u.id, u.account,
		EXISTS(SELECT 1 FROM auth.password_credentials c WHERE c.user_id=u.id)
		FROM auth.users u WHERE u.is_bootstrap_admin FOR UPDATE`).Scan(&existing.ID, &existing.Account, &hasCredential)
	if err == nil {
		if existing.Account != account || !hasCredential {
			return User{}, ErrBootstrapCollision
		}
		if err := tx.Commit(ctx); err != nil {
			return User{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, err
	}
	var ordinaryID string
	err = tx.QueryRow(ctx, "SELECT id FROM auth.users WHERE account_key=$1 FOR UPDATE", account).Scan(&ordinaryID)
	if err == nil {
		return User{}, ErrBootstrapCollision
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, err
	}

	created := User{Account: account}
	if err := tx.QueryRow(ctx, "INSERT INTO auth.users (account, is_bootstrap_admin) VALUES ($1, true) RETURNING id", account).Scan(&created.ID); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO auth.password_credentials (user_id, password_hash) VALUES ($1, $2)", created.ID, in.PasswordHash); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO auth.authentication_events (event_type, outcome, subject_user_id, request_id) VALUES ('bootstrap_created', 'success', $1, $2)", created.ID, in.RequestID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return created, nil
}
