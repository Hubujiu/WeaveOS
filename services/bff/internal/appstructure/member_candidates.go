package appstructure

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Pagination struct {
	NextPageToken *string `json:"nextPageToken"`
	HasMore       bool    `json:"hasMore"`
}
type MemberCandidate struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
}
type candidateCursor struct {
	Actor, App, Query, Account, ID string
	Expires                        int64
}

func (a *Application) cursorKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return a.CandidateNamespace + ":member-candidates:v1:" + hex.EncodeToString(digest[:])
}
func (a *Application) loadCursor(c context.Context, token string, expected candidateCursor) (candidateCursor, error) {
	if a.CandidateRedis == nil || a.CandidateNamespace == "" {
		return candidateCursor{}, ErrUnavailable
	}
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return candidateCursor{}, invalid()
	}
	data, e := a.CandidateRedis.Get(c, a.cursorKey(token)).Bytes()
	if errors.Is(e, redis.Nil) {
		return candidateCursor{}, invalid()
	}
	if e != nil {
		return candidateCursor{}, ErrUnavailable
	}
	var cursor candidateCursor
	if json.Unmarshal(data, &cursor) != nil || cursor.Expires <= a.now().Unix() || cursor.Actor != expected.Actor || cursor.App != expected.App || cursor.Query != expected.Query {
		return candidateCursor{}, invalid()
	}
	return cursor, nil
}
func (a *Application) storeCursor(c context.Context, cursor candidateCursor) (string, error) {
	if a.CandidateRedis == nil || a.CandidateNamespace == "" {
		return "", ErrUnavailable
	}
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return "", e
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	cursor.Expires = a.now().Add(10 * time.Minute).Unix()
	data, _ := json.Marshal(cursor)
	if e := a.CandidateRedis.Set(c, a.cursorKey(token), data, 10*time.Minute).Err(); e != nil {
		return "", ErrUnavailable
	}
	return token, nil
}
func (s *Service) candidates(w http.ResponseWriter, r *http.Request, p session.Principal, app string) {
	query, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		fail(w, r, invalid(), "")
		return
	}
	for k, values := range query {
		if len(values) != 1 || k != "q" && k != "pageSize" && k != "pageToken" {
			fail(w, r, invalid(), "")
			return
		}
	}
	q := strings.TrimSpace(query.Get("q"))
	size := 20
	if text, ok := query["pageSize"]; ok {
		if text[0] == "" {
			fail(w, r, invalid(), "")
			return
		}
		size, e = strconv.Atoi(text[0])
		if e != nil || size < 1 || size > 50 || strconv.Itoa(size) != text[0] {
			fail(w, r, invalid(), "")
			return
		}
	}
	if utf8.RuneCountInString(q) > 100 || !utf8.ValidString(q) || strings.ContainsRune(q, 0) {
		fail(w, r, invalid(), "")
		return
	}
	tx, e := s.Application.read(r.Context(), p, app)
	if e != nil {
		fail(w, r, e, "")
		return
	}
	defer tx.Rollback(context.Background())
	cursor := candidateCursor{Actor: p.UserID, App: app, Query: q}
	var afterAccount, afterID any
	if token := query.Get("pageToken"); token != "" {
		cursor, e = s.Application.loadCursor(r.Context(), token, cursor)
		if e != nil {
			fail(w, r, e, "")
			return
		}
		afterAccount, afterID = cursor.Account, cursor.ID
	}
	rows, e := tx.Query(r.Context(), "SELECT id::text,account,status FROM auth.users WHERE status='active' AND starts_with(account,$1) AND ($2::text IS NULL OR (account,id)>($2,$3::uuid)) ORDER BY account,id LIMIT $4", q, afterAccount, afterID, size+1)
	if e != nil {
		fail(w, r, e, "")
		return
	}
	items := []MemberCandidate{}
	for rows.Next() {
		var item MemberCandidate
		if e = rows.Scan(&item.ID, &item.Label, &item.Status); e != nil {
			break
		}
		items = append(items, item)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		fail(w, r, e, "")
		return
	}
	page := Pagination{}
	if len(items) > size {
		items = items[:size]
		last := items[len(items)-1]
		cursor.Account, cursor.ID = last.Label, last.ID
		token, e := s.Application.storeCursor(r.Context(), cursor)
		if e != nil {
			fail(w, r, e, "")
			return
		}
		page.NextPageToken = &token
		page.HasMore = true
	}
	if e = tx.Commit(r.Context()); e == nil {
		e = s.Authenticator.Renew(r.Context(), w, r, p)
	}
	if e != nil {
		fail(w, r, e, "")
		return
	}
	respond(w, r, 200, "OK", map[string]any{"items": items}, page)
}
