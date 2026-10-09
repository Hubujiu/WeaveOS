package apprecordhttp

import (
	"io"
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func (s *Service) workflowEventHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 7 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "workflow-events" {
		return false
	}
	if r.Method != "GET" {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	for _, id := range []string{parts[0], parts[2], parts[4], parts[6]} {
		if !workflowHTTPID(id) {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		failure(w, r, applications.ErrInvalid, "")
		return true
	}
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(body) != 0 {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	result, err := s.Records.ReadWorkflowEvent(r.Context(), p, apprecordservice.WorkflowEventRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], EventID: parts[6]})
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	s.finish(w, r, p, 200, result, "", false, nil)
	return true
}
