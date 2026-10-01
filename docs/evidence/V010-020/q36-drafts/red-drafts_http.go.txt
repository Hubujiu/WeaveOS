package personnel

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
)

// RED-stage placeholder. Shared HTTP chain and route registration are integrator-owned.
func (s *Service) DraftHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	fail(w, r, ErrNotImplemented)
	return true
}
