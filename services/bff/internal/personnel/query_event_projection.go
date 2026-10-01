package personnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type EventProjectionPage struct {
	Items       []QueryActivity
	Total       int64
	Fingerprint string
}

// Query only the existing security-barrier activity view. All lookups are sets;
// raw authentication audit privileges are neither requested nor used.
const eventSourceSQL = `WITH source AS (
 SELECT e.*,CASE WHEN jsonb_typeof(e.change_summary->'after'->'name')='string' THEN e.change_summary->'after'->>'name'
 WHEN jsonb_typeof(e.change_summary->'before'->'name')='string' THEN e.change_summary->'before'->>'name'
 ELSE COALESCE(u.account,(CASE e.object_type WHEN 'department' THEN '部门' WHEN 'identity' THEN '身份' WHEN 'template' THEN '权限模板' WHEN 'member' THEN '成员' WHEN 'invitation' THEN '邀请码' ELSE '操作对象' END)||CASE WHEN e.object_id IS NULL THEN '' ELSE ' · '||left(e.object_id::text,8) END) END AS display_object
 FROM personnel.activity_events e LEFT JOIN auth.users u ON e.object_type='member' AND u.id=e.object_id
 WHERE e.occurred_at >= $1 AND e.occurred_at < $2 AND ($3='' OR e.action=$3)
 AND ($4='' OR position(lower($4) in lower(e.actor_account||' '||e.action||' '||COALESCE(e.object_type,'')||' '||COALESCE(e.change_summary::text,'')))>0)
), events AS MATERIALIZED (`
const eventDisplaySQL = `), fields(key,label,position) AS (VALUES
 ('name','名称',1),('description','说明',2),('permissionCodes','权限',3),('templateIds','权限模板',4),('identityIds','身份',5),('departmentIds','部门',6),('parentId','父部门',7)
), labels_catalog(kind,id,name) AS NOT MATERIALIZED (
 SELECT 'permissionCodes',code,name FROM personnel.permission_catalog WHERE enabled
 UNION ALL SELECT 'templateIds',id::text,name FROM personnel.permission_templates
 UNION ALL SELECT 'identityIds',id::text,name FROM personnel.identities
 UNION ALL SELECT 'departmentIds',id::text,name FROM personnel.departments
), raw_values AS (
 SELECT e.id,f.*,s.side,CASE
 WHEN f.key IN ('name','description','parentId') AND jsonb_typeof(e.change_summary->s.side->f.key) IN ('string','null') THEN e.change_summary->s.side->f.key
 WHEN f.key IN ('permissionCodes','templateIds','identityIds','departmentIds') AND jsonb_typeof(e.change_summary->s.side->f.key)='array'
 AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(e.change_summary->s.side->f.key) x WHERE jsonb_typeof(x)<>'string') THEN e.change_summary->s.side->f.key
 ELSE NULL END AS value
 FROM events e CROSS JOIN fields f CROSS JOIN (VALUES('before'),('after'))s(side)
), rendered AS (
 SELECT v.id,v.key,v.label,v.position,v.side,v.value,
 CASE WHEN jsonb_typeof(v.value)='array' THEN COALESCE(string_agg(COALESCE(l.name,CASE WHEN v.key='permissionCodes' THEN a.value#>>'{}' ELSE left(a.value#>>'{}',8) END),'、' ORDER BY a.position) FILTER(WHERE a.value IS NOT NULL),'无')
 WHEN v.key='parentId' THEN COALESCE(parent.name,'无')
 WHEN jsonb_typeof(v.value)='string' THEN COALESCE(NULLIF(v.value#>>'{}',''),'无') ELSE '无' END AS display_value
 FROM raw_values v LEFT JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(v.value)='array' THEN v.value ELSE '[]'::jsonb END) WITH ORDINALITY a(value,position) ON TRUE
 LEFT JOIN labels_catalog l ON l.kind=v.key AND l.id=a.value#>>'{}'
 LEFT JOIN personnel.departments parent ON v.key='parentId' AND parent.id::text=v.value#>>'{}'
 GROUP BY v.id,v.key,v.label,v.position,v.side,v.value,parent.name
), paired AS (
 SELECT b.id,b.key,b.label,b.position,b.value AS old_value,a.value AS new_value,b.display_value AS old_display,a.display_value AS new_display
 FROM rendered b JOIN rendered a ON a.id=b.id AND a.key=b.key AND a.side='after' WHERE b.side='before'
), details AS (
 SELECT id,COALESCE(string_agg(label||'：'||old_display||' → '||new_display,'；' ORDER BY position) FILTER(WHERE old_value IS DISTINCT FROM new_value),'配置已更新') AS detail,
 jsonb_build_object('before',COALESCE(jsonb_object_agg(key,old_value) FILTER(WHERE old_value IS NOT NULL),'{}'::jsonb),
 'after',COALESCE(jsonb_object_agg(key,new_value) FILTER(WHERE new_value IS NOT NULL),'{}'::jsonb)) AS safe_summary
 FROM paired GROUP BY id
), q AS (
 SELECT e.*,CASE WHEN e.change_summary IS NULL THEN NULL ELSE d.safe_summary END AS safe_summary,
 CASE WHEN e.change_summary IS NULL AND e.action='INVITATION_CREATED' THEN '已生成单次邀请码' WHEN e.change_summary IS NULL THEN '配置已更新' ELSE d.detail END AS display_detail
 FROM events e JOIN details d ON d.id=e.id
)
SELECT q.id::text,q.occurred_at,q.actor_account,q.action,q.object_type,q.object_id::text,q.safe_summary,q.outcome,q.display_object,q.display_detail FROM q WHERE `

