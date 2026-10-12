package apprecordhttp

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"strings"
)

func (s *Service) draftEditorHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 6 || parts[1] != "forms" || parts[3] != "drafts" || parts[5] != "editor-context" {
		return false
	}
	if r.Method != http.MethodGet {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	for _, id := range []string{parts[0], parts[2], parts[4]} {
		if !appfields.ValidID(id) {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	if _, err := closedQuery(r); err != nil {
		failure(w, r, err, "")
		return true
	}
	result, err := s.Records.GetDraftEditorContext(r.Context(), p, parts[0], parts[2], parts[4])
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	s.finish(w, r, p, 200, result, "", false, nil)
	return true
}
