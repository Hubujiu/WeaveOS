package personnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrQueryChanged = errors.New("complete query projection changed")
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
func validateSavedQuery(ctx context.Context, tx pgx.Tx, saved QueryContext, r QueryRevisions, c queryCriteria, page PageQuery) (*queryProjection, error) {
	if saved.Revisions == r {
		return nil, nil
	}
	if saved.Revisions.People > r.People || saved.Revisions.Configuration > r.Configuration || saved.Revisions.Activity > r.Activity {
		return nil, ErrQueryContextExpired
	}
	projected, err := fullQueryProjection(ctx, tx, saved.View, c, page)
	if err != nil {
		return nil, err
	}
	if projected.fingerprint != saved.Fingerprint || projected.total != saved.Total {
		return nil, ErrQueryChanged
	}
	return &projected, nil
}

type queryReadResult struct {
	Members  []Member
	Events   []QueryActivity
	Total    int64
	Version  string
	Criteria queryCriteria
	Page     PageQuery
}

func (a *Application) executeQuery(ctx context.Context, p session.Principal, view string, c queryCriteria, page PageQuery, version string) (queryReadResult, error) {
	result := queryReadResult{}
	var err error
	page, err = normalizedPage(page)
	if err != nil {
		return result, err
	}
	if a == nil || a.Queries == nil {
		return result, session.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, queryDeadline)
	defer cancel()
	// Load immutable metadata before opening RR so its verified revision cannot
	// originate from a snapshot newer than ours. Authorization still decides every
	// response, including a missing/expired context; errors are deferred until read().
	var saved QueryContext
	var loadErr error
	if version != "" {
		saved, loadErr = a.Queries.Load(ctx, p.SessionRef, version)
	}
	tx, err := a.read(ctx, p)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	if loadErr != nil {
		return result, loadErr
	}
	var old *queryCriteria
	if version != "" {
		if saved.View != view {
			return result, ErrQueryContextExpired
		}
		old = &queryCriteria{}
		if !decodeQueryMetadata(saved.Criteria, old) {
			return result, ErrQueryContextExpired
		}
	}
	c, err = normalizedCriteria(view, c, old)
	if err != nil {
		return result, err
	}
	canonical, err := projectionJSON(c)
	if err != nil {
		return result, err
	}
	r, err := readQueryRevisions(ctx, tx, view)
	if err != nil {
		return result, err
	}
	same := version != "" && bytes.Equal(saved.Criteria, canonical)
	var verified *queryProjection
	if old != nil {
		validationPage := PageQuery{Page: 1, PageSize: 1}
		if same {
			validationPage = page
		}
		verified, err = validateSavedQuery(ctx, tx, saved, r, *old, validationPage)
		if err != nil {
			return result, err
		}
	}
	var projection queryProjection
	if !same {
		projection, err = fullQueryProjection(ctx, tx, view, c, page)
	} else if verified != nil {
		projection = *verified
	} else {
		projection.total = saved.Total
		projection.fingerprint = saved.Fingerprint
		if view == "members" {
			projection.members, err = scanMemberPage(ctx, tx, c.members(page))
		} else {
			projection.events, err = scanEventPage(ctx, tx, c.events(page))
		}
	}
	if err != nil {
		return result, err
	}
	if view == "members" {
		result.Members, err = hydrateQueryMembers(ctx, tx, projection.members)
		if err != nil {
			return result, err
		}
	} else {
		result.Events = projection.events
	}
	// Authorization, old/new projections, page, total and r were read in this one
	// snapshot. Release the read transaction before bounded Redis metadata writes.
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	if old != nil && saved.Revisions != r {
		err = a.Queries.Advance(ctx, p.SessionRef, version, saved.Fingerprint, saved.Revisions, r)
		if err != nil && !errors.Is(err, ErrQueryContextCAS) {
			return result, err
		}
	}
	if !same {
		version, err = a.Queries.Create(ctx, p.SessionRef, QueryContext{View: view, Criteria: canonical, Total: projection.total, Fingerprint: projection.fingerprint, Revisions: r, ProtocolVersion: 1})
		if err != nil {
			return result, err
		}
	}
	result.Total = projection.total
	result.Version = version
	result.Criteria = c
	result.Page = page
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
