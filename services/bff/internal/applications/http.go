package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type Service struct {
	Application       *Application
	Authenticator     session.Authenticator
	Logger            *slog.Logger
	TrustedProxyHosts []string
	Definitions       http.Handler
	Records           http.Handler
	Workflows         http.Handler
}

func (s *Service) TrustedProxies() []string { return append([]string(nil), s.TrustedProxyHosts...) }
func respond(w http.ResponseWriter, r *http.Request, status int, code string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method == "HEAD" {
		return
	}
	message := map[string]string{"OK": "success", "AUTH_UNAUTHENTICATED": "请先登录", "COMMON_CSRF_REJECTED": "请求来源或安全校验失败", "COMMON_INVALID_ARGUMENT": "请求参数不合法", "COMMON_VALIDATION_FAILED": "请求参数校验失败", "AUTH_SESSION_CHANGED": "当前账号已变化，请重新加载", "COMMON_UNSUPPORTED_MEDIA_TYPE": "仅接受 JSON 请求", "COMMON_SERVICE_UNAVAILABLE": "服务暂时不可用，请稍后重试", "API_NOT_FOUND": "请求的接口不存在", "APPLICATION_NOT_FOUND": "应用或配置不存在", "APPLICATION_FORBIDDEN": "没有应用权限", "APPLICATION_POLICY_CONFLICT": "应用权限配置已变化", "APPLICATION_OPERATION_CONFLICT": "操作标识已用于不同请求", "APPLICATION_OPERATION_UNCONFIRMED": "操作结果尚未确认，请查询原操作", "APPLICATION_RESOURCE_INVALID": "应用资源不合法"}[code]
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{code, message, data, map[string]string{"requestId": httpserver.Metadata(r.Context()).RequestID}})
}
func fail(w http.ResponseWriter, r *http.Request, err error) {
	status, code := 503, "COMMON_SERVICE_UNAVAILABLE"
	var value any
	switch {
	case errors.Is(err, ErrExpectedActorInvalid):
		status, code = 400, "COMMON_VALIDATION_FAILED"
		value = ActorHeaderViolation()
	case errors.Is(err, ErrSessionChanged):
		status, code = 409, "AUTH_SESSION_CHANGED"
	case errors.Is(err, session.ErrUnauthorized):
		status, code = 401, "AUTH_UNAUTHENTICATED"
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	case errors.Is(err, session.ErrForbidden):
		status, code = 403, "COMMON_CSRF_REJECTED"
	case errors.Is(err, ErrMissing):
		status, code = 404, "APPLICATION_NOT_FOUND"
	case errors.Is(err, ErrDenied):
		status, code = 403, "APPLICATION_FORBIDDEN"
	case errors.Is(err, ErrPolicyConflict):
		status, code = 409, "APPLICATION_POLICY_CONFLICT"
	case errors.Is(err, ErrOperationConflict):
		status, code = 409, "APPLICATION_OPERATION_CONFLICT"
	case errors.Is(err, ErrUnconfirmed):
		status, code = 503, "APPLICATION_OPERATION_UNCONFIRMED"
	case errors.Is(err, ErrResourceInvalid):
		status, code = 400, "APPLICATION_RESOURCE_INVALID"
	case errors.Is(err, ErrInvalid):
		status, code = 400, "COMMON_INVALID_ARGUMENT"
	}
	respond(w, r, status, code, value)
}
func (s *Service) finish(w http.ResponseWriter, r *http.Request, p session.Principal, result Result) {
	if r.Method == "GET" {
		if err := s.Authenticator.Renew(r.Context(), w, r, p); err != nil {
			fail(w, r, err)
			return
		}
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		err := s.Authenticator.Renew(ctx, w, r, p)
		cancel()
		if err != nil {
			session.ClearCookies(w)
			if s.Logger != nil {
				s.Logger.Warn("session renewal failed after committed application write", "request_id", httpserver.Metadata(r.Context()).RequestID)
			}
		}
	}
	if result.Location != "" {
		w.Header().Set("Location", result.Location)
	}
	respond(w, r, result.Status, "OK", result.Data)
}
func (s *Service) success(w http.ResponseWriter, r *http.Request, p session.Principal, value any) {
	raw, _ := json.Marshal(value)
	s.finish(w, r, p, Result{Status: 200, Data: raw})
}

