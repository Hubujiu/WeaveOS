package apprecordhttp

import (
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// The shared Session, CSRF and expected-actor boundary runs before this adapter.
// Search is read-only even though its closed criteria envelope uses POST.
func (s *Service) workflowReadHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 7 || parts[1] != "forms" || parts[3] != "records" ||
		parts[5] != "workflow-instances" || parts[6] != "search" {
		return false
	}
	if r.Method != "POST" {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	for _, id := range []string{parts[0], parts[2], parts[4]} {
		if !workflowHTTPID(id) {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		failure(w, r, applications.ErrInvalid, "")
		return true
	}
	body, err := appstructure.DecodeRecordBody(w, r, "workflow.instances.search")
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	result, err := s.Records.SearchRecordWorkflows(r.Context(), p, apprecordservice.WorkflowInstanceSearchRequest{
		AppID: parts[0], ViewID: parts[2], RecordID: parts[4],
		QueryVersion: value[string](body, "queryVersion"),
		Page:         value[int](body, "page"), PageSize: value[int](body, "pageSize"),
	})
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	s.finish(w, r, p, 200, result, "", false, nil)
	return true
}
