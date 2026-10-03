package appstructure

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

type Application struct {
	Pool              *pgxpool.Pool
	ConfirmationKey   []byte
	ConfirmationKeyID string
}
type Service struct {
	Application   *Application
	Authenticator session.Authenticator
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "definition behavior absent", 501)
}
