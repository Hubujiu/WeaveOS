package personnel

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func presetFailure(w http.ResponseWriter, r *http.Request, e error) {
	status, code, message := 409, "", ""
	switch {
	case errors.Is(e, ErrPresetName):
		code, message = "PERSONNEL_PRESET_NAME_CONFLICT", "当前表格已有同名筛选，请修改名称"
	case errors.Is(e, ErrPresetLimit):
		code, message = "PERSONNEL_PRESET_LIMIT_REACHED", "当前表格已保存20套筛选，请先管理已有方案"
	case errors.Is(e, ErrPresetConflict):
		code, message = "PERSONNEL_PRESET_CONFLICT", "方案已由其他页面修改，请重新核对"
	case errors.Is(e, ErrPresetSchema):
		status, code, message = 400, "COMMON_INVALID_ARGUMENT", "不支持此筛选方案结构版本，请重新核对"
	default:
		fail(w, r, e)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
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
func presetDecodeBody(w http.ResponseWriter, r *http.Request, out any, fields ...string) bool {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		respond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return false
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if e != nil || !draftJSONUnicode(raw) {
		fail(w, r, ErrInvalid)
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, e := filterJSON(d, 0)
	values, ok := value.(map[string]any)
	if e != nil || !ok {
		fail(w, r, ErrInvalid)
		return false
	}
	if _, e = d.Token(); e != io.EOF {
		fail(w, r, ErrInvalid)
		return false
	}
	allowed := map[string]bool{"filter": true}
	for _, field := range fields {
		allowed[field] = true
		if v, exists := values[field]; !exists || v == nil {
			fail(w, r, ErrInvalid)
			return false
		}
	}
	for field := range values {
		if !allowed[field] {
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

// Registered behind the existing Prepare/Authenticate/CSRF chain; every
// Application method additionally rechecks live personnel.manage authorization.
func (s *Service) PresetHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	const base = "/api/v1/personnel/table-presets"
	if r.URL.Path != base && !strings.HasPrefix(r.URL.Path, base+"/") {
		return false
	}
	collection := r.URL.Path == base
	id := strings.TrimPrefix(r.URL.Path, base+"/")
	if !collection && (!validID(id) || strings.Contains(id, "/")) {
		fail(w, r, ErrInvalid)
		return true
	}
	if collection && r.Method == "GET" {
		values, e := query(r, "view")
		if e != nil || !validPresetView(values.Get("view")) {
			fail(w, r, ErrInvalid)
			return true
		}
		result, e := s.Application.ListPresets(r.Context(), p, values.Get("view"))
		if e != nil {
			presetFailure(w, r, e)
		} else {
			s.finish(w, r, p, 200, result, "")
		}
		return true
	}
	if r.Method != "DELETE" {
		if _, e := query(r); e != nil {
			fail(w, r, e)
			return true
		}
	}
	if collection && r.Method == "POST" {
		var in PresetCreateInput
		if !presetDecodeBody(w, r, &in, "view", "name", "hiddenColumnIds", "schemaVersion") {
			return true
		}
		result, e := s.Application.CreatePreset(r.Context(), p, in)
		if e != nil {
			presetFailure(w, r, e)
		} else {
			s.finish(w, r, p, 201, result, base+"/"+result.ID)
		}
		return true
	}
	if !collection {
		switch r.Method {
		case "GET":
			result, e := s.Application.GetPreset(r.Context(), p, id)
			if e != nil {
				presetFailure(w, r, e)
			} else {
				s.finish(w, r, p, 200, result, "")
			}
			return true
		case "PUT":
			var in PresetUpdateInput
			if !presetDecodeBody(w, r, &in, "version", "name", "hiddenColumnIds", "schemaVersion") {
				return true
			}
			result, e := s.Application.UpdatePreset(r.Context(), p, id, in)
			if e != nil {
				presetFailure(w, r, e)
			} else {
				s.finish(w, r, p, 200, result, "")
			}
			return true
		case "DELETE":
			version, e := deleteVersion(r)
			if e == nil {
				e = s.Application.DeletePreset(r.Context(), p, id, version)
			}
			if e != nil {
				presetFailure(w, r, e)
			} else {
				s.finish(w, r, p, 204, nil, "")
			}
			return true
		}
	}
	respond(w, r, 404, "API_NOT_FOUND", nil)
	return true
}
