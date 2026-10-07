package apprecordhttp

import (
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Session, CSRF and expected-actor checks run in ServeHTTP before this adapter.
func (s *Service) workflowLifecycleHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 8 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "workflow-instances" || (parts[7] != "lifecycle" && parts[7] != "actions") {
		return false
	}
	preview := parts[7] == "lifecycle" && r.Method == "GET"
	action := parts[7] == "actions" && r.Method == "POST"
	if !preview && !action {
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
	req := apprecordservice.WorkflowLifecycleRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], InstanceID: parts[6]}
	if preview {
		result, err := s.Records.PreviewWorkflowLifecycle(r.Context(), p, req)
		if err != nil {
			failure(w, r, err, "")
			return true
		}
		s.finish(w, r, p, 200, result, "", false, nil)
		return true
	}
	body, err := appstructure.DecodeRecordBody(w, r, "workflow.lifecycle.action")
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	operation := value[string](body, "operationId")
	m := httpserver.Metadata(r.Context())
	result, err := s.Records.AcceptWorkflowLifecycle(r.Context(), p, apprecordservice.WorkflowLifecycleActionRequest{
		WorkflowLifecycleRequest: req, OperationID: operation, Action: value[string](body, "action"),
		BasisToken: value[string](body, "basisToken"), TargetNodeID: value[string](body, "targetNodeId"),
	}, applications.Metadata{RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: r.UserAgent()})
	if err != nil {
		failure(w, r, err, operation)
		return true
	}
	status := 200
	if result.Status == "pending" {
		status = 202
	}
	s.finish(w, r, p, status, result, "/api/v1/application-workflow-operations/"+operation, true, nil)
	return true
}
