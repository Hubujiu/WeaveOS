package personnel

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func respond(w http.ResponseWriter, r *http.Request, status int, code string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if status == 204 || r.Method == "HEAD" {
		return
	}
	message := map[string]string{"OK": "success", "AUTH_UNAUTHENTICATED": "请先登录", "COMMON_PERMISSION_DENIED": "没有执行此操作的权限", "COMMON_CSRF_REJECTED": "请求来源或安全校验失败", "COMMON_INVALID_ARGUMENT": "请求参数不合法", "COMMON_UNSUPPORTED_MEDIA_TYPE": "仅接受 JSON 请求", "PERSONNEL_CONFLICT": "配置已变更或仍被引用，请重新核对", "PERSONNEL_NOT_FOUND": "对象不存在", "COMMON_SERVICE_UNAVAILABLE": "服务暂时不可用，请稍后重试", "API_NOT_FOUND": "请求的接口不存在", "COMMON_QUERY_CHANGED": "查询相关数据已变化，请刷新重查", "COMMON_QUERY_CONTEXT_EXPIRED": "查询上下文已过期，请重新查询"}[code]
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{code, message, data, map[string]string{"requestId": httpserver.Metadata(r.Context()).RequestID}})
}
func fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrQueryBusy) {
		queryBusyResponse(w, r)
		return
	}
	status, code := 503, "COMMON_SERVICE_UNAVAILABLE"
	switch {
	case errors.Is(err, session.ErrUnauthorized):
		status, code = 401, "AUTH_UNAUTHENTICATED"
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	case errors.Is(err, session.ErrForbidden):
		status, code = 403, "COMMON_CSRF_REJECTED"
	case errors.Is(err, ErrDenied):
		status, code = 403, "COMMON_PERMISSION_DENIED"
	case errors.Is(err, ErrInvalid):
		status, code = 400, "COMMON_INVALID_ARGUMENT"
	case errors.Is(err, ErrQueryChanged):
		status, code = 409, "COMMON_QUERY_CHANGED"
	case errors.Is(err, ErrQueryContextExpired):
		status, code = 409, "COMMON_QUERY_CONTEXT_EXPIRED"
	case errors.Is(err, ErrConflict):
		status, code = 409, "PERSONNEL_CONFLICT"
	case errors.Is(err, ErrMissing):
		status, code = 404, "PERSONNEL_NOT_FOUND"
	}
	respond(w, r, status, code, nil)
}
func decodeBody(w http.ResponseWriter, r *http.Request, out any, required ...string) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		respond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return false
	}
	bytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		fail(w, r, ErrInvalid)
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(bytes)))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		fail(w, r, ErrInvalid)
		return false
	}
	seen := map[string]bool{}
	values := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			fail(w, r, ErrInvalid)
			return false
		}
		seen[name] = true
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			fail(w, r, ErrInvalid)
			return false
		}
		values[name] = raw
		if string(raw) == "null" {
			fail(w, r, ErrInvalid)
			return false
		}
	}
	if _, err := d.Token(); err != nil {
		fail(w, r, ErrInvalid)
		return false
	}
	if _, err := d.Token(); err != io.EOF {
		fail(w, r, ErrInvalid)
		return false
	}
	for _, field := range required {
		raw, ok := values[field]
		if !ok || string(raw) == "null" {
			fail(w, r, ErrInvalid)
			return false
		}
	}
	d = json.NewDecoder(strings.NewReader(string(bytes)))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		fail(w, r, ErrInvalid)
		return false
	}
	return true
}
func query(r *http.Request, allowed ...string) (url.Values, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, ErrInvalid
	}
	known := map[string]bool{}
	for _, field := range allowed {
		known[field] = true
	}
	for name, value := range values {
		if !known[name] || len(value) != 1 {
			return nil, ErrInvalid
		}
	}
	return values, nil
}
func pageQuery(values url.Values) (PageQuery, error) {
	result := PageQuery{Page: 1, PageSize: 20, Search: values.Get("search")}
	for _, field := range []string{"page", "pageSize"} {
		if raw, ok := values[field]; ok {
			n, err := strconv.Atoi(raw[0])
			if err != nil || n < 1 {
				return result, ErrInvalid
			}
			if field == "page" {
				result.Page = n
			} else {
				result.PageSize = n
			}
		}
	}
	return normalizedPage(result)
}
func deleteVersion(r *http.Request, extra ...string) (int64, error) {
	values, err := query(r, append([]string{"version"}, extra...)...)
	if err != nil {
		return 0, err
	}
	version, err := strconv.ParseInt(values.Get("version"), 10, 64)
	if err != nil || version < 1 || version > maxSafeVersion {
		return 0, ErrInvalid
	}
	return version, nil
}
