package personnel

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	Application       *Application
	Authenticator     session.Authenticator
	Logger            *slog.Logger
	TrustedProxyHosts []string
}

func (s *Service) TrustedProxies() []string { return append([]string(nil), s.TrustedProxyHosts...) }
func (s *Service) finish(w http.ResponseWriter, r *http.Request, p session.Principal, status int, data any, location string) {
	if status >= 200 && status < 300 {
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
					s.Logger.Warn("session renewal failed after committed personnel write", "request_id", httpserver.Metadata(r.Context()).RequestID)
				}
			}
		}
	}
	if location != "" {
		w.Header().Set("Location", location)
	}
	respond(w, r, status, "OK", data)
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if s.Application == nil {
		fail(w, r, session.ErrUnavailable)
		return
	}
	if s.DraftHTTP(w, r, p) {
		return
	}
	meta := RequestMetadata{RequestID: httpserver.Metadata(r.Context()).RequestID, ClientIP: httpserver.Metadata(r.Context()).ClientIP, UserAgent: r.UserAgent()}
	if r.URL.Path == "/api/v1/me/access" && r.Method == "GET" {
		if _, err := query(r); err != nil {
			fail(w, r, err)
			return
		}
		value, err := s.Application.Me(r.Context(), p)
		if err != nil {
			fail(w, r, err)
			return
		}
		s.finish(w, r, p, 200, value, "")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/personnel/")
	if path == r.URL.Path {
		respond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		switch parts[0] {
		case "departments":
			if r.Method == "GET" {
				if _, err := query(r); err != nil {
					fail(w, r, err)
					return
				}
				items, err := s.Application.Departments(r.Context(), p)
				if err != nil {
					fail(w, r, err)
					return
				}
				s.finish(w, r, p, 200, map[string]any{"items": items}, "")
				return
			}
			if r.Method == "POST" {
				var in struct {
					Name     string `json:"name"`
					ParentID string `json:"parentId"`
				}
				if !decodeBody(w, r, &in, "name", "parentId") {
					return
				}
				if _, err := query(r); err != nil {
					fail(w, r, err)
					return
				}
				value, err := s.Application.SaveDepartment(r.Context(), p, "", DepartmentInput{Name: in.Name, ParentID: in.ParentID}, meta)
				if err != nil {
					fail(w, r, err)
					return
				}
				s.finish(w, r, p, 201, value, "/api/v1/personnel/departments/"+value.ID)
				return
			}
		case "members":
			if r.Method == "GET" {
				values, err := query(r, "page", "pageSize", "search", "departmentId", "identityId")
				if err != nil {
					fail(w, r, err)
					return
				}
				page, err := pageQuery(values)
				if err != nil {
					fail(w, r, err)
					return
				}
				value, err := s.Application.ListMembers(r.Context(), p, MemberQuery{PageQuery: page, DepartmentID: values.Get("departmentId"), IdentityID: values.Get("identityId")})
				if err != nil {
					fail(w, r, err)
					return
				}
				s.finish(w, r, p, 200, value, "")
				return
			}
		case "permissions":
			if r.Method == "GET" {
				if _, err := query(r); err != nil {
					fail(w, r, err)
					return
				}
				items, err := s.Application.Catalog(r.Context(), p)
				if err != nil {
					fail(w, r, err)
					return
				}
				s.finish(w, r, p, 200, map[string]any{"items": items}, "")
				return
			}
		case "events":
			if r.Method == "GET" {
				values, err := query(r, "page", "pageSize", "search", "action", "from", "to")
				if err != nil {
					fail(w, r, err)
					return
				}
				page, err := pageQuery(values)
				if err != nil {
					fail(w, r, err)
					return
				}
				in := EventQuery{PageQuery: page, Action: values.Get("action")}
				for _, field := range []string{"from", "to"} {
					if raw, ok := values[field]; ok {
						value, err := time.Parse(time.RFC3339, raw[0])
						if err != nil {
							fail(w, r, ErrInvalid)
							return
						}
						if field == "from" {
							in.From = value
						} else {
							in.To = value
						}
					}
				}
				value, err := s.Application.Events(r.Context(), p, in)
				if err != nil {
					fail(w, r, err)
					return
				}
				s.finish(w, r, p, 200, value, "")
				return
			}
		}
	}
	if parts[0] == "identities" || parts[0] == "templates" {
		s.definitionHTTP(w, r, p, parts, meta)
		return
	}
	if len(parts) == 2 && parts[0] == "departments" {
		if r.Method == "PUT" {
			var in struct {
				Name    string `json:"name"`
				Version int64  `json:"version"`
			}
			if !decodeBody(w, r, &in, "name", "version") {
				return
			}
			if _, err := query(r); err != nil {
				fail(w, r, err)
				return
			}
			value, err := s.Application.SaveDepartment(r.Context(), p, parts[1], DepartmentInput{Name: in.Name, Version: in.Version}, meta)
			if err != nil {
				fail(w, r, err)
				return
			}
			s.finish(w, r, p, 200, value, "")
			return
		}
		if r.Method == "DELETE" {
			version, err := deleteVersion(r)
			if err == nil {
				err = s.Application.DeleteDepartment(r.Context(), p, parts[1], version, meta)
			}
			if err != nil {
				fail(w, r, err)
				return
			}
			s.finish(w, r, p, 204, nil, "")
			return
		}
	}
	if len(parts) == 2 && parts[0] == "members" && r.Method == "GET" {
		if _, err := query(r); err != nil {
			fail(w, r, err)
			return
		}
		value, err := s.Application.GetMember(r.Context(), p, parts[1])
		if err != nil {
			fail(w, r, err)
			return
		}
		s.finish(w, r, p, 200, value, "")
		return
	}
	if len(parts) == 3 && parts[0] == "members" {
		if _, err := query(r); err != nil {
			fail(w, r, err)
			return
		}
		if parts[2] == "identities" && r.Method == "PUT" {
			var in struct {
				IdentityIDs []string `json:"identityIds"`
				Version     int64    `json:"version"`
			}
			if !decodeBody(w, r, &in, "identityIds", "version") {
				return
			}
			value, err := s.Application.SetMemberIdentities(r.Context(), p, parts[1], in.IdentityIDs, in.Version, meta)
			if err != nil {
				fail(w, r, err)
				return
			}
			s.finish(w, r, p, 200, value, "")
			return
		}
		if parts[2] == "groups" && r.Method == "POST" {
			var in struct {
				Operation          string `json:"operation"`
				DepartmentID       string `json:"departmentId"`
				SourceDepartmentID string `json:"sourceDepartmentId"`
				Version            int64  `json:"version"`
			}
			if !decodeBody(w, r, &in, "operation", "departmentId", "version") {
				return
			}
			value, err := s.Application.ChangeMemberGroups(r.Context(), p, parts[1], GroupInput{in.Operation, in.DepartmentID, in.SourceDepartmentID, in.Version}, meta)
			if err != nil {
				fail(w, r, err)
				return
			}
			s.finish(w, r, p, 200, value, "")
			return
		}
	}
	respond(w, r, 404, "API_NOT_FOUND", nil)
}
func (s *Service) definitionHTTP(w http.ResponseWriter, r *http.Request, p session.Principal, parts []string, meta RequestMetadata) {
	kind := Identity
	if parts[0] == "templates" {
		kind = Template
	}
	id := ""
	if len(parts) == 2 {
		id = parts[1]
	}
	if len(parts) > 2 {
		respond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	if r.Method == "GET" {
		if id == "" {
			values, err := query(r, "page", "pageSize", "search")
			if err != nil {
				fail(w, r, err)
				return
			}
			page, err := pageQuery(values)
			if err != nil {
				fail(w, r, err)
				return
			}
			value, err := s.Application.ListDefinitions(r.Context(), p, kind, page)
			if err != nil {
				fail(w, r, err)
				return
			}
			s.finish(w, r, p, 200, value, "")
			return
		}
		if _, err := query(r); err != nil {
			fail(w, r, err)
			return
		}
		value, err := s.Application.GetDefinition(r.Context(), p, kind, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		s.finish(w, r, p, 200, value, "")
		return
	}
	if r.Method == "DELETE" && id != "" {
		version, err := deleteVersion(r)
		if err == nil {
			err = s.Application.DeleteDefinition(r.Context(), p, kind, id, version, meta)
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		s.finish(w, r, p, 204, nil, "")
		return
	}
	if r.Method == "POST" && id == "" || r.Method == "PUT" && id != "" {
		if _, err := query(r); err != nil {
			fail(w, r, err)
			return
		}
		required := []string{"name", "description", "permissionCodes"}
		if id != "" {
			required = append(required, "version")
		}
		var in DefinitionInput
		var suppliedVersion *int64
		if kind == Identity {
			required = append(required, "templateIds")
			var body struct {
				Name            string   `json:"name"`
				Description     string   `json:"description"`
				Version         *int64   `json:"version"`
				PermissionCodes []string `json:"permissionCodes"`
				TemplateIDs     []string `json:"templateIds"`
			}
			if !decodeBody(w, r, &body, required...) {
				return
			}
			suppliedVersion = body.Version
			in = DefinitionInput{Name: body.Name, Description: body.Description, PermissionCodes: body.PermissionCodes, TemplateIDs: body.TemplateIDs}
		} else {
			var body struct {
				Name            string   `json:"name"`
				Description     string   `json:"description"`
				Version         *int64   `json:"version"`
				PermissionCodes []string `json:"permissionCodes"`
			}
			if !decodeBody(w, r, &body, required...) {
				return
			}
			suppliedVersion = body.Version
			in = DefinitionInput{Name: body.Name, Description: body.Description, PermissionCodes: body.PermissionCodes}
		}
		// Creation accepts exactly the create contract; an edit version is not a create field.
		if id == "" && suppliedVersion != nil {
			fail(w, r, ErrInvalid)
			return
		}
		if suppliedVersion != nil {
			in.Version = *suppliedVersion
		}
		value, err := s.Application.SaveDefinition(r.Context(), p, kind, id, in, meta)
		if err != nil {
			fail(w, r, err)
			return
		}
		status, location := 200, ""
		if id == "" {
			status = 201
			location = "/api/v1/personnel/" + parts[0] + "/" + value.ID
		}
		s.finish(w, r, p, status, value, location)
		return
	}
	respond(w, r, 404, "API_NOT_FOUND", nil)
}
