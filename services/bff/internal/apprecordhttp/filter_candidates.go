package apprecordhttp

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func (s *Service) filterCandidatesHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) != 4 || parts[1] != "forms" || parts[3] != "filter-candidates" {
		return false
	}
	if r.Method != http.MethodGet {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return true
	}
	invalid := func() bool { failure(w, r, applications.ErrInvalid, ""); return true }
	if !workflowHTTPID(parts[0]) || !workflowHTTPID(parts[2]) {
		return invalid()
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return invalid()
	}
	for key, values := range q {
		if len(values) != 1 {
			return invalid()
		}
		switch key {
		case "q":
		case "fieldId", "pageSize", "pageToken":
			if values[0] == "" {
				return invalid()
			}
		default:
			return invalid()
		}
	}
	if !workflowHTTPID(q.Get("fieldId")) {
		return invalid()
	}
	search := strings.TrimSpace(q.Get("q"))
	if !utf8.ValidString(search) || utf8.RuneCountInString(search) > 100 || strings.ContainsRune(search, 0) {
		return invalid()
	}
	size := int64(20)
	if q.Has("pageSize") {
		size, e = number(q.Get("pageSize"), 1, 50)
		if e != nil {
			return invalid()
		}
	}
	token := q.Get("pageToken")
	if token != "" {
		raw, e := base64.RawURLEncoding.DecodeString(token)
		if e != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
			return invalid()
		}
	}
	if r.Body != nil {
		raw, e := io.ReadAll(io.LimitReader(r.Body, 1))
		if e != nil || len(raw) != 0 {
			return invalid()
		}
	}
	result, e := s.Records.FilterCandidates(r.Context(), p, apprecordservice.FilterCandidatesRequest{AppID: parts[0], ViewID: parts[2], FieldID: q.Get("fieldId"), Q: search, PageSize: int(size), PageToken: token})
	if e != nil {
		failure(w, r, e, "")
		return true
	}
	var next *string
	if result.NextPageToken != "" {
		next = &result.NextPageToken
	}
	s.finish(w, r, p, 200, result, "", false, map[string]any{"hasMore": result.HasMore, "nextPageToken": next})
	return true
}
