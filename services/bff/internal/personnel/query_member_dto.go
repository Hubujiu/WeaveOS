package personnel

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

// Hydrate only the selected page. Keep the existing rich Member DTO for edits
// and permission explanations; these unused metadata fields are not fingerprints.
func hydrateQueryMembers(ctx context.Context, tx pgx.Tx, page []MemberProjection) ([]Member, error) {
	result := []Member{}
	if len(page) == 0 {
		return result, nil
	}
	ids := make([]string, len(page))
	for i, row := range page {
		ids[i] = row.ID
	}
	sql := `WITH selected AS MATERIALIZED (SELECT u.*,p.position FROM unnest($1::uuid[]) WITH ORDINALITY p(id,position) JOIN auth.users u ON u.id=p.id), sources AS (
 SELECT u.id AS user_id,ip.permission_code,i.id AS identity_id,i.name AS identity_name,NULL::uuid AS template_id,NULL::varchar AS template_name
 FROM selected u JOIN personnel.member_identities mi ON mi.user_id=u.id JOIN personnel.identities i ON i.id=mi.identity_id JOIN personnel.identity_permissions ip ON ip.identity_id=i.id
 UNION ALL
 SELECT u.id,tp.permission_code,i.id,i.name,t.id,t.name FROM selected u JOIN personnel.member_identities mi ON mi.user_id=u.id
 JOIN personnel.identities i ON i.id=mi.identity_id JOIN personnel.identity_templates it ON it.identity_id=i.id JOIN personnel.permission_templates t ON t.id=it.template_id JOIN personnel.template_permissions tp ON tp.template_id=t.id
 )
 SELECT jsonb_build_object('id',u.id,'account',u.account,'status',u.status,'bootstrapAdmin',u.is_bootstrap_admin,
 'version',COALESCE((SELECT version FROM personnel.member_configuration WHERE user_id=u.id),0),
 'departmentIds',COALESCE((SELECT jsonb_agg(department_id ORDER BY department_id) FROM personnel.department_members WHERE user_id=u.id),'[]'::jsonb),
 'identityIds',COALESCE((SELECT jsonb_agg(identity_id ORDER BY identity_id) FROM personnel.member_identities WHERE user_id=u.id),'[]'::jsonb),
 'departments',COALESCE((SELECT jsonb_agg(` + departmentJSON + ` ORDER BY d.id) FROM personnel.departments d JOIN personnel.department_members dm ON dm.department_id=d.id WHERE dm.user_id=u.id),'[]'::jsonb),
 'identities',COALESCE((SELECT jsonb_agg(` + definitionSelect(Identity) + ` ORDER BY d.id) FROM personnel.identities d JOIN personnel.member_identities mi ON mi.identity_id=d.id WHERE mi.user_id=u.id),'[]'::jsonb),
 'permissions',COALESCE((SELECT jsonb_agg(jsonb_build_object('code',c.code,'name',c.name,'category',c.category,'appId',c.app_id,'enabled',c.enabled,
 'sources',COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object('identityId',s.identity_id,'identityName',s.identity_name,'templateId',s.template_id,'templateName',s.template_name)) ORDER BY s.identity_id,s.template_id) FROM sources s WHERE s.user_id=u.id AND s.permission_code=c.code),'[]'::jsonb)) ORDER BY c.code)
 FROM personnel.permission_catalog c WHERE u.status='active' AND c.enabled AND (u.is_bootstrap_admin OR EXISTS(SELECT 1 FROM sources s WHERE s.user_id=u.id AND s.permission_code=c.code))),'[]'::jsonb)) FROM selected u ORDER BY u.position`
	rows, err := tx.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var m Member
		if err = json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
