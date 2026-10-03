package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

type criteria struct {
	Filter json.RawMessage `json:"filter"`
	Sort   json.RawMessage `json:"sort"`
	Quick  json.RawMessage `json:"quickSearch,omitempty"`
}
type revisions struct {
	Data       int64 `json:"data"`
	Dependency int64 `json:"dependency"`
	Schema     int64 `json:"schema"`
	View       int64 `json:"view"`
	Policy     int64 `json:"policy"`
	Source     int64 `json:"source"`
}
type recordStrategy struct {
	service                *Service
	principal              session.Principal
	appID, viewID, tableID string
	ownerID                string
	policy                 appaccess.Policy
	fields                 []appquery.Field
	control                revisions
}

func referenceRelation(kind appquery.FieldKind) string {
	switch kind {
	case appquery.Member:
		return "applications.member_sources"
	case appquery.Department:
		return "applications.department_sources"
	}
	return ""
}
func referenceColumn(id string) string {
	return "r." + pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
}

func (s *Service) Search(ctx context.Context, principal session.Principal, req SearchRequest) (SearchResult, error) {
	if s == nil || s.Pool == nil || s.Queries == nil || !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || principal.SessionRef == "" {
		return SearchResult{}, ErrUnavailable
	}
	if _, _, err := appquery.PageWindow(req.Page, req.PageSize); err != nil {
		return SearchResult{}, err
	}
	incoming, err := json.Marshal(criteria{Filter: req.Filter, Sort: req.Sort, Quick: req.QuickSearch})
	if err != nil {
		return SearchResult{}, err
	}
	strategy := &recordStrategy{service: s, principal: principal, appID: req.AppID, viewID: req.ViewID}
	result, err := querycontext.Execute(ctx, s.Queries, principal.SessionRef, req.QueryVersion, incoming, querycontext.Page{Number: int(req.Page), Size: int(req.PageSize)}, strategy)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Items: result.Items, Total: result.Total, QueryVersion: result.Version,
		SchemaVersion: strategy.control.Schema, ViewVersion: strategy.control.View, PolicyRevision: strategy.control.Policy}, nil
}

