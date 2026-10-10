package apptemplates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

type HTTP struct {
	Application       *Service
	Authenticator     session.Authenticator
	TrustedProxyHosts []string
}

var errTooLarge = errors.New("template payload exceeds limit")

type importWire struct {
	OperationID string          `json:"operationId"`
	Manifest    json.RawMessage `json:"manifest"`
	Bindings    []Binding       `json:"bindings"`
}

type preflightWire struct {
	Manifest json.RawMessage `json:"manifest"`
	Bindings []Binding       `json:"bindings"`
}

func templateRespond(w http.ResponseWriter, r *http.Request, status int, code string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method == "HEAD" {
		return
	}
	message := "请求未完成"
	if code == "OK" {
		message = "success"
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{code, message, data, map[string]string{"requestId": httpserver.Metadata(r.Context()).RequestID}})
}
func templateFail(w http.ResponseWriter, r *http.Request, e error) {
	status, code, data := 503, "COMMON_SERVICE_UNAVAILABLE", any(nil)
	switch {
	case errors.Is(e, session.ErrUnauthorized):
		status, code = 401, "AUTH_UNAUTHENTICATED"
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	case errors.Is(e, session.ErrForbidden):
		status, code = 403, "COMMON_CSRF_REJECTED"
	case errors.Is(e, applications.ErrExpectedActorInvalid):
		status, code, data = 400, "COMMON_VALIDATION_FAILED", applications.ActorHeaderViolation()
	case errors.Is(e, applications.ErrSessionChanged):
		status, code = 409, "AUTH_SESSION_CHANGED"
	case errors.Is(e, applications.ErrDenied):
		status, code = 403, "APPLICATION_FORBIDDEN"
	case errors.Is(e, applications.ErrOperationConflict):
		status, code = 409, "APPLICATION_OPERATION_CONFLICT"
	case errors.Is(e, applications.ErrPolicyConflict):
		status, code = 409, "APPLICATION_POLICY_CONFLICT"
	case errors.Is(e, applications.ErrMissing):
		status, code = 404, "APPLICATION_NOT_FOUND"
	case errors.Is(e, applications.ErrResourceInvalid):
		status, code = 400, "APPLICATION_RESOURCE_INVALID"
	case errors.Is(e, errTooLarge):
		status, code = 413, "APPLICATION_TEMPLATE_TOO_LARGE"
	case errors.Is(e, ErrNotExportable):
		status, code = 409, "APPLICATION_TEMPLATE_NOT_EXPORTABLE"
	case errors.Is(e, ErrInvalid) || errors.Is(e, applications.ErrInvalid):
		status, code = 400, "COMMON_INVALID_ARGUMENT"
	}
	templateRespond(w, r, status, code, data)
}
func decodePreflight(raw []byte) (Manifest, []Binding, error) {
	if len(raw) > 4*1024*1024 {
		return Manifest{}, nil, errTooLarge
	}
	if len(raw) == 0 || !utf8.Valid(raw) {
		return Manifest{}, nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if tokens(d, 0) != nil {
		return Manifest{}, nil, ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return Manifest{}, nil, ErrInvalid
	}
	if !shape(raw, reflect.TypeOf(preflightWire{}), 0) {
		return Manifest{}, nil, ErrInvalid
	}
	var in preflightWire
	if json.Unmarshal(raw, &in) != nil {
		return Manifest{}, nil, ErrInvalid
	}
	if len(in.Manifest) > 1048576 {
		return Manifest{}, nil, errTooLarge
	}
	m, e := DecodeManifest(in.Manifest)
	return m, in.Bindings, e
}
func decodeImport(raw []byte) (string, Manifest, []Binding, error) {
	if len(raw) > 4*1024*1024 {
		return "", Manifest{}, nil, errTooLarge
	}
	if len(raw) == 0 || !utf8.Valid(raw) {
		return "", Manifest{}, nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if tokens(d, 0) != nil {
		return "", Manifest{}, nil, ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return "", Manifest{}, nil, ErrInvalid
	}
	if !shape(raw, reflect.TypeOf(importWire{}), 0) {
		return "", Manifest{}, nil, ErrInvalid
	}
	var in importWire
	if json.Unmarshal(raw, &in) != nil || !validID(in.OperationID) {
		return "", Manifest{}, nil, ErrInvalid
	}
	if len(in.Manifest) > 1048576 {
		return "", Manifest{}, nil, errTooLarge
	}
	m, e := DecodeManifest(in.Manifest)
	return in.OperationID, m, in.Bindings, e
}
func (s *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var e error
	r, e = httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if e != nil {
		templateFail(w, r, e)
		return
	}
	p, e := s.Authenticator.Authenticate(r, r.Method != "GET" && r.Method != "HEAD")
	if e != nil {
		templateFail(w, r, e)
		return
	}
	if e = applications.ExpectedActor(r, p); e != nil {
		templateFail(w, r, e)
		return
	}
	if s.Application == nil || s.Application.Pool == nil {
		templateFail(w, r, session.ErrUnavailable)
		return
	}
	if r.URL.RawQuery != "" {
		templateFail(w, r, ErrInvalid)
		return
	}
	preflight := r.URL.Path == "/api/v1/application-templates/preflight"
	importing := r.URL.Path == "/api/v1/application-templates/import"
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	export := path != r.URL.Path && len(parts) == 2 && parts[1] == "structure-template"
	if !preflight && !export && !importing {
		templateRespond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	if (preflight || importing) && r.Method != "POST" || export && r.Method != "GET" && r.Method != "HEAD" {
		allow := "POST"
		if export {
			allow = "GET, HEAD"
		}
		w.Header().Set("Allow", allow)
		templateRespond(w, r, 405, "COMMON_METHOD_NOT_ALLOWED", nil)
		return
	}
	var result any
	if export {
		result, e = s.Application.ExportManifest(r.Context(), p, parts[0])
	} else {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		encoding := r.Header.Get("Content-Encoding")
		if err != nil || media != "application/json" || encoding != "" && encoding != "identity" {
			templateRespond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024+1))
		if err != nil {
			templateFail(w, r, ErrInvalid)
			return
		}
		if importing {
			op, m, b, err := decodeImport(raw)
			if err != nil {
				templateFail(w, r, err)
				return
			}
			meta := httpserver.Metadata(r.Context())
			committed, err := s.Application.Import(r.Context(), p, op, m, b, applications.Metadata{RequestID: meta.RequestID, ClientIP: meta.ClientIP, UserAgent: meta.UserAgent})
			if errors.Is(err, applications.ErrUnconfirmed) {
				templateRespond(w, r, 503, "APPLICATION_OPERATION_UNCONFIRMED", map[string]string{"operationId": op})
				return
			}
			if err != nil {
				templateFail(w, r, err)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			err = s.Authenticator.Renew(ctx, w, r, p)
			cancel()
			if err != nil {
				session.ClearCookies(w)
			}
			w.Header().Set("Location", committed.Location)
			templateRespond(w, r, committed.Status, "OK", committed.Data)
			return
		}
		m, b, err := decodePreflight(raw)
		if err != nil {
			templateFail(w, r, err)
			return
		}
		result, e = s.Application.Preflight(r.Context(), p, m, b)
	}
	if e != nil {
		templateFail(w, r, e)
		return
	}
	if e = s.Authenticator.Renew(r.Context(), w, r, p); e != nil {
		templateFail(w, r, e)
		return
	}
	templateRespond(w, r, 200, "OK", result)
}
