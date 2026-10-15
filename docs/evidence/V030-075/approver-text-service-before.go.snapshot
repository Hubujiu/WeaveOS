package appworkflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type approverItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
}
type approverResult struct {
	Items          []approverItem `json:"items"`
	Total          int64          `json:"total"`
	Page           int            `json:"page"`
	PageSize       int            `json:"pageSize"`
	QueryVersion   string         `json:"queryVersion"`
	SchemaVersion  int64          `json:"schemaVersion"`
	PolicyRevision int64          `json:"policyRevision"`
}

func approverResource(b managerBinding) string {
	return "workflow-approvers:" + b.AppID + ":" + b.ViewID
}
func NewApproverQueryStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-approvers", generation, querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			b, ok := managerValidBinding(m.Criteria)
			r, valid := managerValidRevision(m.Revision)
			return ok && valid && m.View == approverResource(b) && m.Fingerprint == r.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			_, a := managerValidRevision(old)
			_, b := managerValidRevision(next)
			p := strings.Split(resource, ":")
			return a && b && len(p) == 3 && p[0] == "workflow-approvers" && validID(p[1]) && validID(p[2])
		},
	})
}

type approverStrategy struct {
	app            *Application
	principal      session.Principal
	binding        managerBinding
	page           querycontext.Page
	schema, policy int64
	owner          string
	fields         []string
	observation    querycontext.Observation[[]approverItem]
}

