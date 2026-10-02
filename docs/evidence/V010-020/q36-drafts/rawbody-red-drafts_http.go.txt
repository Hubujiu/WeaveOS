package personnel

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func draftFailure(w http.ResponseWriter, r *http.Request, e error) {
	code, message := "", ""
	switch {
	case errors.Is(e, ErrDraftConflict):
		code, message = "PERSONNEL_DRAFT_CONFLICT", "草稿已由其他页面修改，请重新核对"
	case errors.Is(e, ErrDraftLimit):
		code, message = "PERSONNEL_DRAFT_LIMIT_REACHED", "已保存20份草稿，请先管理已有草稿"
	default:
		fail(w, r, e)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(409)
	if r.Method == "HEAD" {
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{code, message, nil, map[string]string{"requestId": httpserver.Metadata(r.Context()).RequestID}})
}
func draftDecodeBody(w http.ResponseWriter, r *http.Request, out any, fields ...string) bool {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		respond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return false
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		fail(w, r, ErrInvalid)
		return false
	}
	values, e := draftObject(raw, fields...)
	if e != nil {
		fail(w, r, e)
		return false
	}
	for _, field := range fields {
		if field != "targetId" && field != "baseVersion" && string(values[field]) == "null" {
			fail(w, r, ErrInvalid)
			return false
		}
	}
	if json.Unmarshal(raw, out) != nil {
		fail(w, r, ErrInvalid)
		return false
	}
	return true
}

// DraftHTTP dispatches only draft paths AFTER the shared Prepare/Authenticate
// chain (Authenticate with write=true on mutations includes CSRF). Returns false
// for other paths; the integrator registers this in Service.ServeHTTP. The
// Application rechecks account/auth_version/personnel.manage on every operation.
func (s *Service) DraftHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	const base = "/api/v1/personnel/drafts"
	if r.URL.Path != base && !strings.HasPrefix(r.URL.Path, base+"/") {
		return false
	}
	if s.Application == nil {
		fail(w, r, session.ErrUnavailable)
		return true
	}
	id := strings.TrimPrefix(r.URL.Path, base+"/")
	collection := r.URL.Path == base
	if !collection && (strings.Contains(id, "/") || !validID(id)) {
		fail(w, r, ErrInvalid)
		return true
	}
	if r.Method != "DELETE" {
		if _, e := query(r); e != nil {
			fail(w, r, e)
			return true
		}
	}
	if collection {
		switch r.Method {
		case "GET":
			result, e := s.Application.ListDrafts(r.Context(), p)
			if e != nil {
				draftFailure(w, r, e)
			} else {
				s.finish(w, r, p, 200, result, "")
			}
			return true
		case "POST":
			var in DraftCreateInput
			if !draftDecodeBody(w, r, &in, "kind", "targetId", "baseVersion", "payload") {
				return true
			}
			result, e := s.Application.CreateDraft(r.Context(), p, in)
			if e != nil {
				draftFailure(w, r, e)
			} else {
				s.finish(w, r, p, 201, result, base+"/"+result.ID)
			}
			return true
		}
	} else {
		switch r.Method {
		case "GET":
			result, e := s.Application.GetDraft(r.Context(), p, id)
			if e != nil {
				draftFailure(w, r, e)
			} else {
				s.finish(w, r, p, 200, result, "")
			}
			return true
		case "PUT":
			var in DraftUpdateInput
			if !draftDecodeBody(w, r, &in, "version", "payload") {
				return true
			}
			result, e := s.Application.UpdateDraft(r.Context(), p, id, in)
			if e != nil {
				draftFailure(w, r, e)
			} else {
				s.finish(w, r, p, 200, result, "")
			}
			return true
		case "DELETE":
			version, e := deleteVersion(r)
			if e == nil {
				e = s.Application.DeleteDraft(r.Context(), p, id, version)
			}
			if e != nil {
				draftFailure(w, r, e)
			} else {
				s.finish(w, r, p, 204, nil, "")
			}
			return true
		}
	}
	respond(w, r, 404, "API_NOT_FOUND", nil)
	return true
}
