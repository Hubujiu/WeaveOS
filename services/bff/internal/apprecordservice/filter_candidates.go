package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type FilterCandidatesRequest struct {
	AppID, ViewID, FieldID, Q, PageToken string
	PageSize                             int
}
type FilterCandidatesResult struct {
	Items         []ReferenceDisplay `json:"items"`
	NextPageToken string             `json:"-"`
	HasMore       bool               `json:"-"`
}
type filterCandidateCursor struct {
	ActorID   string `json:"actorId"`
	AppID     string `json:"appId"`
	TableID   string `json:"tableId"`
	ViewID    string `json:"viewId"`
	FieldID   string `json:"fieldId"`
	Kind      string `json:"kind"`
	Q         string `json:"q"`
	LastLabel string `json:"lastLabel"`
	LastID    string `json:"lastId"`
}

func validFilterCandidateQuery(q string) bool {
	return utf8.ValidString(q) && utf8.RuneCountInString(q) <= 100 && !strings.ContainsRune(q, 0) && strings.TrimSpace(q) == q
}
func filterCandidateFingerprint(b filterCandidateCursor, r revisions) string {
	raw, _ := json.Marshal(struct {
		Binding  filterCandidateCursor
		Revision revisions
	}{b, r})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func newFilterCandidateStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "record-filter-values", generation, querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			var b filterCandidateCursor
			var r revisions
			if !workflowClosedMetadata(m.Criteria, 4096, 9, &b) || !workflowClosedMetadata(m.Revision, 512, 6, &r) || !validRevisions(r) || m.Total != 0 {
				return false
			}
			for _, id := range []string{b.ActorID, b.AppID, b.TableID, b.ViewID, b.FieldID, b.LastID} {
				if !appfields.ValidID(id) {
					return false
				}
			}
			if b.Kind != "member" && b.Kind != "department" || !validFilterCandidateQuery(b.Q) || !utf8.ValidString(b.LastLabel) || len(b.LastLabel) > 1016 || strings.ContainsRune(b.LastLabel, 0) {
				return false
			}
			canonical, _ := json.Marshal(b)
			revision, _ := json.Marshal(r)
			return bytes.Equal(canonical, m.Criteria) && bytes.Equal(revision, m.Revision) && m.View == "record-filter-values:"+b.AppID+":"+b.ViewID+":"+b.FieldID && m.Fingerprint == filterCandidateFingerprint(b, r)
		},
		Forward: func(string, json.RawMessage, json.RawMessage) bool { return false },
	})
}

// FilterCandidates reuses the complete record read policy and lifecycle predicate.
// It never enumerates the organisation directory or opens ordinary write authority.
func (s *Service) FilterCandidates(ctx context.Context, p session.Principal, req FilterCandidatesRequest) (FilterCandidatesResult, error) {
	empty := FilterCandidatesResult{}
	if s == nil || s.Pool == nil || s.FilterValues == nil {
		return empty, ErrUnavailable
	}
	req.Q = strings.TrimSpace(req.Q)
	if !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || !appfields.ValidID(req.FieldID) || !validFilterCandidateQuery(req.Q) || req.PageSize < 1 || req.PageSize > 50 {
		return empty, applications.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	strategy := &recordStrategy{service: s, principal: p, appID: req.AppID, viewID: req.ViewID}
	tx, e := strategy.OpenRead(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(context.Background())
	source, kind := "", ""
	for _, field := range strategy.fields {
		if field.ID == req.FieldID {
			source = referenceRelation(field.Kind)
			kind = string(field.Kind)
			break
		}
	}
	if source == "" {
		return empty, applications.ErrInvalid
	}
	if !strategy.policy.CoversRead([]string{req.FieldID}) {
		return empty, applications.ErrDenied
	}
	binding := filterCandidateCursor{ActorID: p.UserID, AppID: req.AppID, TableID: strategy.tableID, ViewID: req.ViewID, FieldID: req.FieldID, Kind: kind, Q: req.Q}
	if req.PageToken != "" {
		saved, err := s.FilterValues.Load(ctx, p.SessionRef, req.PageToken)
		if err != nil {
			return empty, err
		}
		var old filterCandidateCursor
		var rev revisions
		if json.Unmarshal(saved.Criteria, &old) != nil || json.Unmarshal(saved.Revision, &rev) != nil {
			return empty, querycontext.ErrInvalid
		}
		scoped := old
		scoped.LastLabel = ""
		scoped.LastID = ""
		if scoped != binding {
			return empty, querycontext.ErrExpired
		}
		if rev != strategy.control {
			return empty, querycontext.ErrChanged
		}
		binding = old
	}
	_, predicate, args, e := strategy.sql(criteria{Filter: json.RawMessage("null"), Sort: json.RawMessage("null")})
	if e != nil {
		return empty, e
	}
	rel := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(strategy.tableID, "-", "")}.Sanitize()
	column := referenceColumn(req.FieldID)
	// Check every readable non-null value before prefix/keyset filtering. An inner
	// join must not silently conceal broken source integrity.
	var missing bool
	e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+rel+" r LEFT JOIN "+source+" src ON src.id="+column+" WHERE "+predicate+" AND "+column+" IS NOT NULL AND src.id IS NULL)", args...).Scan(&missing)
	if e != nil {
		return empty, e
	}
	if missing {
		return empty, ErrUnavailable
	}
	query := "SELECT DISTINCT src.id::text,src.label,src.status='deleted' FROM " + rel + " r JOIN " + source + " src ON src.id=" + column + " WHERE " + predicate
	if req.Q != "" {
		args = append(args, req.Q)
		query += fmt.Sprintf(" AND starts_with(src.label,$%d::text)", len(args))
	}
	if binding.LastID != "" {
		args = append(args, binding.LastLabel, binding.LastID)
		query += fmt.Sprintf(" AND (src.label,src.id::text)>($%d::text,$%d::text)", len(args)-1, len(args))
	}
	args = append(args, req.PageSize+1)
	query += fmt.Sprintf(" ORDER BY src.label,src.id::text LIMIT $%d", len(args))
	rows, e := tx.Query(ctx, query, args...)
	if e != nil {
		return empty, e
	}
	items := make([]ReferenceDisplay, 0, req.PageSize+1)
	for rows.Next() {
		var item ReferenceDisplay
		if e = rows.Scan(&item.ID, &item.Label, &item.Deleted); e != nil {
			break
		}
		items = append(items, item)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return empty, e
	}
	out := FilterCandidatesResult{Items: items, HasMore: len(items) > req.PageSize}
	if out.HasMore {
		out.Items = items[:req.PageSize]
		last := out.Items[len(out.Items)-1]
		binding.LastID = last.ID
		binding.LastLabel = last.Label
		raw, _ := json.Marshal(binding)
		revision, _ := json.Marshal(strategy.control)
		out.NextPageToken, e = s.FilterValues.Create(ctx, p.SessionRef, querycontext.Metadata{View: "record-filter-values:" + binding.AppID + ":" + binding.ViewID + ":" + binding.FieldID, Criteria: raw, Revision: revision, Total: 0, ProtocolVersion: 1, Fingerprint: filterCandidateFingerprint(binding, strategy.control)})
		if e != nil {
			return empty, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return empty, e
	}
	return out, nil
}
