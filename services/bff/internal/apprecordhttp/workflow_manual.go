package apprecordhttp

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"strings"
)

// Authentication, CSRF and expected actor were checked by ServeHTTP.
func (s *Service) workflowManualHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) < 6 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "workflow-starts" {
		return false
	}
	if len(parts) != 6 || r.Method != "POST" {
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
	body, err := appstructure.DecodeRecordBody(w, r, "workflow.manual.start")
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	operation := value[string](body, "operationId")
	m := httpserver.Metadata(r.Context())
	req := apprecordservice.WorkflowManualStartRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], OperationID: operation, FlowID: value[string](body, "flowId"), ExpectedWorkflowRevision: value[int64](body, "expectedWorkflowRevision"), ExpectedSchemaVersion: value[int64](body, "expectedSchemaVersion"), ExpectedRecordVersion: value[int64](body, "expectedRecordVersion")}
	result, err := s.Records.StartManualWorkflow(r.Context(), p, req, applications.Metadata{RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: r.UserAgent()})
	if err != nil {
		failure(w, r, err, operation)
		return true
	}
	s.finish(w, r, p, 202, result, "/api/v1/application-operations/"+operation, true, nil)
	return true
}
