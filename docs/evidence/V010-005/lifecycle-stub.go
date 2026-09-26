package session

import "context"

func (s *Store) Ping(context.Context) error { return ErrInvalid }
func (s *Store) Close() error { return nil }
