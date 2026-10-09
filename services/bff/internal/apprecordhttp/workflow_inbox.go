package apprecordhttp

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
)

// Invoked only after live Session/CSRF/expected-actor validation. This search
// does not accept actor identity or business mutation instructions.
func (s *Service) workflowInboxHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	if r.URL.Path != "/api/v1/workflow-tasks/search" {
		return false
	}
	if r.Method != "POST" {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		failure(w, r, applications.ErrInvalid, "")
		return true
	}
	body, err := appstructure.DecodeRecordBody(w, r, "workflow.inbox.search")
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	result, err := s.Records.SearchWorkflowInbox(r.Context(), p, apprecordservice.WorkflowInboxRequest{Page: value[int](body, "page"), PageSize: value[int](body, "pageSize"), QueryVersion: value[string](body, "queryVersion")})
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	s.finish(w, r, p, 200, result, "", false, nil)
	return true
}
