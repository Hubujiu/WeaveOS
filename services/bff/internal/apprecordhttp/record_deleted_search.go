package apprecordhttp

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"strings"
)

func (s *Service) recordDeletedSearchHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 6 || parts[1] != "forms" || parts[3] != "records" || parts[4] != "deleted" || parts[5] != "search" {
		return false
	}
	if r.Method != http.MethodPost {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	if !workflowHTTPID(parts[0]) || !workflowHTTPID(parts[2]) || r.URL.RawQuery != "" || r.URL.ForceQuery {
		failure(w, r, applications.ErrInvalid, "")
		return true
	}
	if encoding := r.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		failure(w, r, &appstructure.Error{Code: "COMMON_UNSUPPORTED_MEDIA_TYPE"}, "")
		return true
	}
	body, e := appstructure.DecodeRecordBody(w, r, "record.deleted.search")
	if e != nil {
		failure(w, r, e, "")
		return true
	}
	result, e := s.Records.SearchDeletedRecords(r.Context(), p, apprecordservice.DeletedRecordSearchRequest{AppID: parts[0], ViewID: parts[2], Page: value[int](body, "page"), PageSize: value[int](body, "pageSize"), QueryVersion: value[string](body, "queryVersion")})
	if e != nil {
		failure(w, r, e, "")
		return true
	}
	s.finish(w, r, p, 200, result, "", false, nil)
	return true
}
