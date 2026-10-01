package personnel

import (
	"bytes"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"mime"
	"net/http"
	"time"
)

func isQuerySearch(r *http.Request) bool {
	return r.Method == "POST" && (r.URL.Path == "/api/v1/personnel/members/search" || r.URL.Path == "/api/v1/personnel/events/search")
}
func decodeQueryBody(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		respond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if err != nil || !draftJSONUnicode(raw) {
		fail(w, r, ErrInvalid)
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	parsed, err := filterJSON(d, 0)
	object, ok := parsed.(map[string]any)
	if err != nil || !ok {
		fail(w, r, ErrInvalid)
		return false
	}
	for name, value := range object {
		if value == nil {
			fail(w, r, ErrInvalid)
			return false
		}
		switch name {
		case "page", "pageSize":
			n, ok := value.(json.Number)
			v, e := n.Int64()
			if !ok || e != nil || v < 1 || v > maxSafeVersion || (name == "pageSize" && v > 100) {
				fail(w, r, ErrInvalid)
				return false
			}
		case "departmentId", "identityId":
			v, ok := value.(string)
			if !ok || !validID(v) {
				fail(w, r, ErrInvalid)
				return false
			}
		case "queryVersion":
			v, ok := value.(string)
			if !ok || v == "" || len(v) > 256 {
				fail(w, r, ErrInvalid)
				return false
			}
		case "action":
			v, ok := value.(string)
			if !ok || !activityActions[v] {
				fail(w, r, ErrInvalid)
				return false
			}
		case "sortBy":
			if value != "occurredAt" {
				fail(w, r, ErrInvalid)
				return false
			}
		case "sortDirection":
			if value != "asc" && value != "desc" {
				fail(w, r, ErrInvalid)
				return false
			}
		}
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		fail(w, r, ErrInvalid)
		return false
	}
	return true
}
func (s *Service) QueryHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool {
	member := r.URL.Path == "/api/v1/personnel/members" || r.URL.Path == "/api/v1/personnel/members/search"
	event := r.URL.Path == "/api/v1/personnel/events" || r.URL.Path == "/api/v1/personnel/events/search"
	if !member && !event {
		return false
	}
	post := isQuerySearch(r)
	legacy := r.Method == "GET" && (r.URL.Path == "/api/v1/personnel/members" || r.URL.Path == "/api/v1/personnel/events")
	if !post && !legacy {
		return false
	}
	if post {
		if _, err := query(r); err != nil {
			fail(w, r, err)
			return true
		}
	}
	if member {
		var in MemberQueryInput
		if post {
			var body MemberSearchInput
			if !decodeQueryBody(w, r, &body) {
				return true
			}
			in = MemberQueryInput{MemberQuery: MemberQuery{PageQuery: PageQuery{Page: body.Page, PageSize: body.PageSize, Search: body.Search}, DepartmentID: body.DepartmentID, IdentityID: body.IdentityID}, QueryOptions: QueryOptions{Filter: body.Filter, QueryVersion: body.QueryVersion}}
		} else {
			values, err := query(r, "page", "pageSize", "search", "departmentId", "identityId", "queryVersion")
			if err != nil {
				fail(w, r, err)
				return true
			}
			page, err := pageQuery(values)
			if err != nil {
				fail(w, r, err)
				return true
			}
			in = MemberQueryInput{MemberQuery: MemberQuery{PageQuery: page, DepartmentID: values.Get("departmentId"), IdentityID: values.Get("identityId")}, QueryOptions: QueryOptions{QueryVersion: values.Get("queryVersion")}}
		}
		if post && in.QueryVersion == "" && in.Page > 1 {
			fail(w, r, ErrInvalid)
			return true
		}
		result, err := s.Application.SearchMembers(r.Context(), p, in)
		if err != nil {
			fail(w, r, err)
		} else {
			s.finish(w, r, p, 200, result, "")
		}
		return true
	}
	var in EventQueryInput
	if post {
		var body EventSearchInput
		if !decodeQueryBody(w, r, &body) {
			return true
		}
		in = EventQueryInput{EventQuery: EventQuery{PageQuery: PageQuery{Page: body.Page, PageSize: body.PageSize, Search: body.Search}, Action: body.Action}, QueryOptions: QueryOptions{Filter: body.Filter, QueryVersion: body.QueryVersion}}
		if body.From != nil {
			in.From = *body.From
		}
		if body.To != nil {
			in.To = *body.To
		}
		if body.SortBy != "" || body.SortDirection != "" {
			if body.SortBy != "occurredAt" || (body.SortDirection != "asc" && body.SortDirection != "desc") {
				fail(w, r, ErrInvalid)
				return true
			}
			in.Sort = &QuerySort{Key: body.SortBy, Direction: body.SortDirection}
		}
	} else {
		values, err := query(r, "page", "pageSize", "search", "action", "from", "to", "queryVersion")
		if err != nil {
			fail(w, r, err)
			return true
		}
		page, err := pageQuery(values)
		if err != nil {
			fail(w, r, err)
			return true
		}
		in = EventQueryInput{EventQuery: EventQuery{PageQuery: page, Action: values.Get("action")}, QueryOptions: QueryOptions{QueryVersion: values.Get("queryVersion")}}
		for _, key := range []string{"from", "to"} {
			if values.Has(key) {
				v, err := time.Parse(time.RFC3339Nano, values.Get(key))
				if err != nil {
					fail(w, r, ErrInvalid)
					return true
				}
				if key == "from" {
					in.From = v
				} else {
					in.To = v
				}
			}
		}
	}
	if post && in.QueryVersion == "" && in.Page > 1 {
		fail(w, r, ErrInvalid)
		return true
	}
	result, err := s.Application.SearchEvents(r.Context(), p, in)
	if err != nil {
		fail(w, r, err)
	} else {
		s.finish(w, r, p, 200, result, "")
	}
	return true
}
func queryBusyResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(503)
	if r.Method == "HEAD" {
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    any               `json:"data"`
		Meta    map[string]string `json:"meta"`
	}{"COMMON_SERVICE_UNAVAILABLE", "数据持续变动，请稍后重试", nil, map[string]string{"requestId": httpserver.Metadata(r.Context()).RequestID, "reason": "QUERY_BUSY"}})
}