func eventProjectionQuery(in EventQueryInput, pageOnly bool) (string, []any, PageQuery, error) {
	page, err := normalizedPage(in.PageQuery)
	if err != nil {
		return "", nil, page, err
	}
	if in.From.IsZero() || in.To.IsZero() || !in.From.Before(in.To) || in.From.Nanosecond()%1000 != 0 || in.To.Nanosecond()%1000 != 0 || in.Action != "" && !activityActions[in.Action] {
		return "", nil, page, ErrInvalid
	}
	direction := "DESC"
	if in.Sort != nil {
		if in.Sort.Key != "occurredAt" || (in.Sort.Direction != "asc" && in.Sort.Direction != "desc") {
			return "", nil, page, ErrInvalid
		}
		direction = strings.ToUpper(in.Sort.Direction)
	}
	var raw []byte
	if in.Filter != nil {
		raw, err = projectionJSON(in.Filter)
		if err != nil {
			return "", nil, page, err
		}
	}
	plan, err := CompileFilter("events", raw, 5)
	if err != nil {
		return "", nil, page, err
	}
	args := append([]any{in.From, in.To, in.Action, page.Search}, plan.Arguments...)
	order := " ORDER BY q.occurred_at " + direction + ",q.id " + direction
	limit := ""
	if pageOnly {
		limit = fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, page.PageSize, int64(page.Page-1)*int64(page.PageSize))
	}
	// Detail conditions necessarily evaluate rendered candidates. Other criteria
	// select actual page IDs first; this path never computes a complete digest.
	earlyPage := pageOnly && !filterHasField(in.Filter, "detail")
	selection := "SELECT * FROM source"
	predicate := plan.Predicate
	if earlyPage {
		selection = "SELECT * FROM source q WHERE " + predicate + order + limit
		predicate = "TRUE"
		limit = ""
	}
	return eventSourceSQL + selection + eventDisplaySQL + predicate + order + limit, args, page, nil
}
func filterHasField(group *FilterGroup, field string) bool {
	if group == nil {
		return false
	}
	var visit func(json.RawMessage) bool
	visit = func(raw json.RawMessage) bool {
		var n struct {
			Field    string
			Children []json.RawMessage
		}
		_ = json.Unmarshal(raw, &n)
		if n.Field == field {
			return true
		}
		for _, c := range n.Children {
			if visit(c) {
				return true
			}
		}
		return false
	}
	for _, c := range group.Children {
		if visit(c) {
			return true
		}
	}
	return false
}

var eventActionLabels = map[string]string{"DEPARTMENT_CREATED": "新建部门", "DEPARTMENT_UPDATED": "重命名部门", "DEPARTMENT_DELETED": "删除部门", "IDENTITY_CREATED": "新建身份", "IDENTITY_UPDATED": "修改身份", "IDENTITY_DELETED": "删除身份", "TEMPLATE_CREATED": "新建权限模板", "TEMPLATE_UPDATED": "修改权限模板", "TEMPLATE_DELETED": "删除权限模板", "MEMBER_IDENTITIES_UPDATED": "分配身份", "MEMBER_GROUPS_UPDATED": "调整分组", "INVITATION_CREATED": "邀请成员"}

func readEventProjection(rows pgx.Rows) (QueryActivity, error) {
	var e QueryActivity
	var raw []byte
	if err := rows.Scan(&e.ID, &e.OccurredAt, &e.ActorAccount, &e.Action, &e.ObjectType, &e.ObjectID, &raw, &e.Outcome, &e.Display.Object, &e.Display.Detail); err != nil {
		return e, err
	}
	e.OccurredAt = e.OccurredAt.UTC()
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &e.Summary); err != nil {
			return e, err
		}
	}
	e.Display.Action = eventActionLabels[e.Action]
	if e.Display.Action == "" {
		e.Display.Action = e.Action
	}
	e.Display.Outcome = "失败"
	if e.Outcome == "success" {
		e.Display.Outcome = "已完成"
	}
	return e, nil
}
func scanEventProjection(ctx context.Context, tx pgx.Tx, in EventQueryInput) (EventProjectionPage, error) {
	sql, args, page, err := eventProjectionQuery(in, false)
	if err != nil {
		return EventProjectionPage{}, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return EventProjectionPage{}, err
	}
	defer rows.Close()
	result := EventProjectionPage{Items: []QueryActivity{}}
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos:q36:event-projection:v1\x00"))
	offset := int64(page.Page-1) * int64(page.PageSize)
	for rows.Next() {
		e, err := readEventProjection(rows)
		if err != nil {
			return result, err
		}
		// Object versions and non-visible audit properties cannot invalidate a table.
		projected := struct {
			ID         string
			OccurredAt time.Time
			Actor      string
			Display    ActivityDisplay
		}{e.ID, e.OccurredAt, e.ActorAccount, e.Display}
		if err = fingerprintFrame(h, projected); err != nil {
			return result, err
		}
		if result.Total >= offset && result.Total < offset+int64(page.PageSize) {
			result.Items = append(result.Items, e)
		}
		result.Total++
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if err = fingerprintFrame(h, result.Total); err != nil {
		return result, err
	}
	result.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return result, nil
}
func scanEventPage(ctx context.Context, tx pgx.Tx, in EventQueryInput) ([]QueryActivity, error) {
	sql, args, _, err := eventProjectionQuery(in, true)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []QueryActivity{}
	for rows.Next() {
		e, err := readEventProjection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
