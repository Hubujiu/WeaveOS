package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type DeletedRecordSearchRequest struct {
	AppID, ViewID, QueryVersion string
	Page, PageSize              int
}
type DeletedRecordItem struct {
	ID            string `json:"id"`
	RecordVersion int64  `json:"recordVersion"`
	DeletedAt     string `json:"deletedAt"`
}
type DeletedRecordSearchResult struct {
	Items         []DeletedRecordItem `json:"items"`
	Total         int64               `json:"total"`
	Page          int                 `json:"page"`
	PageSize      int                 `json:"pageSize"`
	QueryVersion  string              `json:"queryVersion"`
	SchemaVersion int64               `json:"schemaVersion"`
}
type deletedRecordBinding struct {
	ActorID string `json:"actorId"`
	AppID   string `json:"appId"`
	ViewID  string `json:"viewId"`
}
type deletedRecordRevision struct {
	Fingerprint string `json:"fingerprint"`
}

func validDeletedBinding(raw json.RawMessage) (deletedRecordBinding, bool) {
	var b deletedRecordBinding
	ok := workflowClosedMetadata(raw, 512, 3, &b) && appfields.ValidID(b.ActorID) && appfields.ValidID(b.AppID) && appfields.ValidID(b.ViewID)
	return b, ok
}
func validDeletedRevision(raw json.RawMessage) (deletedRecordRevision, bool) {
	var r deletedRecordRevision
	ok := workflowClosedMetadata(raw, 128, 1, &r) && workflowReadFingerprintValid(r.Fingerprint)
	return r, ok
}
func validDeletedResource(resource string) bool {
	parts := strings.Split(resource, ":")
	return len(parts) == 4 && parts[0] == "record-deleted" && appfields.ValidID(parts[1]) && appfields.ValidID(parts[2]) && appfields.ValidID(parts[3])
}
func newDeletedRecordStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "record-deleted", generation, querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			b, ok := validDeletedBinding(m.Criteria)
			r, valid := validDeletedRevision(m.Revision)
			return ok && valid && validDeletedResource(m.View) && strings.HasPrefix(m.View, "record-deleted:"+b.AppID+":"+b.ViewID+":") && m.Fingerprint == r.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			_, a := validDeletedRevision(old)
			_, b := validDeletedRevision(next)
			return a && b && validDeletedResource(resource)
		},
	})
}
func (s *Service) SearchDeletedRecords(ctx context.Context, p session.Principal, q DeletedRecordSearchRequest) (DeletedRecordSearchResult, error) {
	empty := DeletedRecordSearchResult{}
	if s == nil || s.Pool == nil || s.DeletedRecords == nil {
		return empty, ErrUnavailable
	}
	if !appfields.ValidID(q.AppID) || !appfields.ValidID(q.ViewID) || !utf8.ValidString(q.QueryVersion) || utf8.RuneCountInString(q.QueryVersion) > 256 {
		return empty, applications.ErrInvalid
	}
	if _, _, e := appquery.PageWindow(int64(q.Page), int64(q.PageSize)); e != nil {
		return empty, e
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b := deletedRecordBinding{ActorID: p.UserID, AppID: q.AppID, ViewID: q.ViewID}
	raw, _ := json.Marshal(b)
	strategy := &deletedRecordStrategy{service: s, principal: p, binding: b, page: querycontext.Page{Number: q.Page, Size: q.PageSize}}
	r, e := querycontext.Execute(ctx, s.DeletedRecords, p.SessionRef, q.QueryVersion, raw, strategy.page, strategy)
	if e != nil {
		return empty, e
	}
	return DeletedRecordSearchResult{Items: r.Items, Total: r.Total, Page: q.Page, PageSize: q.PageSize, QueryVersion: r.Version, SchemaVersion: strategy.facts.SchemaVersion}, nil
}

type deletedRecordStrategy struct {
	service     *Service
	principal   session.Principal
	binding     deletedRecordBinding
	page        querycontext.Page
	facts       applications.RecordContext
	observation querycontext.Observation[[]DeletedRecordItem]
}

func (s *deletedRecordStrategy) Resource() string {
	return "record-deleted:" + s.binding.AppID + ":" + s.binding.ViewID + ":" + s.facts.TableID
}
func (s *deletedRecordStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	tx, f, e := (&applications.Application{Pool: s.service.Pool}).BeginRecordRead(ctx, s.principal, s.binding.AppID, s.binding.ViewID)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (pgx.Tx, error) { _ = tx.Rollback(context.Background()); return nil, e }
	if e = lifecycleManagerAuthorization(ctx, tx, f); e != nil {
		return fail(e)
	}
	if !f.SchemaReady {
		return fail(apprecords.ErrNotReady)
	}
	if !appfields.ValidID(f.TableID) || f.SchemaVersion < 1 || f.SchemaVersion > workflowMaxCounter {
		return fail(ErrUnavailable)
	}
	s.facts = f
	return tx, nil
}
func (s *deletedRecordStrategy) validCriteria(raw json.RawMessage) bool {
	b, ok := validDeletedBinding(raw)
	return ok && b == s.binding
}
func (s *deletedRecordStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	if len(saved) > 0 && !s.validCriteria(saved) {
		return nil, nil, querycontext.ErrExpired
	}
	if !s.validCriteria(incoming) {
		return nil, nil, querycontext.ErrInvalid
	}
	return saved, incoming, nil
}
func (s *deletedRecordStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	_, offset, e := appquery.PageWindow(int64(s.page.Number), int64(s.page.Size))
	if e != nil {
		return nil, e
	}
	table := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.facts.TableID, "-", "")}.Sanitize()
	rows, e := tx.Query(ctx, `SELECT r.id::text,r.record_version,l.changed_at FROM applications.record_lifecycle l
 JOIN `+table+` r ON r.id=l.record_id WHERE l.app_id=$1 AND l.table_id=$2 AND l.deleted
 ORDER BY l.changed_at DESC,r.id DESC`, s.binding.AppID, s.facts.TableID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/record-deleted/v1\x00"))
	scope, _ := json.Marshal(struct {
		TableID       string
		SchemaVersion int64
	}{s.facts.TableID, s.facts.SchemaVersion})
	_, _ = h.Write(scope)
	_, _ = h.Write([]byte{0})
	s.observation = querycontext.Observation[[]DeletedRecordItem]{Items: make([]DeletedRecordItem, 0, s.page.Size)}
	for rows.Next() {
		var item DeletedRecordItem
		var changed time.Time
		if e = rows.Scan(&item.ID, &item.RecordVersion, &changed); e != nil {
			return nil, e
		}
		if !appfields.ValidID(item.ID) || item.RecordVersion < 1 || item.RecordVersion > workflowMaxCounter || s.observation.Total >= workflowMaxCounter {
			return nil, ErrUnavailable
		}
		item.DeletedAt = changed.UTC().Format(time.RFC3339Nano)
		raw, e := json.Marshal(item)
		if e != nil {
			return nil, e
		}
		_, _ = h.Write(raw)
		_, _ = h.Write([]byte{0})
		n := s.observation.Total
		s.observation.Total++
		if n >= offset && len(s.observation.Items) < s.page.Size {
			s.observation.Items = append(s.observation.Items, item)
		}
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	s.observation.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return json.Marshal(deletedRecordRevision{Fingerprint: s.observation.Fingerprint})
}
func (s *deletedRecordStrategy) Page(_ context.Context, _ pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]DeletedRecordItem, error) {
	if !s.validCriteria(raw) || page != s.page {
		return nil, querycontext.ErrInvalid
	}
	return s.observation.Items, nil
}
func (s *deletedRecordStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]DeletedRecordItem], error) {
	_, e := s.Page(ctx, tx, raw, page)
	return s.observation, e
}