func (s *approverStrategy) Resource() string { return approverResource(s.binding) }
func (s *approverStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	return (&managerStrategy{app: s.app, principal: s.principal, binding: s.binding}).OpenRead(ctx)
}
func (s *approverStrategy) validCriteria(raw json.RawMessage) bool {
	b, ok := managerValidBinding(raw)
	return ok && b == s.binding
}
func (s *approverStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	if len(saved) > 0 && !s.validCriteria(saved) {
		return nil, nil, querycontext.ErrExpired
	}
	if !s.validCriteria(incoming) {
		return nil, nil, querycontext.ErrInvalid
	}
	return saved, incoming, nil
}
func (s *approverStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	_, offset, e := appquery.PageWindow(int64(s.page.Number), int64(s.page.Size))
	if e != nil {
		return nil, e
	}
	var rawFields []byte
	e = tx.QueryRow(ctx, `SELECT a.owner_user_id::text,a.policy_revision,t.schema_version,t.fields_json FROM applications.apps a
 JOIN applications.form_views v ON v.app_id=a.id AND v.id=$2 JOIN applications.logical_tables t ON t.app_id=v.app_id AND t.id=v.table_id
 WHERE a.id=$1 AND a.deleted_at IS NULL AND v.deleted_at IS NULL AND t.deleted_at IS NULL`, s.binding.AppID, s.binding.ViewID).Scan(&s.owner, &s.policy, &s.schema, &rawFields)
	if e != nil {
		return nil, e
	}
	var fields []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(rawFields, &fields) != nil || s.policy < 1 || s.schema < 1 {
		return nil, session.ErrUnavailable
	}
	s.fields = make([]string, 0, len(fields))
	for _, f := range fields {
		if !validID(f.ID) {
			return nil, session.ErrUnavailable
		}
		s.fields = append(s.fields, f.ID)
	}
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/workflow-approvers/v1\x00"))
	versions, _ := json.Marshal([]int64{s.schema, s.policy})
	_, _ = h.Write(versions)
	s.observation = querycontext.Observation[[]approverItem]{Items: make([]approverItem, 0, s.page.Size)}
	var beforeAccount, beforeID string
	type candidate struct {
		item approverItem
		root bool
	}
	for {
		sql := `SELECT id::text,account,is_bootstrap_admin FROM auth.users WHERE status='active'`
		args := []any{}
		if beforeID != "" {
			sql += " AND (account,id)>($1::text,$2::uuid)"
			args = append(args, beforeAccount, beforeID)
		}
		sql += " ORDER BY account,id LIMIT 64"
		rows, e := tx.Query(ctx, sql, args...)
		if e != nil {
			return nil, e
		}
		batch := make([]candidate, 0, 64)
		for rows.Next() {
			var c candidate
			c.item.Status = "active"
			if e = rows.Scan(&c.item.ID, &c.item.Label, &c.root); e != nil {
				rows.Close()
				return nil, e
			}
			batch = append(batch, c)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		for _, c := range batch {
			eligible, e := s.eligible(ctx, tx, c.item.ID, c.root)
			if e != nil {
				return nil, e
			}
			if !eligible {
				continue
			}
			if s.observation.Total >= maxSafeInteger {
				return nil, session.ErrUnavailable
			}
			raw, e := json.Marshal(c.item)
			if e != nil {
				return nil, e
			}
			_, _ = h.Write(raw)
			_, _ = h.Write([]byte{0})
			n := s.observation.Total
			s.observation.Total++
			if n >= offset && len(s.observation.Items) < s.page.Size {
				s.observation.Items = append(s.observation.Items, c.item)
			}
		}
		if len(batch) < 64 {
			break
		}
		last := batch[len(batch)-1]
		beforeAccount, beforeID = last.item.Label, last.item.ID
	}
	s.observation.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return json.Marshal(managerRevision{Fingerprint: s.observation.Fingerprint})
}

// Keep each grant tuple intact. A candidate's own row is only the existing
// configuration qualification; execution still checks the actual record owner.
func (s *approverStrategy) eligible(ctx context.Context, tx pgx.Tx, id string, root bool) (bool, error) {
	if root || id == s.owner {
		return true, nil
	}
	if len(s.fields) == 0 {
		return false, nil
	}
	rows, e := tx.Query(ctx, `SELECT p.id::text,g.resource_kind,g.resource_id::text,g.action,g.row_scope,
 ARRAY(SELECT gf.field_id::text FROM applications.grant_fields gf WHERE gf.app_id=g.app_id AND gf.grant_id=g.id ORDER BY gf.field_id)
 FROM applications.permission_groups p JOIN applications.group_members m ON m.app_id=p.app_id AND m.group_id=p.id
 JOIN applications.grants g ON g.app_id=p.app_id AND g.group_id=p.id
 WHERE p.app_id=$1 AND m.user_id=$2 AND p.enabled AND g.resource_kind='form' AND g.resource_id=$3 ORDER BY p.id,g.id`, s.binding.AppID, id, s.binding.ViewID)
	if e != nil {
		return false, e
	}
	defer rows.Close()
	policy := apppolicy.TrustedContext{Actor: apppolicy.TrustedActor{ID: id}, Application: apppolicy.Application{ID: s.binding.AppID, OwnerID: s.owner, Exists: true}}
	indices := map[string]int{}
	for rows.Next() {
		var group, kind, resource, action, scope string
		var fields []string
		if e = rows.Scan(&group, &kind, &resource, &action, &scope, &fields); e != nil {
			return false, e
		}
		index, ok := indices[group]
		if !ok {
			index = len(policy.Groups)
			indices[group] = index
			policy.Groups = append(policy.Groups, apppolicy.PermissionGroup{ID: group, ApplicationID: s.binding.AppID, Enabled: true, MemberIDs: []string{id}})
		}
		policy.Groups[index].Grants = append(policy.Groups[index].Grants, apppolicy.Grant{Resource: apppolicy.ResourceRef{ApplicationID: s.binding.AppID, Kind: kind, ID: resource}, Action: apppolicy.Action(action), Rows: apppolicy.RowScope(scope), Fields: fields})
	}
	if e = rows.Err(); e != nil {
		return false, e
	}
	resource := apppolicy.Resource{Ref: apppolicy.ResourceRef{ApplicationID: s.binding.AppID, Kind: "form", ID: s.binding.ViewID}, Exists: true}
	if !apppolicy.Allows(policy, resource, apppolicy.MenuEnter, apppolicy.RowFact{}, "") {
		return false, nil
	}
	for _, field := range s.fields {
		if apppolicy.Allows(policy, resource, apppolicy.DataRead, apppolicy.RowFact{ApplicationID: s.binding.AppID, Exists: true, CreatedBy: id}, field) {
			return true, nil
		}
	}
	return false, nil
}
func (s *approverStrategy) Page(_ context.Context, _ pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]approverItem, error) {
	if !s.validCriteria(raw) || page != s.page {
		return nil, querycontext.ErrInvalid
	}
	return s.observation.Items, nil
}
func (s *approverStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]approverItem], error) {
	_, e := s.Page(ctx, tx, raw, page)
	return s.observation, e
}
func (s *Service) approverSearch(w http.ResponseWriter, r *http.Request, p session.Principal, appID, viewID string) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	if !validID(appID) || !validID(viewID) || r.URL.RawQuery != "" || r.URL.ForceQuery {
		fail(w, r, errInvalid, "")
		return
	}
	raw, e := strictJSON(w, r)
	if e != nil {
		if errors.Is(e, errUnsupportedMediaType) {
			writeEnvelope(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		} else {
			fail(w, r, e, "")
		}
		return
	}
	fields, e := exactObject(raw, []string{"page", "pageSize"}, []string{"queryVersion"})
	var q struct {
		Page         int    `json:"page"`
		PageSize     int    `json:"pageSize"`
		QueryVersion string `json:"queryVersion"`
	}
	if e != nil || json.Unmarshal(raw, &q) != nil {
		fail(w, r, errInvalid, "")
		return
	}
	if _, _, e = appquery.PageWindow(int64(q.Page), int64(q.PageSize)); e != nil {
		fail(w, r, errInvalid, "")
		return
	}
	if _, exists := fields["queryVersion"]; exists && (q.QueryVersion == "" || len(q.QueryVersion) > 256) {
		fail(w, r, errInvalid, "")
		return
	}
	if s.Application.ApproverQueries == nil {
		fail(w, r, session.ErrUnavailable, "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	b := managerBinding{ActorID: p.UserID, AppID: appID, ViewID: viewID}
	criteria, _ := json.Marshal(b)
	strategy := &approverStrategy{app: s.Application, principal: p, binding: b, page: querycontext.Page{Number: q.Page, Size: q.PageSize}}
	result, e := querycontext.Execute(ctx, s.Application.ApproverQueries, p.SessionRef, q.QueryVersion, criteria, strategy.page, strategy)
	if e != nil {
		switch {
		case errors.Is(e, querycontext.ErrChanged):
			writeEnvelope(w, r, 409, "COMMON_QUERY_CHANGED", nil)
		case errors.Is(e, querycontext.ErrExpired), errors.Is(e, querycontext.ErrCAS):
			writeEnvelope(w, r, 409, "COMMON_QUERY_CONTEXT_EXPIRED", nil)
		case errors.Is(e, querycontext.ErrInvalid):
			fail(w, r, errInvalid, "")
		default:
			fail(w, r, e, "")
		}
		return
	}
	if e = s.Authenticator.Renew(ctx, w, r, p); e != nil {
		fail(w, r, e, "")
		return
	}
	writeEnvelope(w, r, 200, "OK", approverResult{Items: result.Items, Total: result.Total, Page: q.Page, PageSize: q.PageSize, QueryVersion: result.Version, SchemaVersion: strategy.schema, PolicyRevision: strategy.policy})
}
