package persistence

import (
	"context"
	"errors"
)

var ErrBootstrapCollision = errors.New("bootstrap account collision")

type BootstrapSeedInput struct {
	Account      string
	PasswordHash string
	RequestID    string
}

func (s *Store) SeedBootstrap(_ context.Context, _ BootstrapSeedInput) (User, error) {
	return User{}, errors.New("bootstrap seed not implemented")
}
