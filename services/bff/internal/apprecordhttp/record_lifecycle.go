package apprecordhttp

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"strings"
)

func (s *Service) recordLifecycleHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 6 || parts[1] != "forms" || parts[3] != "records" || parts[5] != "lifecycle" && parts[5] != "deletion" && parts[5] != "restoration" {
		return false
	}
	read := parts[5] == "lifecycle" && (r.Method == "GET" || r.Method == "HEAD")
	if !read && (r.Method != "POST" || parts[5] == "lifecycle") {
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
	if read {
		state, e := s.Records.GetRecordLifecycle(r.Context(), p, parts[0], parts[2], parts[4])
		if e != nil {
			failure(w, r, e, "")
			return true
		}
		s.finish(w, r, p, 200, state, "", false, nil)
		return true
	}
	if encoding := r.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		failure(w, r, &appstructure.Error{Code: "COMMON_UNSUPPORTED_MEDIA_TYPE"}, "")
		return true
	}
	body, e := appstructure.DecodeRecordBody(w, r, "record.lifecycle")
	if e != nil {
		failure(w, r, e, "")
		return true
	}
	op := value[string](body, "operationId")
	result, e := s.Records.ChangeRecordLifecycle(r.Context(), p, apprecordservice.RecordLifecycleRequest{AppID: parts[0], ViewID: parts[2], RecordID: parts[4], OperationID: op, ExpectedSchemaVersion: value[int64](body, "expectedSchemaVersion"), ExpectedRecordVersion: value[int64](body, "expectedRecordVersion"), Deleted: parts[5] == "deletion"})
	if e != nil {
		failure(w, r, e, op)
		return true
	}
	s.finish(w, r, p, 200, result, "", true, nil)
	return true
}
