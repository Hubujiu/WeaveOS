package apptemplates

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func (s *HTTP) candidates(w http.ResponseWriter, r *http.Request, p session.Principal, kind string) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		templateRespond(w, r, 405, "COMMON_METHOD_NOT_ALLOWED", nil)
		return
	}
	query, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		templateFail(w, r, ErrInvalid)
		return
	}
	for key, values := range query {
		if len(values) != 1 || key != "q" && key != "pageSize" && key != "pageToken" {
			templateFail(w, r, ErrInvalid)
			return
		}
	}
	q, size := strings.TrimSpace(query.Get("q")), 20
	if text, ok := query["pageSize"]; ok {
		size, e = strconv.Atoi(text[0])
		if e != nil || size < 1 || size > 50 || strconv.Itoa(size) != text[0] {
			templateFail(w, r, ErrInvalid)
			return
		}
	}
	if !utf8.ValidString(q) || utf8.RuneCountInString(q) > 100 || strings.ContainsRune(q, 0) {
		templateFail(w, r, ErrInvalid)
		return
	}
	items, page, e := s.Candidates.TemplateReferenceCandidates(r.Context(), p, kind, q, size, query.Get("pageToken"))
	if e != nil {
		var invalid *appstructure.Error
		if errors.As(e, &invalid) && invalid.Code == "COMMON_VALIDATION_FAILED" {
			e = ErrInvalid
		}
		templateFail(w, r, e)
		return
	}
	if e = s.Authenticator.Renew(r.Context(), w, r, p); e != nil {
		templateFail(w, r, e)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if r.Method == "HEAD" {
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
		Meta    map[string]any `json:"meta"`
	}{"OK", "success", map[string]any{"items": items}, map[string]any{"requestId": httpserver.Metadata(r.Context()).RequestID, "pagination": page}})
}
