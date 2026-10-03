package personnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

var ErrQueryChanged = querycontext.ErrChanged
var ErrQueryBusy = errors.New("query validation remained busy")

const queryDeadline = 10 * time.Second

type queryCriteria struct {
	Search       string       `json:"search,omitempty"`
	DepartmentID string       `json:"departmentId,omitempty"`
	IdentityID   string       `json:"identityId,omitempty"`
	Action       string       `json:"action,omitempty"`
	Filter       *FilterGroup `json:"filter,omitempty"`
	Sort         *QuerySort   `json:"sort,omitempty"`
	Range        *QueryRange  `json:"range,omitempty"`
	TimeBounds   []QueryRange `json:"timeBounds,omitempty"`
}

func (c queryCriteria) members(page PageQuery) MemberQueryInput {
	page.Search = c.Search
	return MemberQueryInput{MemberQuery: MemberQuery{PageQuery: page, DepartmentID: c.DepartmentID, IdentityID: c.IdentityID}, QueryOptions: QueryOptions{Filter: c.Filter}}
}
func (c queryCriteria) events(page PageQuery) EventQueryInput {
	page.Search = c.Search
	return EventQueryInput{EventQuery: EventQuery{PageQuery: page, Action: c.Action, From: c.Range.From, To: c.Range.To}, QueryOptions: QueryOptions{Filter: c.Filter}, Sort: c.Sort, FrozenTimeBounds: c.TimeBounds}
}
func normalizedCriteria(view string, c queryCriteria, old *queryCriteria) (queryCriteria, error) {
	if !utf8.ValidString(c.Search) || strings.ContainsRune(c.Search, 0) {
		return c, ErrInvalid
	}
	c.DepartmentID = strings.ToLower(c.DepartmentID)
	c.IdentityID = strings.ToLower(c.IdentityID)
	if view == "members" {
		if c.Range != nil || c.Sort != nil || c.Action != "" {
			return c, ErrInvalid
		}
		if c.DepartmentID != "" && !validID(c.DepartmentID) || c.IdentityID != "" && !validID(c.IdentityID) {
			return c, ErrInvalid
		}
	} else {
		if c.DepartmentID != "" || c.IdentityID != "" || c.Action != "" && !activityActions[c.Action] {
			return c, ErrInvalid
		}
		if c.Sort != nil && (c.Sort.Key != "occurredAt" || (c.Sort.Direction != "asc" && c.Sort.Direction != "desc")) {
			return c, ErrInvalid
		}
		r := QueryRange{}
		if c.Range != nil {
			r = *c.Range
		}
		if old != nil && old.Range != nil {
			if r.From.IsZero() {
				r.From = old.Range.From
			}
			if r.To.IsZero() {
				r.To = old.Range.To
			}
		}
		if r.To.IsZero() {
			r.To = time.Now().UTC().Truncate(time.Microsecond)
		}
		if r.From.IsZero() {
			r.From = r.To.AddDate(0, 0, -7)
		}
		if !r.From.Before(r.To) || r.From.Nanosecond()%1000 != 0 || r.To.Nanosecond()%1000 != 0 {
			return c, ErrInvalid
		}
		r.From = r.From.UTC()
		r.To = r.To.UTC()
		c.Range = &r
	}
	if c.Filter != nil {
		raw, err := projectionJSON(c.Filter)
		if err != nil {
			return c, ErrInvalid
		}
		plan, err := CompileFilter(view, raw, 1)
		if err != nil {
			return c, err
		}
		c.TimeBounds = plan.FrozenDates
		if old != nil && old.Filter != nil {
			previous, _ := projectionJSON(old.Filter)
			if bytes.Equal(previous, plan.Canonical) {
				frozen, err := compileFrozenFilter(view, plan.Canonical, 1, old.TimeBounds)
				if err != nil {
					return c, err
				}
				c.TimeBounds = frozen.FrozenDates
			}
		}
		var group FilterGroup
		if json.Unmarshal(plan.Canonical, &group) != nil {
			return c, ErrInvalid
		}
		c.Filter = &group
	} else {
		c.TimeBounds = nil
	}
	return c, nil
}
func readQueryRevisions(ctx context.Context, tx pgx.Tx, view string) (QueryRevisions, error) {
	var r QueryRevisions
	err := tx.QueryRow(ctx, `SELECT max(revision) FILTER(WHERE scope='people'),max(revision) FILTER(WHERE scope='configuration'),max(revision) FILTER(WHERE scope='activity') FROM personnel.query_revisions`).Scan(&r.People, &r.Configuration, &r.Activity)
	if view == "members" {
		r.Activity = 0
	}
	return r, err
}

type queryProjection struct {
	members     []MemberProjection
	events      []QueryActivity
	total       int64
	fingerprint string
}

func fullQueryProjection(ctx context.Context, tx pgx.Tx, view string, c queryCriteria, page PageQuery) (queryProjection, error) {
	if view == "members" {
		r, err := scanMemberProjection(ctx, tx, c.members(page))
		return queryProjection{members: r.Items, total: r.Total, fingerprint: r.Fingerprint}, err
	}
	r, err := scanEventProjection(ctx, tx, c.events(page))
	return queryProjection{events: r.Items, total: r.Total, fingerprint: r.Fingerprint}, err
}

