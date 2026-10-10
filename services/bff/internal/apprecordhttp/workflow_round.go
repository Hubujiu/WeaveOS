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

func (s *Service) workflowRoundHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 8 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "workflows" || (parts[7] != "round-actions" && parts[7] != "rework") {
		return false
	}
	preview := parts[7] == "round-actions" && r.Method == "GET"
	if !preview && r.Method != "POST" {
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
	q := apprecordservice.WorkflowRoundRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], InstanceID: parts[6]}
	if preview {
		result, err := s.Records.PreviewWorkflowRound(r.Context(), p, q)
		if err != nil {
			failure(w, r, err, "")
			return true
		}
		s.finish(w, r, p, 200, result, "", false, nil)
		return true
	}
	kind := "workflow.round.start"
	if parts[7] == "rework" {
		kind = "workflow.round.rework"
	}
	body, err := appstructure.DecodeRecordBody(w, r, kind)
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	op := value[string](body, "operationId")
	m := httpserver.Metadata(r.Context())
	metadata := applications.Metadata{RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: r.UserAgent()}
	if kind == "workflow.round.rework" {
		result, e := s.Records.ReworkWorkflowRound(r.Context(), p, apprecordservice.WorkflowRoundReworkRequest{WorkflowRoundRequest: q, OperationID: op, ExpectedSchemaVersion: value[int64](body, "expectedSchemaVersion"), ExpectedRecordVersion: value[int64](body, "expectedRecordVersion"), Changes: value[map[string]any](body, "changes")}, metadata)
		if e != nil {
			failure(w, r, e, op)
			return true
		}
		s.finish(w, r, p, 200, result, "", true, nil)
		return true
	}
	result, e := s.Records.StartWorkflowRound(r.Context(), p, apprecordservice.WorkflowRoundStartRequest{WorkflowRoundRequest: q, OperationID: op, Kind: value[string](body, "kind"), ExpectedWorkflowRevision: value[int64](body, "expectedWorkflowRevision"), ExpectedSchemaVersion: value[int64](body, "expectedSchemaVersion"), ExpectedRecordVersion: value[int64](body, "expectedRecordVersion")}, metadata)
	if e != nil {
		failure(w, r, e, op)
		return true
	}
	s.finish(w, r, p, 202, result, "/api/v1/application-operations/"+op, true, nil)
	return true
}