// Token traversal rejects duplicates at every nesting level before decoding DTOs.
func jsonValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return ErrInvalid
			}
			seen[name] = true
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	default:
		return ErrInvalid
	}
	end, err := d.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrInvalid
	}
	return nil
}
func decode(w http.ResponseWriter, r *http.Request, kind string) (Input, bool) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		respond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return Input{}, false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		fail(w, r, ErrInvalid)
		return Input{}, false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if jsonValue(d) != nil {
		fail(w, r, ErrInvalid)
		return Input{}, false
	}
	if _, err := d.Token(); err != io.EOF {
		fail(w, r, ErrInvalid)
		return Input{}, false
	}
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		fail(w, r, ErrInvalid)
		return Input{}, false
	}
	required := []string{"operationId"}
	switch kind {
	case "application.create":
		required = append(required, "name")
	case "group.create":
		required = append(required, "name", "expectedPolicyRevision")
	case "group.update":
		required = append(required, "name", "enabled", "expectedPolicyRevision")
	case "members.replace":
		required = append(required, "memberIds", "expectedPolicyRevision")
	case "grants.replace":
		required = append(required, "grants", "expectedPolicyRevision")
	}
	if len(fields) != len(required) {
		fail(w, r, ErrInvalid)
		return Input{}, false
	}
	for _, name := range required {
		v, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			fail(w, r, ErrInvalid)
			return Input{}, false
		}
	}
	if kind == "grants.replace" {
		var grants []json.RawMessage
		if json.Unmarshal(fields["grants"], &grants) != nil {
			fail(w, r, ErrInvalid)
			return Input{}, false
		}
		for _, rawGrant := range grants {
			var grant map[string]json.RawMessage
			if json.Unmarshal(rawGrant, &grant) != nil || len(grant) != 5 {
				fail(w, r, ErrInvalid)
				return Input{}, false
			}
			for _, key := range []string{"resourceKind", "resourceId", "action", "rowScope", "fields"} {
				v, ok := grant[key]
				if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
					fail(w, r, ErrInvalid)
					return Input{}, false
				}
			}
		}
	}
	var in Input
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		fail(w, r, ErrInvalid)
		return Input{}, false
	}
	return in, true
}
func (s *Service) write(w http.ResponseWriter, r *http.Request, p session.Principal, kind, appID, groupID string) {
	in, ok := decode(w, r, kind)
	if !ok {
		return
	}
	m := httpserver.Metadata(r.Context())
	result, err := s.Application.Write(r.Context(), p, kind, appID, groupID, in, Metadata{RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: r.UserAgent()})
	if err != nil {
		fail(w, r, err)
		return
	}
	s.finish(w, r, p, result)
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Workflows != nil {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
		parts := strings.Split(path, "/")
		if path != r.URL.Path && len(parts) >= 4 && parts[1] == "forms" && parts[3] == "workflows" {
			s.Workflows.ServeHTTP(w, r)
			return
		}
	}
	if s.Records != nil {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
		parts := strings.Split(path, "/")
		if path != r.URL.Path && len(parts) >= 4 && parts[1] == "forms" && (parts[3] == "records" || parts[3] == "drafts") && !(len(parts) == 6 && parts[3] == "records" && parts[5] == "history") {
			s.Records.ServeHTTP(w, r)
			return
		}
	}
	if s.Definitions != nil {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
		parts := strings.Split(path, "/")
		if path != r.URL.Path && len(parts) >= 2 && (parts[1] == "structure" || parts[1] == "directories" || parts[1] == "tables" || parts[1] == "forms" || parts[1] == "member-candidates" || parts[1] == "department-candidates") {
			s.Definitions.ServeHTTP(w, r)
			return
		}
	}
	var err error
	r, err = httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := s.Authenticator.Authenticate(r, r.Method != "GET" && r.Method != "HEAD")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err = ExpectedActor(r, p); err != nil {
		fail(w, r, err)
		return
	}
	if s.Application == nil {
		fail(w, r, session.ErrUnavailable)
		return
	}
	if r.URL.RawQuery != "" {
		fail(w, r, ErrInvalid)
		return
	}
	if r.URL.Path == "/api/v1/applications" {
		switch r.Method {
		case "POST":
			s.write(w, r, p, "application.create", "", "")
			return
		case "GET":
			items, err := s.Application.List(r.Context(), p)
			if err != nil {
				fail(w, r, err)
				return
			}
			s.success(w, r, p, map[string]any{"items": items})
			return
		}
	}
	if path := strings.TrimPrefix(r.URL.Path, "/api/v1/application-operations/"); path != r.URL.Path {
		id, ok := canonicalID(path)
		if !ok {
			fail(w, r, ErrInvalid)
			return
		}
		if r.Method == "GET" {
			value, err := s.Application.Operation(r.Context(), p, id)
			if err != nil {
				fail(w, r, err)
				return
			}
			s.success(w, r, p, value)
			return
		}
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	if path == r.URL.Path {
		respond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	parts := strings.Split(path, "/")
	appID, ok := canonicalID(parts[0])
	if !ok {
		fail(w, r, ErrInvalid)
		return
	}
	if r.Method == "GET" && (len(parts) == 1 || len(parts) == 2 && parts[1] == "access") {
		app, access, err := s.Application.Get(r.Context(), p, appID)
		if err != nil {
			fail(w, r, err)
			return
		}
		if len(parts) == 1 {
			s.success(w, r, p, app)
		} else {
			s.success(w, r, p, access)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "permission-groups" {
		switch r.Method {
		case "POST":
			s.write(w, r, p, "group.create", appID, "")
			return
		case "GET":
			v, err := s.Application.Configuration(r.Context(), p, appID, "", "")
			if err != nil {
				fail(w, r, err)
				return
			}
			s.success(w, r, p, v)
			return
		}
	}
	if len(parts) >= 3 && parts[1] == "permission-groups" {
		groupID, ok := canonicalID(parts[2])
		if !ok {
			fail(w, r, ErrInvalid)
			return
		}
		if len(parts) == 3 && r.Method == "PUT" {
			s.write(w, r, p, "group.update", appID, groupID)
			return
		}
		if len(parts) == 4 && (parts[3] == "members" || parts[3] == "grants") {
			switch r.Method {
			case "PUT":
				s.write(w, r, p, parts[3]+".replace", appID, groupID)
				return
			case "GET":
				v, err := s.Application.Configuration(r.Context(), p, appID, groupID, parts[3])
				if err != nil {
					fail(w, r, err)
					return
				}
				s.success(w, r, p, v)
				return
			}
		}
	}
	respond(w, r, 404, "API_NOT_FOUND", nil)
}
