package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// This is the current personnel member vocabulary, not a general SQL platform.
// Adding a field requires a reviewed mapping, visible projection and tests.
// Both uses bind u to a fixed SQL alias; no client identifier enters this SQL.
const memberManageExpression = `(u.status='active' AND (u.is_bootstrap_admin OR (
 EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='personnel.manage' AND enabled)
 AND EXISTS(
  SELECT 1 FROM personnel.member_identities mi
  JOIN personnel.identity_permissions ip ON ip.identity_id=mi.identity_id
  WHERE mi.user_id=u.id AND ip.permission_code='personnel.manage'
  UNION ALL
  SELECT 1 FROM personnel.member_identities mi
  JOIN personnel.identity_templates it ON it.identity_id=mi.identity_id
  JOIN personnel.template_permissions tp ON tp.template_id=it.template_id
  WHERE mi.user_id=u.id AND tp.permission_code='personnel.manage'
  LIMIT 1
 ))))`

// The candidate's optional relationship fields exist only for filter predicates.
// PostgreSQL prunes unused expressions. Materializing the selected IDs forms a
// limit boundary before visible label aggregation; no whole-table label CTE.
const memberCandidateSQL = `WITH q AS (
 SELECT u.id,u.created_at,u.account,u.status,u.is_bootstrap_admin,
 ARRAY(SELECT dm.department_id FROM personnel.department_members dm WHERE dm.user_id=u.id) AS department_ids,
 ARRAY(SELECT mi.identity_id FROM personnel.member_identities mi WHERE mi.user_id=u.id) AS identity_ids,
 ` + memberManageExpression + ` AS personnel_manage FROM auth.users u
), selected AS MATERIALIZED (
 SELECT q.id,q.created_at,q.account,q.status,q.is_bootstrap_admin FROM q
 WHERE ($1='' OR position(lower($1) in lower(q.account))>0)
 AND ($2='' OR NULLIF($2,'')::uuid=ANY(q.department_ids))
 AND ($3='' OR NULLIF($3,'')::uuid=ANY(q.identity_ids)) AND `

const memberSelectedDisplaySQL = `), department_labels AS (
 SELECT dm.user_id,jsonb_agg(jsonb_build_object('id',d.id,'name',d.name) ORDER BY d.id) AS labels
 FROM selected u JOIN personnel.department_members dm ON dm.user_id=u.id
 JOIN personnel.departments d ON d.id=dm.department_id GROUP BY dm.user_id
), identity_labels AS (
 SELECT mi.user_id,jsonb_agg(jsonb_build_object('id',i.id,'name',i.name) ORDER BY i.id) AS labels
 FROM selected u JOIN personnel.member_identities mi ON mi.user_id=u.id
 JOIN personnel.identities i ON i.id=mi.identity_id GROUP BY mi.user_id
), manage_users AS (
 SELECT mi.user_id FROM selected u JOIN personnel.member_identities mi ON mi.user_id=u.id
 JOIN personnel.identity_permissions ip ON ip.identity_id=mi.identity_id WHERE ip.permission_code='personnel.manage'
 UNION
 SELECT mi.user_id FROM selected u JOIN personnel.member_identities mi ON mi.user_id=u.id
 JOIN personnel.identity_templates it ON it.identity_id=mi.identity_id
 JOIN personnel.template_permissions tp ON tp.template_id=it.template_id WHERE tp.permission_code='personnel.manage'
)
SELECT u.id::text,u.created_at,u.account,u.status,u.is_bootstrap_admin,
 (u.status='active' AND (u.is_bootstrap_admin OR (
 EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='personnel.manage' AND enabled)
 AND EXISTS(SELECT 1 FROM manage_users m WHERE m.user_id=u.id)))) AS personnel_manage,
 COALESCE(d.labels,'[]'::jsonb),COALESCE(i.labels,'[]'::jsonb)
FROM selected u LEFT JOIN department_labels d ON d.user_id=u.id LEFT JOIN identity_labels i ON i.user_id=u.id
ORDER BY u.created_at ASC,u.id ASC`

// A parameterized page-local aggregate prevents the optimizer from replacing
// seven indexed relationship lookups with a scan of every member's relations.
// The full fingerprint path above uses set aggregation for all selected users.
const memberPageDisplaySQL = `)
SELECT u.id::text,u.created_at,u.account,u.status,u.is_bootstrap_admin,
 ` + memberManageExpression + ` AS personnel_manage,
 COALESCE(d.labels,'[]'::jsonb),COALESCE(i.labels,'[]'::jsonb)
FROM selected u
LEFT JOIN LATERAL (
 SELECT jsonb_agg(jsonb_build_object('id',d.id,'name',d.name) ORDER BY d.id) AS labels
 FROM personnel.department_members dm JOIN personnel.departments d ON d.id=dm.department_id
 WHERE dm.user_id=u.id
) d ON TRUE
LEFT JOIN LATERAL (
 SELECT jsonb_agg(jsonb_build_object('id',i.id,'name',i.name) ORDER BY i.id) AS labels
 FROM personnel.member_identities mi JOIN personnel.identities i ON i.id=mi.identity_id
 WHERE mi.user_id=u.id
) i ON TRUE
ORDER BY u.created_at ASC,u.id ASC`

func memberSelectQuery(input MemberQueryInput, pageOnly bool) (string, []any, PageQuery, error) {
	predicate, args, page, err := memberProjectionFilter(input)
	if err != nil {
		return "", nil, page, err
	}
	selection := memberCandidateSQL + predicate + " ORDER BY q.created_at ASC,q.id ASC"
	if pageOnly {
		selection += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, page.PageSize, int64(page.Page-1)*int64(page.PageSize))
		return selection + memberPageDisplaySQL, args, page, nil
	}
	return selection + memberSelectedDisplaySQL, args, page, nil
}
func memberPageQuery(input MemberQueryInput) (string, []any, PageQuery, error) {
	return memberSelectQuery(input, true)
}

// The engine must call this only after checking revisions in the same authorized
// RR. It reuses the context total/fingerprint, never scans or hashes full results.
// SQL still pays predicate, ordering and offset costs; this is not O(1) paging.
func scanMemberPage(ctx context.Context, tx pgx.Tx, input MemberQueryInput) ([]MemberProjection, error) {
	sql, args, _, err := memberPageQuery(input)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []MemberProjection{}
	for rows.Next() {
		row, err := readMemberProjection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
func readMemberProjection(rows pgx.Rows) (MemberProjection, error) {
	var row MemberProjection
	var departments, identities []byte
	if err := rows.Scan(&row.ID, &row.CreatedAt, &row.Account, &row.Status, &row.BootstrapAdmin, &row.PersonnelManage, &departments, &identities); err != nil {
		return row, err
	}
	row.CreatedAt = row.CreatedAt.UTC()
	if err := json.Unmarshal(departments, &row.Departments); err != nil {
		return row, err
	}
	if err := json.Unmarshal(identities, &row.Identities); err != nil {
		return row, err
	}
	return row, nil
}
