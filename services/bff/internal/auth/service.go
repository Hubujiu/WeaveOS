package auth

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
)

type Service struct {
	Pool               *pgxpool.Pool
	Sessions           *session.Store
	Origin, AuditKeyID string
	AuditKey           []byte
	Logger             *slog.Logger
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
func (s *Service) Ready(context.Context) error { return errors.New("unwired") }
