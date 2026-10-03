package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

func (s *Service) referenceCandidates(w http.ResponseWriter, r *http.Request, p session.Principal, app, view string) {
	query, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		fail(w, r, invalid(), "")
		return
	}
	for k, v := range query {
		if len(v) != 1 || k != "fieldId" && k != "action" && k != "recordId" && k != "q" && k != "pageSize" && k != "pageToken" {
			fail(w, r, invalid(), "")
			return
		}
	}
	field, action, record := query.Get("fieldId"), query.Get("action"), query.Get("recordId")
	if !appfields.ValidID(field) || action != "create" && action != "edit" || action == "edit" && !appfields.ValidID(record) || action == "create" && query.Has("recordId") {
		fail(w, r, invalid(), "")
		return
	}
	size := 20
	if query.Has("pageSize") {
		size, e = strconv.Atoi(query.Get("pageSize"))
		if e != nil || size < 1 || size > 50 || strconv.Itoa(size) != query.Get("pageSize") {
			fail(w, r, invalid(), "")
			return
		}
	}
	q := strings.TrimSpace(query.Get("q"))
	if !utf8.ValidString(q) || utf8.RuneCountInString(q) > 100 || strings.ContainsRune(q, 0) || query.Has("pageToken") && query.Get("pageToken") == "" {
		fail(w, r, invalid(), "")
		return
	}
	tx, facts, e := (&applications.Application{Pool: s.Application.Pool}).BeginRecordRead(r.Context(), p, app, view)
	if e != nil {
		fail(w, r, e, "")
		return
	}
	defer tx.Rollback(context.Background())
	access, e := s.Application.recordAccess(r.Context(), tx, facts)
	if e != nil {
		fail(w, r, e, "")
		return
	}
	if !access.MenuEnter {
		fail(w, r, applications.ErrDenied, "")
		return
	}
	var fields []appfields.Field
	if json.Unmarshal(facts.Fields, &fields) != nil {
		fail(w, r, ErrUnavailable, "")
		return
	}
	kind := ""
	for _, f := range fields {
		if f.ID == field {
			kind = f.Kind
			break
		}
	}
	if kind != "member" && kind != "department" {
		fail(w, r, applications.ErrResourceInvalid, "")
		return
	}
	mask, ok := access.Fields[field]
	if action == "create" {
		if !ok || !access.Create || !mask.Create {
			fail(w, r, applications.ErrDenied, "")
			return
		}
	} else {
		if !ok || !validScope(mask.Edit) || mask.Edit == "none" {
			fail(w, r, applications.ErrDenied, "")
			return
		}
		var creator string
		e = tx.QueryRow(r.Context(), "SELECT created_by::text FROM "+tablePhysical(facts.TableID)+" WHERE id=$1", record).Scan(&creator)
		if errors.Is(e, pgx.ErrNoRows) {
			fail(w, r, applications.ErrMissing, "")
			return
		}
		if e != nil {
			fail(w, r, ErrUnavailable, "")
			return
		}
		if !rowScope(mask.Edit, p.UserID, creator) {
			fail(w, r, applications.ErrDenied, "")
			return
		}
	}
	if !facts.SchemaReady {
		fail(w, r, &Error{Code: "APPLICATION_SCHEMA_NOT_READY"}, "")
		return
	}
	cursor := candidateCursor{Actor: p.UserID, App: app, Query: q, Kind: kind, View: view, Field: field, Action: action, Record: record, PolicyRevision: facts.App.PolicyRevision}
	items, page, e := s.Application.candidatePage(r.Context(), tx, cursor, size, query.Get("pageToken"))
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e == nil {
		e = s.Authenticator.Renew(r.Context(), w, r, p)
	}
	if e != nil {
		fail(w, r, e, "")
		return
	}
	respond(w, r, 200, "OK", map[string]any{"items": items}, page)
}