func (s *recordStrategy) Resource() string {
	return "apprecords:" + s.appID + ":" + s.tableID + ":" + s.viewID
}
func (s *recordStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.service.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	fail := func(e error) (pgx.Tx, error) { _ = tx.Rollback(context.Background()); return nil, e }
	var bootstrap bool
	err = tx.QueryRow(ctx, "SELECT is_bootstrap_admin FROM auth.users WHERE id=$1 AND status='active' AND auth_version::text=$2", s.principal.UserID, s.principal.Record.AuthVersion).Scan(&bootstrap)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(session.ErrUnauthorized)
	}
	if err != nil {
		return fail(err)
	}
	var ready bool
	err = tx.QueryRow(ctx, `SELECT a.owner_user_id::text,a.policy_revision,t.id::text,t.schema_version,t.dependency_revision,t.data_revision,
 t.schema_ready,v.view_version FROM applications.apps a
 JOIN applications.form_views v ON v.app_id=a.id
 JOIN applications.logical_tables t ON t.app_id=v.app_id AND t.id=v.table_id
 JOIN applications.menu_resources m ON m.app_id=v.app_id AND m.resource_kind='form' AND m.resource_id=v.id
 WHERE a.id=$1 AND v.id=$2`, s.appID, s.viewID).Scan(&s.ownerID, &s.control.Policy, &s.tableID, &s.control.Schema, &s.control.Dependency, &s.control.Data, &ready, &s.control.View)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(applications.ErrMissing)
	}
	if err != nil {
		return fail(err)
	}
	if !ready {
		return fail(ErrUnavailable)
	}
	var grants []appaccess.Grant
	var menu bool
	if !bootstrap && s.principal.UserID != s.ownerID {
		rows, e := tx.Query(ctx, `SELECT g.action,g.row_scope,
 ARRAY(SELECT field_id::text FROM applications.grant_fields gf WHERE gf.app_id=g.app_id AND gf.grant_id=g.id ORDER BY field_id)
 FROM applications.grants g JOIN applications.permission_groups p ON p.app_id=g.app_id AND p.id=g.group_id AND p.enabled
 JOIN applications.group_members gm ON gm.app_id=p.app_id AND gm.group_id=p.id AND gm.user_id=$2
 WHERE g.app_id=$1 AND g.resource_kind='form' AND g.resource_id=$3 ORDER BY g.id`, s.appID, s.principal.UserID, s.viewID)
		if e != nil {
			return fail(e)
		}
		for rows.Next() {
			var action, scope string
			var ids []string
			if e = rows.Scan(&action, &scope, &ids); e != nil {
				break
			}
			if action == "menu.enter" {
				menu = true
				continue
			}
			var sc appaccess.Scope
			switch scope {
			case "all":
				sc = appaccess.All
			case "own":
				sc = appaccess.Own
			default:
				e = ErrUnavailable
			}
			grants = append(grants, appaccess.Grant{AppID: s.appID, ViewID: s.viewID, Action: appaccess.Action(action), Scope: sc, Fields: ids})
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return fail(e)
		}
		if !menu {
			return fail(applications.ErrDenied)
		}
	}
	s.policy = appaccess.Policy{ActorID: s.principal.UserID, AppID: s.appID, OwnerID: s.ownerID, ViewID: s.viewID, ResourceExists: true, BootstrapAdmin: bootstrap, Grants: grants}
	if s.policy.VisibleScope() == appaccess.None {
		return fail(applications.ErrDenied)
	}
	rows, e := tx.Query(ctx, "SELECT definition FROM applications.fields WHERE app_id=$1 AND table_id=$2 AND NOT removed ORDER BY id", s.appID, s.tableID)
	if e != nil {
		return fail(e)
	}
	for rows.Next() {
		var raw []byte
		var field appfields.Field
		if e = rows.Scan(&raw); e != nil {
			break
		}
		if e = json.Unmarshal(raw, &field); e != nil {
			break
		}
		s.fields = append(s.fields, appquery.Field{ID: field.ID, Kind: appquery.FieldKind(field.Kind)})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return fail(e)
	}
	if e = tx.QueryRow(ctx, "SELECT revision FROM applications.reference_source_revision WHERE singleton").Scan(&s.control.Source); errors.Is(e, pgx.ErrNoRows) {
		return fail(ErrUnavailable)
	}
	if e != nil {
		return fail(e)
	}
	return tx, nil
}