type queryReadResult struct {
	Members  []Member
	Events   []QueryActivity
	Total    int64
	Version  string
	Criteria queryCriteria
	Page     PageQuery
}

type queryItems struct {
	members []Member
	events  []QueryActivity
}
type personnelQueryStrategy struct {
	app       *Application
	principal session.Principal
	view      string
}

func (s personnelQueryStrategy) Resource() string { return s.view }
func (s personnelQueryStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	return s.app.read(ctx, s.principal)
}
func (s personnelQueryStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	var old *queryCriteria
	if saved != nil {
		old = &queryCriteria{}
		if !decodeQueryMetadata(saved, old) {
			return nil, nil, ErrQueryContextExpired
		}
	}
	if incoming == nil {
		return saved, saved, nil
	}
	var current queryCriteria
	if !decodeQueryMetadata(incoming, &current) {
		return nil, nil, ErrInvalid
	}
	normalized, err := normalizedCriteria(s.view, current, old)
	if err != nil {
		return nil, nil, err
	}
	canonical, err := projectionJSON(normalized)
	if err != nil {
		return nil, nil, err
	}
	return saved, canonical, nil
}
func (s personnelQueryStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	revision, err := readQueryRevisions(ctx, tx, s.view)
	if err != nil {
		return nil, err
	}
	return json.Marshal(revision)
}
func (s personnelQueryStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[queryItems], error) {
	var c queryCriteria
	if !decodeQueryMetadata(raw, &c) {
		return querycontext.Observation[queryItems]{}, ErrQueryContextExpired
	}
	projected, err := fullQueryProjection(ctx, tx, s.view, c, PageQuery{Page: page.Number, PageSize: page.Size})
	if err != nil {
		return querycontext.Observation[queryItems]{}, err
	}
	items := queryItems{events: projected.events}
	if s.view == "members" {
		items.members, err = hydrateQueryMembers(ctx, tx, projected.members)
		if err != nil {
			return querycontext.Observation[queryItems]{}, err
		}
	}
	return querycontext.Observation[queryItems]{Items: items, Total: projected.total, Fingerprint: projected.fingerprint}, nil
}
func (s personnelQueryStrategy) Page(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (queryItems, error) {
	var c queryCriteria
	if !decodeQueryMetadata(raw, &c) {
		return queryItems{}, ErrQueryContextExpired
	}
	p := PageQuery{Page: page.Number, PageSize: page.Size}
	if s.view == "members" {
		projected, err := scanMemberPage(ctx, tx, c.members(p))
		if err != nil {
			return queryItems{}, err
		}
		members, err := hydrateQueryMembers(ctx, tx, projected)
		return queryItems{members: members}, err
	}
	events, err := scanEventPage(ctx, tx, c.events(p))
	return queryItems{events: events}, err
}
func (a *Application) executeQuery(ctx context.Context, p session.Principal, view string, c queryCriteria, page PageQuery, version string) (queryReadResult, error) {
	result := queryReadResult{}
	normalized, err := normalizedPage(page)
	if err != nil {
		return result, err
	}
	if a == nil || a.Queries == nil {
		return result, session.ErrUnavailable
	}
	incoming, err := projectionJSON(c)
	if err != nil {
		return result, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, queryDeadline)
	defer cancel()
	executed, err := querycontext.Execute(ctx, a.Queries.shared, p.SessionRef, version, incoming,
		querycontext.Page{Number: normalized.Page, Size: normalized.PageSize},
		personnelQueryStrategy{app: a, principal: p, view: view})
	if err != nil {
		return result, personnelContextError(err)
	}
	if !decodeQueryMetadata(executed.Criteria, &result.Criteria) {
		return queryReadResult{}, ErrInvalid
	}
	result.Members, result.Events = executed.Items.members, executed.Items.events
	result.Total, result.Version, result.Page = executed.Total, executed.Version, normalized
	return result, nil
}
func (a *Application) SearchMembers(ctx context.Context, p session.Principal, in MemberQueryInput) (MembersQueryPage, error) {
	r, err := a.executeQuery(ctx, p, "members", queryCriteria{Search: in.Search, DepartmentID: in.DepartmentID, IdentityID: in.IdentityID, Filter: in.Filter}, in.PageQuery, in.QueryVersion)
	return MembersQueryPage{Page: Page[Member]{Items: r.Members, Total: r.Total, Page: r.Page.Page, PageSize: r.Page.PageSize}, QueryVersion: r.Version, Sort: nil}, err
}
func (a *Application) SearchEvents(ctx context.Context, p session.Principal, in EventQueryInput) (EventsQueryPage, error) {
	r, err := a.executeQuery(ctx, p, "events", queryCriteria{Search: in.Search, Action: in.Action, Filter: in.Filter, Sort: in.Sort, Range: &QueryRange{From: in.From, To: in.To}}, in.PageQuery, in.QueryVersion)
	out := EventsQueryPage{QueryPage: QueryPage[QueryActivity]{Page: Page[QueryActivity]{Items: r.Events, Total: r.Total, Page: r.Page.Page, PageSize: r.Page.PageSize}, QueryVersion: r.Version, Sort: r.Criteria.Sort}}
	if r.Criteria.Range != nil {
		out.Range = *r.Criteria.Range
	}
	return out, err
}
