package auth

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

func reply(w http.ResponseWriter, r *http.Request, status int, code string, data any) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if r.Method == "HEAD" || status == 204 {
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{code, publicMessage(code), data, map[string]string{"requestId": httpserver.Metadata(r.Context()).RequestID}})
}

func publicMessage(code string) string {
	if message, ok := map[string]string{
		"OK": "success", "AUTH_INVALID_CREDENTIALS": "账号或密码错误",
		"AUTH_UNAUTHENTICATED": "请先登录", "AUTH_SESSION_EXPIRED": "登录已过期，请重新登录",
		"COMMON_INVALID_ARGUMENT": "请求参数不合法", "COMMON_VALIDATION_FAILED": "请求参数不合法",
		"COMMON_PERMISSION_DENIED": "没有执行此操作的权限", "COMMON_CSRF_REJECTED": "请求来源或安全校验失败",
		"USER_ACCOUNT_ALREADY_EXISTS": "账号已存在", "USER_NOT_FOUND": "用户不存在",
		"INVITATION_INVALID": "邀请码无效", "INVITATION_ALREADY_USED": "邀请码已被使用",
		"COMMON_UNSUPPORTED_MEDIA_TYPE": "仅接受 JSON 请求", "COMMON_SERVICE_UNAVAILABLE": "服务暂时不可用，请稍后重试",
		"COMMON_DEADLINE_EXCEEDED": "请求超时，请稍后重试", "API_NOT_FOUND": "请求的接口不存在",
	}[code]; ok {
		return message
	}
	return "请求失败"
}

func (s *Service) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, code := 503, "COMMON_SERVICE_UNAVAILABLE"
	var data any
	var failure *Failure
	if errors.As(err, &failure) {
		switch failure.Code {
		case "COMMON_INVALID_ARGUMENT", "COMMON_VALIDATION_FAILED", "INVITATION_INVALID":
			status, code = 400, failure.Code
		case "AUTH_INVALID_CREDENTIALS":
			status, code = 401, failure.Code
		case "COMMON_PERMISSION_DENIED":
			status, code = 403, failure.Code
		case "USER_NOT_FOUND":
			status, code = 404, failure.Code
		case "USER_ACCOUNT_ALREADY_EXISTS", "INVITATION_ALREADY_USED":
			status, code = 409, failure.Code
		case "COMMON_INTERNAL_ERROR":
			status, code = 500, failure.Code
		}
		if code == "COMMON_VALIDATION_FAILED" {
			data = struct {
				Violations []violation `json:"violations"`
			}{failure.Violations}
		}
	}
	if errors.Is(err, session.ErrUnauthorized) {
		status, code = 401, "AUTH_UNAUTHENTICATED"
	}
	if errors.Is(err, session.ErrForbidden) {
		status, code = 403, "COMMON_CSRF_REJECTED"
	}
	reply(w, r, status, code, data)
}

func decode(w http.ResponseWriter, r *http.Request, out any, stringFields ...string) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		reply(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return false
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	seen := map[string]bool{}
	var violations []violation
	for d.More() {
		k, err := d.Token()
		name, ok := k.(string)
		if err != nil || !ok || seen[name] {
			reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
			return false
		}
		for _, field := range stringFields {
			if name == field {
				var text string
				if string(value) == "null" || json.Unmarshal(value, &text) != nil {
					violations = append(violations, violation{"body", field, "COMMON_INVALID_ARGUMENT", "字段必须为字符串"})
				}
			}
		}
	}
	if _, err := d.Token(); err != nil {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	if _, err := d.Token(); err != io.EOF {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	if len(violations) > 0 {
		reply(w, r, 400, "COMMON_VALIDATION_FAILED", struct {
			Violations []violation `json:"violations"`
		}{violations})
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil {
		reply(w, r, 400, "COMMON_INVALID_ARGUMENT", nil)
		return false
	}
	return true
}
