package session

import "context"

func (s *Store) Ping(ctx context.Context) error {
	if err := s.ready(); err != nil {
		return err
	}
	return s.client.Ping(ctx).Err()
}
func (s *Store) Close() error {
	if err := s.ready(); err != nil {
		return err
	}
	return s.client.Close()
}
