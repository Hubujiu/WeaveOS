package appstructure

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type historyCursor struct {
	Actor, SessionRef, App, Table, View, Record string
	PolicyRevision, SchemaVersion               int64
	PageSize                                    int
	AfterTime                                   time.Time
	AfterID                                     string
	Expires                                     int64
}

func (a *Application) historyCursorKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return a.CandidateNamespace + ":record-history:v1:" + hex.EncodeToString(digest[:])
}
func (a *Application) loadHistoryCursor(c context.Context, token string, want historyCursor) (historyCursor, error) {
	if a.CandidateRedis == nil || a.CandidateNamespace == "" {
		return historyCursor{}, ErrUnavailable
	}
	bytes, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(bytes) != 32 || base64.RawURLEncoding.EncodeToString(bytes) != token {
		return historyCursor{}, invalid()
	}
	raw, e := a.CandidateRedis.Get(c, a.historyCursorKey(token)).Bytes()
	if errors.Is(e, redis.Nil) {
		return historyCursor{}, invalid()
	}
	if e != nil {
		return historyCursor{}, ErrUnavailable
	}
	var cursor historyCursor
	if json.Unmarshal(raw, &cursor) != nil || cursor.Expires <= a.now().Unix() || cursor.Actor != want.Actor || cursor.SessionRef != want.SessionRef || cursor.App != want.App || cursor.Table != want.Table || cursor.View != want.View || cursor.Record != want.Record || cursor.PolicyRevision != want.PolicyRevision || cursor.SchemaVersion != want.SchemaVersion || cursor.PageSize != want.PageSize || cursor.AfterTime.IsZero() || !appfields.ValidID(cursor.AfterID) {
		return historyCursor{}, invalid()
	}
	return cursor, nil
}
func (a *Application) storeHistoryCursor(c context.Context, cursor historyCursor) (string, error) {
	if a.CandidateRedis == nil || a.CandidateNamespace == "" {
		return "", ErrUnavailable
	}
	bytes := make([]byte, 32)
	if _, e := rand.Read(bytes); e != nil {
		return "", ErrUnavailable
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	cursor.Expires = a.now().Add(10 * time.Minute).Unix()
	raw, e := json.Marshal(cursor)
	if e != nil {
		return "", ErrUnavailable
	}
	if e = a.CandidateRedis.Set(c, a.historyCursorKey(token), raw, 10*time.Minute).Err(); e != nil {
		return "", ErrUnavailable
	}
	return token, nil
}
func rowScope(scope, actor, creator string) bool {
	return scope == "all" || scope == "own" && actor == creator
}
func (s *Service) history(w http.ResponseWriter, r *http.Request, p session.Principal, app, view, record string) {
	query, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		fail(w, r, invalid(), "")
		return
	}
	for k, v := range query {
		if len(v) != 1 || k != "pageSize" && k != "pageToken" || v[0] == "" {
			fail(w, r, invalid(), "")
			return
		}
	}
	size := 20
	if v, ok := query["pageSize"]; ok {
		size, e = strconv.Atoi(v[0])
		if e != nil || size < 1 || size > 100 || strconv.Itoa(size) != v[0] {
			fail(w, r, invalid(), "")
			return
		}
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
	if !validScope(access.Read) || !validScope(access.History) {
		fail(w, r, ErrUnavailable, "")
		return
	}
	if access.Read == "none" {
		fail(w, r, applications.ErrMissing, "")
		return
	}
	if !facts.SchemaReady {
		fail(w, r, &Error{Code: "APPLICATION_SCHEMA_NOT_READY"}, "")
		return
	}
	var creator string
	e = tx.QueryRow(r.Context(), "SELECT created_by::text FROM "+tablePhysical(facts.TableID)+" WHERE id=$1", record).Scan(&creator)
	if errors.Is(e, pgx.ErrNoRows) || e == nil && !rowScope(access.Read, p.UserID, creator) {
		fail(w, r, applications.ErrMissing, "")
		return
	}
	if e != nil {
		fail(w, r, ErrUnavailable, "")
		return
	}
	if !rowScope(access.History, p.UserID, creator) {
		fail(w, r, applications.ErrDenied, "")
		return
	}
	ids := []string{}
	for id, mask := range access.Fields {
		if !validScope(mask.Read) || !validScope(mask.History) {
			fail(w, r, ErrUnavailable, "")
			return
		}
		if rowScope(mask.Read, p.UserID, creator) && rowScope(mask.History, p.UserID, creator) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	cursor := historyCursor{Actor: p.UserID, SessionRef: p.Record.SessionRef, App: app, Table: facts.TableID, View: view, Record: record, PolicyRevision: facts.App.PolicyRevision, SchemaVersion: facts.SchemaVersion, PageSize: size}
	read := HistoryRead{AppID: app, TableID: facts.TableID, ViewID: view, RecordID: record, FieldIDs: ids, AllFields: facts.Actor.BootstrapAdmin || facts.Actor.ID == facts.App.OwnerUserID, PageSize: size}
	if token := query.Get("pageToken"); token != "" {
		cursor, e = s.Application.loadHistoryCursor(r.Context(), token, cursor)
		if e != nil {
			fail(w, r, e, "")
			return
		}
		read.AfterTime = &cursor.AfterTime
		read.AfterID = &cursor.AfterID
	}
	page, e := (RecordHistoryStore{}).Page(r.Context(), tx, read)
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, r, e, "")
		return
	}
	pagination := Pagination{HasMore: page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		cursor.AfterTime = last.OccurredAt
		cursor.AfterID = last.ID
		token, e := s.Application.storeHistoryCursor(r.Context(), cursor)
		if e != nil {
			fail(w, r, e, "")
			return
		}
		pagination.NextPageToken = &token
	}
	if e = s.Authenticator.Renew(r.Context(), w, r, p); e != nil {
		fail(w, r, e, "")
		return
	}
	respond(w, r, 200, "OK", page, pagination)
}
