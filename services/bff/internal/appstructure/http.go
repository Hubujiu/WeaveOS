package appstructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

func respond(w http.ResponseWriter, r *http.Request, status int, code string, value any, pagination ...Pagination) {
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
	meta := map[string]any{"requestId": httpserver.Metadata(r.Context()).RequestID}
	if len(pagination) > 0 {
		meta["pagination"] = pagination[0]
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Data    any            `json:"data"`
		Meta    map[string]any `json:"meta"`
	}{code, message, value, meta})
}
func fail(w http.ResponseWriter, r *http.Request, e error, operationID string) {
	status, code, value := 503, "COMMON_SERVICE_UNAVAILABLE", any(nil)
	var domain *Error
	switch {
	case errors.Is(e, applications.ErrExpectedActorInvalid):
		status, code, value = 400, "COMMON_VALIDATION_FAILED", applications.ActorHeaderViolation()
	case errors.Is(e, applications.ErrSessionChanged):
		status, code = 409, "AUTH_SESSION_CHANGED"
	case errors.As(e, &domain):
		code, value = domain.Code, domain.Data
		status = 409
		if code == "COMMON_VALIDATION_FAILED" {
			status = 400
		}
	case errors.Is(e, session.ErrUnauthorized):
		status, code = 401, "AUTH_UNAUTHENTICATED"
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	case errors.Is(e, session.ErrForbidden):
		status, code = 403, "COMMON_CSRF_REJECTED"
	case errors.Is(e, applications.ErrDenied):
		status, code = 403, "APPLICATION_FORBIDDEN"
	case errors.Is(e, applications.ErrMissing):
		status, code = 404, "APPLICATION_NOT_FOUND"
	case errors.Is(e, applications.ErrResourceInvalid):
		status, code = 400, "APPLICATION_RESOURCE_INVALID"
	case errors.Is(e, applications.ErrOperationConflict):
		status, code = 409, "APPLICATION_OPERATION_CONFLICT"
	case errors.Is(e, applications.ErrUnconfirmed):
		code = "APPLICATION_OPERATION_UNCONFIRMED"
		value = map[string]string{"operationId": operationID}
	case errors.Is(e, applications.ErrInvalid):
		status, code = 400, "COMMON_INVALID_ARGUMENT"
	}
	respond(w, r, status, code, value)
}
func jsonValue(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			t, e = d.Token()
			s, ok := t.(string)
			if e != nil || !ok || seen[s] {
				return invalid()
			}
			seen[s] = true
			if e = jsonValue(d); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e = jsonValue(d); e != nil {
				return e
			}
		}
	default:
		return invalid()
	}
	t, e = d.Token()
	if e != nil || delim == '{' && t != json.Delim('}') || delim == '[' && t != json.Delim(']') {
		return invalid()
	}
	return nil
}
func object(raw []byte, required, optional, nullable []string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, invalid()
	}
	allow, null := map[string]bool{}, map[string]bool{}
	for _, k := range required {
		allow[k] = true
		if _, ok := m[k]; !ok {
			return nil, invalid()
		}
	}
	for _, k := range optional {
		allow[k] = true
	}
	for _, k := range nullable {
		null[k] = true
	}
	for k, v := range m {
		if !allow[k] || !null[k] && bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, invalid()
		}
	}
	return m, nil
}
func layouts(raw json.RawMessage) error {
	var nodes []json.RawMessage
	if json.Unmarshal(raw, &nodes) != nil || nodes == nil {
		return invalid()
	}
	for _, n := range nodes {
		var tag struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(n, &tag) != nil {
			return invalid()
		}
		required := []string{"id", "kind"}
		optional := []string{}
		switch tag.Kind {
		case "field", "system_field":
			required = append(required, "fieldId")
			optional = append(optional, "span")
		case "group":
			required = append(required, "title", "children")
			optional = append(optional, "span")
		case "divider":
		case "description":
			required = append(required, "text")
		default:
			return invalid()
		}
		m, e := object(n, required, optional, nil)
		if e != nil {
			return e
		}
		if raw, ok := m["span"]; ok {
			var span int
			if json.Unmarshal(raw, &span) != nil || span < 1 || span > 12 {
				return invalid()
			}
		}
		if tag.Kind == "group" {
			if e = layouts(m["children"]); e != nil {
				return e
			}
		}
	}
	return nil
}
func decode(w http.ResponseWriter, r *http.Request, kind string) (Input, error) {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return Input{}, &Error{"COMMON_UNSUPPORTED_MEDIA_TYPE", nil}
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		return Input{}, invalid()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if jsonValue(d) != nil {
		return Input{}, invalid()
	}
	if _, e = d.Token(); e != io.EOF {
		return Input{}, invalid()
	}
	required := []string{"operationId"}
	nullable := []string{}
	if strings.HasPrefix(kind, "definition.") {
		required = []string{"expectedSchemaVersion", "expectedViewVersion", "fields", "layout", "optionMappings"}
		if kind == "definition.save" {
			required = append(required, "operationId", "confirmationToken")
			nullable = append(nullable, "confirmationToken")
		}
	} else {
		required = append(required, "name", "position", "expectedStructureVersion")
		if strings.HasPrefix(kind, "directory.") {
			required = append(required, "parentId")
			nullable = append(nullable, "parentId")
		} else {
			required = append(required, "directoryId")
			nullable = append(nullable, "directoryId")
		}
		if kind == "form.create" {
			required = append(required, "source")
		}
	}
	m, e := object(raw, required, nil, nullable)
	if e != nil {
		return Input{}, e
	}
	if strings.HasPrefix(kind, "definition.") {
		var fields []json.RawMessage
		if json.Unmarshal(m["fields"], &fields) != nil || fields == nil {
			return Input{}, invalid()
		}
		for _, f := range fields {
			field, e := object(f, []string{"id", "name", "kind", "required", "default", "config", "presentation"}, nil, []string{"default"})
			if e != nil {
				return Input{}, e
			}
			if _, e = object(field["presentation"], []string{"helpText"}, []string{"displayTimeZone"}, []string{"helpText", "displayTimeZone"}); e != nil {
				return Input{}, e
			}
		}
		if e = layouts(m["layout"]); e != nil {
			return Input{}, e
		}
		var mappings []json.RawMessage
		if json.Unmarshal(m["optionMappings"], &mappings) != nil || mappings == nil {
			return Input{}, invalid()
		}
		for _, mapping := range mappings {
			if _, e = object(mapping, []string{"fieldId", "fromOptionId", "toOptionId"}, nil, []string{"toOptionId"}); e != nil {
				return Input{}, e
			}
		}
	}
	if kind == "form.create" {
		var source Source
		if json.Unmarshal(m["source"], &source) != nil {
			return Input{}, invalid()
		}
		keys := []string{"kind"}
		if source.Kind == "existing_table" {
			keys = append(keys, "tableId")
		}
		if _, e = object(m["source"], keys, nil, nil); e != nil {
			return Input{}, e
		}
	}
	var in Input
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		return Input{}, invalid()
	}
	return normalize(kind, in)
}
func (s *Service) finish(w http.ResponseWriter, r *http.Request, p session.Principal, result applications.Result, write bool) {
	c := r.Context()
	cancel := func() {}
	if write {
		c, cancel = context.WithTimeout(c, 2*time.Second)
	}
	e := s.Authenticator.Renew(c, w, r, p)
	cancel()
	if e != nil {
		if !write {
			fail(w, r, e, "")
			return
		}
		session.ClearCookies(w)
	}
	if result.Location != "" {
		w.Header().Set("Location", result.Location)
	}
	respond(w, r, result.Status, "OK", result.Data)
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var e error
	r, e = httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if e != nil {
		fail(w, r, e, "")
		return
	}
	p, e := s.Authenticator.Authenticate(r, r.Method != "GET" && r.Method != "HEAD")
	if e != nil {
		fail(w, r, e, "")
		return
	}
	if e = applications.ExpectedActor(r, p); e != nil {
		fail(w, r, e, "")
		return
	}
	if s.Application == nil || s.Application.Pool == nil {
		fail(w, r, ErrUnavailable, "")
		return
	}
	if r.URL.RawQuery != "" && !strings.HasSuffix(r.URL.Path, "/member-candidates") && !strings.HasSuffix(r.URL.Path, "/department-candidates") && !strings.HasSuffix(r.URL.Path, "/history") {
		fail(w, r, invalid(), "")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) < 2 {
		respond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	app := parts[0]
	if !appfields.ValidID(app) {
		fail(w, r, invalid(), "")
		return
	}
	if len(parts) == 6 && parts[1] == "forms" && parts[3] == "records" && parts[5] == "history" && r.Method == "GET" {
		if !appfields.ValidID(parts[2]) || !appfields.ValidID(parts[4]) {
			fail(w, r, invalid(), "")
			return
		}
		s.history(w, r, p, app, parts[2], parts[4])
		return
	}
	if len(parts) == 4 && parts[1] == "forms" && parts[3] == "runtime" && r.Method == "GET" {
		if !appfields.ValidID(parts[2]) {
			fail(w, r, invalid(), "")
			return
		}
		value, e := s.Application.runtime(r.Context(), p, app, parts[2])
		if e != nil {
			fail(w, r, e, "")
			return
		}
		raw, e := json.Marshal(value)
		if e != nil {
			fail(w, r, ErrUnavailable, "")
			return
		}
		s.finish(w, r, p, applications.Result{Status: 200, Data: raw}, false)
		return
	}
	if len(parts) == 2 && (parts[1] == "member-candidates" || parts[1] == "department-candidates") && r.Method == "GET" {
		kind := "member"
		if parts[1] == "department-candidates" {
			kind = "department"
		}
		s.candidates(w, r, p, app, kind)
		return
	}
	id, kind := "", ""
	if len(parts) == 2 && r.Method == "POST" {
		kind = map[string]string{"directories": "directory.create", "tables": "table.create", "forms": "form.create"}[parts[1]]
	}
	if len(parts) >= 3 {
		id = parts[2]
		if !appfields.ValidID(id) {
			fail(w, r, invalid(), "")
			return
		}
		if len(parts) == 3 && r.Method == "PUT" {
			kind = map[string]string{"directories": "directory.update", "tables": "table.update", "forms": "form.update"}[parts[1]]
		}
		if parts[1] == "forms" && len(parts) == 4 && parts[3] == "definition" && r.Method == "PUT" {
			kind = "definition.save"
		}
		if parts[1] == "forms" && len(parts) == 5 && parts[3] == "definition" && parts[4] == "preflight" && r.Method == "POST" {
			kind = "definition.preflight"
		}
	}
	if kind != "" && kind != "definition.preflight" {
		in, e := decode(w, r, kind)
		if e != nil {
			var d *Error
			if errors.As(e, &d) && d.Code == "COMMON_UNSUPPORTED_MEDIA_TYPE" {
				respond(w, r, 415, d.Code, nil)
			} else {
				fail(w, r, e, "")
			}
			return
		}
		result, e := s.Application.write(r.Context(), p, app, id, kind, in)
		if e != nil {
			fail(w, r, e, in.OperationID)
			return
		}
		s.finish(w, r, p, result, true)
		return
	}
	allowed := r.Method == "GET" && (len(parts) == 2 && parts[1] == "structure" || len(parts) == 3 && (parts[1] == "directories" || parts[1] == "tables" || parts[1] == "forms") || len(parts) == 4 && parts[1] == "forms" && parts[3] == "definition") || kind == "definition.preflight"
	if !allowed {
		respond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	tx, e := s.Application.read(r.Context(), p, app)
	if e != nil {
		fail(w, r, e, "")
		return
	}
	defer tx.Rollback(context.Background())
	var value any
	switch {
	case kind == "definition.preflight":
		var in Input
		in, e = decode(w, r, kind)
		if e == nil {
			var d Definition
			d, e = definition(r.Context(), tx, app, id, false)
			if e == nil {
				e = s.Application.references(r.Context(), tx, app, id, in)
				if e == nil {
					value, e = s.Application.inspect(r.Context(), tx, p, d, in, true)
				}
			}
		}
	case len(parts) == 2:
		value, e = structure(r.Context(), tx, app)
	case len(parts) == 4:
		value, e = definition(r.Context(), tx, app, id, false)
	case parts[1] == "tables":
		value, _, e = loadTable(r.Context(), tx, app, id, false)
	case parts[1] == "forms":
		var form Form
		form, _, e = loadForm(r.Context(), tx, app, id, false)
		if e == nil {
			var table Table
			table, _, e = loadTable(r.Context(), tx, app, form.TableID, false)
			var version int64
			if e == nil {
				e = tx.QueryRow(r.Context(), "SELECT structure_version FROM applications.apps WHERE id=$1", app).Scan(&version)
			}
			value = map[string]any{"table": table, "form": form, "structureVersion": version}
		}
	case parts[1] == "directories":
		var dir Directory
		var version int64
		e = tx.QueryRow(r.Context(), "SELECT d.id::text,d.app_id::text,d.name,d.parent_id::text,d.position,a.structure_version FROM applications.directories d JOIN applications.apps a ON a.id=d.app_id WHERE d.app_id=$1 AND d.id=$2", app, id).Scan(&dir.ID, &dir.AppID, &dir.Name, &dir.ParentID, &dir.Position, &version)
		e = missing(e)
		value = map[string]any{"directory": dir, "structureVersion": version}
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		var d *Error
		if errors.As(e, &d) && d.Code == "COMMON_UNSUPPORTED_MEDIA_TYPE" {
			respond(w, r, 415, d.Code, nil)
		} else {
			fail(w, r, e, "")
		}
		return
	}
	raw, _ := json.Marshal(value)
	s.finish(w, r, p, applications.Result{Status: 200, Data: raw}, false)
}
