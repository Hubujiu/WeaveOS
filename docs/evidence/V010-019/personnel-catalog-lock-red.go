package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"strconv"
)

// One statement supplies one MVCC snapshot for the account, identities, templates,
// catalog and sources. No Session or cross-request permission snapshot is trusted.
const accessSQL = `
WITH me AS (
 SELECT id, account, is_bootstrap_admin FROM auth.users
 WHERE id=$1 AND status='active' AND auth_version::text=$2
), my_identities AS (
 SELECT i.* FROM personnel.identities i JOIN personnel.member_identities m ON m.identity_id=i.id JOIN me ON me.id=m.user_id
), sources AS (
 SELECT p.permission_code, i.id AS identity_id, i.name AS identity_name, NULL::uuid AS template_id, NULL::varchar AS template_name
 FROM my_identities i JOIN personnel.identity_permissions p ON p.identity_id=i.id
 UNION ALL
 SELECT p.permission_code, i.id, i.name, t.id, t.name
 FROM my_identities i JOIN personnel.identity_templates it ON it.identity_id=i.id
 JOIN personnel.permission_templates t ON t.id=it.template_id JOIN personnel.template_permissions p ON p.template_id=t.id
), grants AS (
 SELECT c.*, COALESCE((
 SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object('identityId',s.identity_id,'identityName',s.identity_name,'templateId',s.template_id,'templateName',s.template_name)) ORDER BY s.identity_id,s.template_id)
 FROM sources s WHERE s.permission_code=c.code),'[]'::jsonb) AS sources
 FROM personnel.permission_catalog c, me
 WHERE c.enabled AND (me.is_bootstrap_admin OR EXISTS(SELECT 1 FROM sources s WHERE s.permission_code=c.code))
)
SELECT jsonb_build_object(
 'user',jsonb_build_object('id',me.id,'account',me.account),
 'bootstrapAdmin',me.is_bootstrap_admin,
 'personnelManage',me.is_bootstrap_admin OR EXISTS(SELECT 1 FROM grants WHERE code='personnel.manage'),
 'identities',COALESCE((SELECT jsonb_agg(jsonb_build_object(
   'id',i.id,'name',i.name,'description',i.description,'version',i.version,
   'permissionCodes',COALESCE((SELECT jsonb_agg(permission_code ORDER BY permission_code) FROM personnel.identity_permissions WHERE identity_id=i.id),'[]'::jsonb),
   'templateIds',COALESCE((SELECT jsonb_agg(template_id ORDER BY template_id) FROM personnel.identity_templates WHERE identity_id=i.id),'[]'::jsonb),
   'affectedMembers',(SELECT count(*) FROM personnel.member_identities WHERE identity_id=i.id),
   'affectedIdentities',0
 ) ORDER BY i.created_at,i.id) FROM my_identities i),'[]'::jsonb),
 'permissions',COALESCE((SELECT jsonb_agg(jsonb_build_object('code',code,'name',name,'category',category,'appId',app_id,'enabled',enabled,'sources',sources) ORDER BY code) FROM grants),'[]'::jsonb),
 'applications',COALESCE((SELECT jsonb_agg(jsonb_build_object('code',code,'name',name,'category',category,'appId',app_id,'enabled',enabled,'sources',sources) ORDER BY code) FROM grants WHERE category='application'),'[]'::jsonb)
) FROM me
`

type accessReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readAccess(ctx context.Context, db accessReader, p session.Principal) (Access, error) {
	var raw []byte
	if err := db.QueryRow(ctx, accessSQL, p.UserID, p.Record.AuthVersion).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Access{}, session.ErrUnauthorized
		}
		return Access{}, err
	}
	var result Access
	if err := json.Unmarshal(raw, &result); err != nil {
		return Access{}, err
	}
	return result, nil
}
func (a *Application) Me(ctx context.Context, p session.Principal) (Access, error) {
	if a == nil || a.Pool == nil {
		return Access{}, session.ErrUnavailable
	}
	return readAccess(ctx, a.Pool, p)
}
func (a *Application) AllowApplication(ctx context.Context, p session.Principal, appID string) error {
	access, err := a.Me(ctx, p)
	if err != nil {
		return err
	}
	for _, app := range access.Applications {
		if app.AppID != nil && *app.AppID == appID {
			return nil
		}
	}
	return ErrDenied
}

// The authentication owner can call this inside its existing invitation transaction.
// Personnel only reads/locks auth.users and owns its configuration, not credentials.
func (a *Application) AuthorizeWrite(ctx context.Context, tx pgx.Tx, p session.Principal) error {
	var status, snapshot string
	if err := tx.QueryRow(ctx, "SELECT status,auth_version::text FROM auth.users WHERE id=$1 FOR SHARE", p.UserID).Scan(&status, &snapshot); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return session.ErrUnauthorized
		}
		return err
	}
	if !security.CurrentUserMatchesSession(p.Record.AuthVersion, status, parseVersion(snapshot)) {
		return session.ErrUnauthorized
	}
	if _, err := tx.Exec(ctx, "INSERT INTO personnel.member_configuration(user_id) VALUES($1) ON CONFLICT DO NOTHING", p.UserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "SELECT user_id FROM personnel.member_configuration WHERE user_id=$1 FOR SHARE", p.UserID); err != nil {
		return err
	}
	for _, sql := range []string{
		"SELECT i.id FROM personnel.identities i JOIN personnel.member_identities m ON m.identity_id=i.id WHERE m.user_id=$1 ORDER BY i.id FOR SHARE OF i",
		"SELECT t.id FROM personnel.permission_templates t WHERE EXISTS(SELECT 1 FROM personnel.identity_templates it JOIN personnel.member_identities m ON m.identity_id=it.identity_id WHERE it.template_id=t.id AND m.user_id=$1) ORDER BY t.id FOR SHARE OF t",
		`SELECT c.code FROM personnel.permission_catalog c WHERE EXISTS(
   SELECT 1 FROM personnel.member_identities m JOIN personnel.identity_permissions ip ON ip.identity_id=m.identity_id WHERE m.user_id=$1 AND ip.permission_code=c.code
   UNION ALL
   SELECT 1 FROM personnel.member_identities m JOIN personnel.identity_templates it ON it.identity_id=m.identity_id JOIN personnel.template_permissions tp ON tp.template_id=it.template_id WHERE m.user_id=$1 AND tp.permission_code=c.code
  ) ORDER BY c.code FOR SHARE OF c`,
	} {
		if _, err := tx.Exec(ctx, sql, p.UserID); err != nil {
			return err
		}
	}
	access, err := readAccess(ctx, tx, p)
	if err != nil {
		return err
	}
	if !access.PersonnelManage {
		return ErrDenied
	}
	return nil
}

func parseVersion(value string) int64 { n, _ := strconv.ParseInt(value, 10, 64); return n }
