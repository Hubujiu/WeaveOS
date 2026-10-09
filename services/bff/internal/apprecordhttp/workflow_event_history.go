package apprecordhttp

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"net/http"
	"strings"
)

func (s *Service) workflowEventHistoryHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 6 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "workflow-events" {
		return false
	}
	if r.Method != "GET" {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	for _, id := range []string{parts[0], parts[2], parts[4]} {
		if !workflowHTTPID(id) {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	if r.URL.ForceQuery {
		failure(w, r, applications.ErrInvalid, "")
		return true
	}
	query, err := closedQuery(r, "pageSize", "pageToken")
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	size := int64(20)
	if raw := query.Get("pageSize"); raw != "" {
		size, err = number(raw, 1, 100)
		if err != nil {
			failure(w, r, err, "")
			return true
		}
	}
	if r.Body != nil {
		b, e := io.ReadAll(io.LimitReader(r.Body, 1))
		if e != nil || len(b) > 0 {
			failure(w, r, applications.ErrInvalid, "")
			return true
		}
	}
	result, err := s.Records.ReadWorkflowEventHistory(r.Context(), p, apprecordservice.WorkflowEventHistoryRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], PageSize: int(size), PageToken: query.Get("pageToken")})
	if err != nil {
		failure(w, r, err, "")
		return true
	}
	data := struct {
		Items []apprecordservice.WorkflowEventSummary `json:"items"`
	}{Items: result.Items}
	s.finish(w, r, p, 200, data, "", false, appstructure.Pagination{HasMore: result.HasMore, NextPageToken: result.NextPageToken})
	return true
}
