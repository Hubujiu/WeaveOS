package personnel

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"log/slog"
	"net/http"
)

type Service struct {
	Application       *Application
	Authenticator     session.Authenticator
	Logger            *slog.Logger
	TrustedProxyHosts []string
}

func (s *Service) TrustedProxies() []string { return append([]string(nil), s.TrustedProxyHosts...) }
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "personnel adapter not implemented", http.StatusServiceUnavailable)
}
