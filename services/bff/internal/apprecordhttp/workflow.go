package apprecordhttp

import (
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func workflowHTTPID(id string) bool {
	return appfields.ValidID(id) && id != "00000000-0000-0000-0000-000000000000"
}

// workflowHTTP runs only after the existing Session and expected-actor boundary.
func (s *Service) workflowHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	const statusPrefix = "/api/v1/application-workflow-operations/"
	if strings.HasPrefix(r.URL.Path, statusPrefix) {
		operation := strings.TrimPrefix(r.URL.Path, statusPrefix)
		if r.Method != "GET" || strings.Contains(operation, "/") {
			respond(w, r, 404, "API_NOT_FOUND", nil, nil)
			return true
		}
		if !workflowHTTPID(operation) || r.URL.RawQuery != "" || r.URL.ForceQuery {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
		result, err := s.Records.WorkflowOperation(r.Context(), p, operation)
		if err != nil {
			failure(w, r, err, operation)
			return true
		}
		s.finish(w, r, p, 200, result, "", false, nil)
		return true
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) < 6 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "workflow-instances" {
		return false
	}
	preview := len(parts) == 9 && parts[7] == "tasks" && r.Method == "GET"
	action := len(parts) == 10 && parts[7] == "tasks" && parts[9] == "actions" && r.Method == "POST"
	if !preview && !action {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	for _, id := range []string{parts[0], parts[2], parts[4], parts[6], parts[8]} {
		if !workflowHTTPID(id) {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		failure(w, r, applications.ErrInvalid, "")
		return true
	}
	req := apprecordservice.WorkflowTaskRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], InstanceID: parts[6], TaskID: parts[8]}
	if preview {
		result, err := s.Records.PreviewWorkflowTask(r.Context(), p, req)
		if err != nil {
			failure(w, r, err, "")
			return true
		}
		s.finish(w, r, p, 200, result, "", false, nil)
		return true
	}
	body, err := appstructure.DecodeRecordBody(w, r, "workflow.task.action")
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	operation := value[string](body, "operationId")
	m := httpserver.Metadata(r.Context())
	result, err := s.Records.AcceptWorkflowTask(r.Context(), p, apprecordservice.WorkflowTaskActionRequest{WorkflowTaskRequest: req, OperationID: operation, Action: value[string](body, "action"), BasisToken: value[string](body, "basisToken")}, applications.Metadata{RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: r.UserAgent()})
	if err != nil {
		failure(w, r, err, operation)
		return true
	}
	status := 200
	if result.Status == "pending" {
		status = 202
	}
	s.finish(w, r, p, status, result, statusPrefix+operation, true, nil)
	return true
}