func (s *recordStrategy) canonical(raw json.RawMessage, saved bool) (json.RawMessage, error) {
	var c criteria
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, appquery.ErrInvalid
	}
	plan, err := appquery.CompileSearch(s.policy, s.fields, c.Filter, c.Sort, 1)
	if err != nil {
		if saved && errors.Is(err, appquery.ErrInvalid) {
			return nil, querycontext.ErrChanged
		}
		return nil, err
	}
	c.Filter = plan.Filter.Canonical
	if len(c.Filter) == 0 {
		c.Filter = json.RawMessage("null")
	}
	if len(c.Sort) == 0 {
		c.Sort = json.RawMessage("null")
	} else {
		var v any
		if json.Unmarshal(c.Sort, &v) != nil {
			return nil, appquery.ErrInvalid
		}
		c.Sort, _ = json.Marshal(v)
	}
	if len(c.Quick) > 0 {
		quick, e := appquery.CompileQuickSearch(c.Quick, s.fields, s.policy, 1)
		if e != nil {
			return nil, e
		}
		c.Quick = quick.Canonical
	}
	return json.Marshal(c)
}
func (s *recordStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	var old, current json.RawMessage
	var err error
	if len(saved) > 0 {
		old, err = s.canonical(saved, true)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(incoming) == 0 {
		return old, old, nil
	}
	current, err = s.canonical(incoming, false)
	return old, current, err
}
func (s *recordStrategy) Revisions(_ context.Context, _ pgx.Tx) (json.RawMessage, error) {
	return json.Marshal(s.control)
}

func (s *recordStrategy) sql(c criteria) (appquery.SearchSQL, string, []any, error) {
	plan, err := appquery.CompileSearch(s.policy, s.fields, c.Filter, c.Sort, 1)
	if err != nil {
		return plan, "", nil, err
	}
	args := append([]any{}, plan.Arguments...)
	predicate := plan.Access.RowPredicate + " AND " + plan.Filter.Predicate
	if len(c.Quick) > 0 {
		q, e := appquery.CompileQuickSearch(c.Quick, s.fields, s.policy, len(args)+1)
		if e != nil {
			return plan, "", nil, e
		}
		predicate += " AND " + q.Predicate
		args = append(args, q.Arguments...)
	}
	for _, id := range plan.Filter.ReferencedFields {
		for _, field := range s.fields {
			if field.ID == id && referenceRelation(field.Kind) != "" {
				predicate += " AND EXISTS(SELECT 1 FROM " + referenceRelation(field.Kind) + " source WHERE source.id=" + referenceColumn(id) + ")"
			}
		}
	}
	return plan, predicate, args, nil
}
func (s *recordStrategy) selectSQL(raw json.RawMessage, full bool) (string, string, []any, error) {
	var c criteria
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", "", nil, appquery.ErrInvalid
	}
	plan, predicate, args, err := s.sql(c)
	if err != nil {
		return "", "", nil, err
	}
	if !appfields.ValidID(s.tableID) {
		return "", "", nil, ErrUnavailable
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.tableID, "-", "")}.Sanitize()
	joins := ""
	refExpression := "'{}'::jsonb"
	if full {
		for n, field := range s.fields {
			source := referenceRelation(field.Kind)
			if source == "" || s.policy.FieldScope(appaccess.Read, field.ID) == appaccess.None {
				continue
			}
			alias := fmt.Sprintf("src%d", n)
			column := referenceColumn(field.ID)
			joins += " LEFT JOIN " + source + " " + alias + " ON " + alias + ".id=" + column
			cell := fmt.Sprintf("CASE WHEN %s IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('%s',jsonb_build_object(%s::text,jsonb_build_object('id',%s.id,'label',%s.label,'deleted',%s.status='deleted'))) END", column, field.ID, column, alias, alias, alias)
			if s.policy.FieldScope(appaccess.Read, field.ID) == appaccess.Own {
				cell = fmt.Sprintf("CASE WHEN r.created_by=$1::uuid THEN %s ELSE '{}'::jsonb END", cell)
			}
			refExpression += " || " + cell
		}
	}
	projection := fmt.Sprintf(`jsonb_build_object('id',r.id,'createdBy',r.created_by,'createdAt',r.created_at,
 'updatedAt',r.updated_at,'recordVersion',r.record_version,'values',%s,'referenceDisplays',%s)`, plan.Access.Projection, refExpression)
	return "SELECT " + projection + " FROM " + relation + " r" + joins + " WHERE " + predicate, plan.Filter.Order, args, nil
}
func (s *recordStrategy) Page(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]Record, error) {
	base, order, args, err := s.selectSQL(raw, false)
	if err != nil {
		return nil, err
	}
	limit, offset, err := appquery.PageWindow(int64(page.Number), int64(page.Size))
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("%s ORDER BY %s LIMIT $%d OFFSET $%d", base, order, len(args)+1, len(args)+2)
	args = append(args, limit, offset)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var raw []byte
		var item Record
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err = s.hydrateReferences(ctx, tx, items); err != nil {
		return nil, err
	}
	return items, nil
}
func (s *recordStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]Record], error) {
	base, order, args, err := s.selectSQL(raw, true)
	if err != nil {
		return querycontext.Observation[[]Record]{}, err
	}
	if err = s.validateReferences(ctx, tx, raw); err != nil {
		return querycontext.Observation[[]Record]{}, err
	}
	rows, err := tx.Query(ctx, base+" ORDER BY "+order, args...)
	if err != nil {
		return querycontext.Observation[[]Record]{}, err
	}
	projection, err := appquery.FingerprintRows(ctx, rows)
	if err != nil {
		return querycontext.Observation[[]Record]{}, err
	}
	items, err := s.Page(ctx, tx, raw, page)
	if err != nil {
		return querycontext.Observation[[]Record]{}, err
	}
	return querycontext.Observation[[]Record]{Items: items, Total: projection.Total, Fingerprint: projection.Fingerprint}, nil
}

