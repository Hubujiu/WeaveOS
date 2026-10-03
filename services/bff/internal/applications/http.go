package applications

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

type Application struct{ Pool *pgxpool.Pool }
type Service struct {
	Application   *Application
	Authenticator session.Authenticator
}

// Compile-capable RED placeholder; no application behavior is implemented.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := s.Authenticator.Authenticate(r, r.Method != "GET" && r.Method != "HEAD")
	if err != nil {
		if err == session.ErrUnauthorized {
			w.WriteHeader(401)
		} else {
			w.WriteHeader(403)
		}
		return
	}
	w.WriteHeader(503)
}
