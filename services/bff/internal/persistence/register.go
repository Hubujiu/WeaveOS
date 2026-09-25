package persistence

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvitationUnavailable = errors.New("invitation unavailable")
	ErrAccountTaken          = errors.New("account taken")
	ErrInvalidAccount        = errors.New("invalid account")
	ErrNotImplemented        = errors.New("storage not implemented")
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

func (s *Store) Register(_ context.Context, _ RegistrationInput) (User, error) {
	return User{}, ErrNotImplemented
}