func (s *recordStrategy) validateReferences(ctx context.Context, tx pgx.Tx, raw json.RawMessage) error {
	var c criteria
	if err := json.Unmarshal(raw, &c); err != nil {
		return appquery.ErrInvalid
	}
	plan, predicate, args, err := s.sql(c)
	if err != nil {
		return err
	}
	if len(plan.Access.Arguments) > 0 {
		predicate += " AND $1::uuid IS NOT NULL"
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.tableID, "-", "")}.Sanitize()
	for _, field := range s.fields {
		source := referenceRelation(field.Kind)
		if source == "" || s.policy.FieldScope(appaccess.Read, field.ID) == appaccess.None {
			continue
		}
		column := referenceColumn(field.ID)
		mask := ""
		if s.policy.FieldScope(appaccess.Read, field.ID) == appaccess.Own {
			mask = " AND r.created_by=$1::uuid"
		}
		query := "SELECT EXISTS(SELECT 1 FROM " + relation + " r LEFT JOIN " + source + " ref ON ref.id=" + column + " WHERE " + predicate + mask + " AND " + column + " IS NOT NULL AND ref.id IS NULL)"
		var missing bool
		if err := tx.QueryRow(ctx, query, args...).Scan(&missing); err != nil {
			return err
		}
		if missing {
			return ErrUnavailable
		}
	}
	return nil
}

// The authorized page is fetched first; one query per reference kind then
// resolves all distinct visible IDs within the same RR snapshot.
func (s *recordStrategy) hydrateReferences(ctx context.Context, tx pgx.Tx, items []Record) error {
	for _, kind := range []appquery.FieldKind{appquery.Member, appquery.Department} {
		ids := map[string]bool{}
		for _, item := range items {
			for _, field := range s.fields {
				if field.Kind != kind {
					continue
				}
				if id, ok := item.Values[field.ID].(string); ok && id != "" {
					ids[id] = true
				}
			}
		}
		if len(ids) == 0 {
			continue
		}
		list := make([]string, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		rows, err := tx.Query(ctx, "SELECT id::text,label,status FROM "+referenceRelation(kind)+" WHERE id=ANY($1::uuid[])", list)
		if err != nil {
			return err
		}
		displays := map[string]ReferenceDisplay{}
		for rows.Next() {
			var id, label, status string
			if err = rows.Scan(&id, &label, &status); err != nil {
				break
			}
			displays[id] = ReferenceDisplay{ID: id, Label: label, Deleted: status == "deleted"}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if len(displays) != len(ids) {
			return ErrUnavailable
		}
		for i := range items {
			for _, field := range s.fields {
				if field.Kind != kind {
					continue
				}
				id, ok := items[i].Values[field.ID].(string)
				if !ok || id == "" {
					continue
				}
				if items[i].ReferenceDisplays == nil {
					items[i].ReferenceDisplays = map[string]map[string]ReferenceDisplay{}
				}
				items[i].ReferenceDisplays[field.ID] = map[string]ReferenceDisplay{id: displays[id]}
			}
		}
	}
	return nil
}
