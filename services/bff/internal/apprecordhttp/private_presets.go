package apprecordhttp

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppresets"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Authentication, Origin/CSRF and expected actor are enforced by ServeHTTP
// before this route. The service rechecks all live database authorization.
func (s *Service) privatePresetsHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) < 4 || len(parts) > 5 || parts[1] != "forms" || parts[3] != "table-presets" {
		return false
	}
	app, view, id := parts[0], parts[2], ""
	if len(parts) == 5 {
		id = parts[4]
		if id == "" {
			respond(w, r, 404, "API_NOT_FOUND", nil, nil)
			return true
		}
	}
	service := &apppresets.Service{Pool: s.Records.Pool, Limits: s.Records.Limits}
	var data any
	var e error
	status, location, operation := 200, "", ""
	mutation := r.Method != "GET" && r.Method != "HEAD"
	switch {
	case r.Method == "GET" || r.Method == "HEAD":
		if _, e = closedQuery(r); e == nil {
			if id == "" {
				var items []json.RawMessage
				items, e = service.List(r.Context(), p, app, view)
				data = map[string]any{"items": items}
			} else {
				data, e = service.Get(r.Context(), p, app, view, id)
			}
		}
	case r.Method == "POST" && id == "" || r.Method == "PUT" && id != "":
		if _, e = closedQuery(r); e == nil {
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			encoding := r.Header.Get("Content-Encoding")
			if err != nil || media != "application/json" || encoding != "" && encoding != "identity" {
				e = &appstructure.Error{Code: "COMMON_UNSUPPORTED_MEDIA_TYPE"}
			} else {
				var raw []byte
				raw, e = io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
				if e != nil {
					e = apppresets.ErrInvalid
				} else {
					var request apppresets.Request
					request, e = apppresets.DecodeRequest(raw, r.Method == "PUT")
					if e == nil {
						request.AppID = app
						request.ViewID = view
						request.ID = id
						operation = request.OperationID
						var result applications.Result
						if r.Method == "POST" {
							result, e = service.Create(r.Context(), p, request)
						} else {
							result, e = service.Update(r.Context(), p, request)
						}
						status, location, data = result.Status, result.Location, result.Data
					}
				}
			}
		}
	case r.Method == "DELETE" && id != "":
		q, err := closedQuery(r, "operationId", "expectedVersion")
		e = err
		if e == nil {
			operation = q.Get("operationId")
			version, err := number(q.Get("expectedVersion"), 1, 9007199254740991)
			e = err
			if e == nil {
				var body []byte
				body, e = io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
				if e != nil || len(body) > 0 {
					e = apppresets.ErrInvalid
				}
			}
			if e == nil {
				var result applications.Result
				result, e = service.Delete(r.Context(), p, apppresets.Request{AppID: app, ViewID: view, ID: id, OperationID: operation, ExpectedVersion: version})
				status = result.Status
			}
		}
	default:
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	if e != nil {
		failure(w, r, e, operation)
		return true
	}
	s.finish(w, r, p, status, data, location, mutation, nil)
	return true
}
