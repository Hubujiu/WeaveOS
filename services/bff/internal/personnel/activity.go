package personnel

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"time"
)

var activityActions = map[string]bool{"DEPARTMENT_CREATED": true, "DEPARTMENT_UPDATED": true, "DEPARTMENT_DELETED": true, "IDENTITY_CREATED": true, "IDENTITY_UPDATED": true, "IDENTITY_DELETED": true, "TEMPLATE_CREATED": true, "TEMPLATE_UPDATED": true, "TEMPLATE_DELETED": true, "MEMBER_IDENTITIES_UPDATED": true, "MEMBER_GROUPS_UPDATED": true, "INVITATION_CREATED": true}

func (a *Application) Events(ctx context.Context, p session.Principal, query EventQuery) (Page[Activity], error) {
	page, err := normalizedPage(query.PageQuery)
	if err != nil {
		return Page[Activity]{}, err
	}
	if query.Action != "" && !activityActions[query.Action] {
		return Page[Activity]{}, ErrInvalid
	}
	if query.To.IsZero() {
		query.To = time.Now().UTC()
	}
	if query.From.IsZero() {
		query.From = query.To.AddDate(0, 0, -7)
	}
	if !query.From.Before(query.To) {
		return Page[Activity]{}, ErrInvalid
	}
	result := Page[Activity]{Items: []Activity{}, Page: page.Page, PageSize: page.PageSize}
	tx, err := a.read(ctx, p)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	filter := ` WHERE occurred_at >= $1 AND occurred_at < $2 AND ($3='' OR action=$3)
 AND ($4='' OR position(lower($4) in lower(actor_account||' '||action||' '||COALESCE(object_type,'')||' '||COALESCE(change_summary::text,'')))>0)`
	args := []any{query.From, query.To, query.Action, page.Search}
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.activity_events"+filter, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, "SELECT id::text,occurred_at,actor_account,action,object_type,object_id::text,change_summary,outcome FROM personnel.activity_events"+filter+" ORDER BY occurred_at DESC,id DESC LIMIT $5 OFFSET $6", append(args, page.PageSize, (page.Page-1)*page.PageSize)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var value Activity
		var raw []byte
		if err := rows.Scan(&value.ID, &value.OccurredAt, &value.ActorAccount, &value.Action, &value.ObjectType, &value.ObjectID, &raw, &value.Outcome); err != nil {
			return result, err
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &value.Summary); err != nil {
				return result, err
			}
		}
		result.Items = append(result.Items, value)
	}
	return result, rows.Err()
}
func (a *Application) Catalog(ctx context.Context, p session.Principal, queryVersion ...string) ([]Permission, error) {
	ctx, cancel := context.WithTimeout(ctx, queryDeadline)
	defer cancel()
	version := ""
	if len(queryVersion) > 0 {
		version = queryVersion[0]
	}
	tx, receipt, err := a.readWithQuery(ctx, p, version)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	result := []Permission{}
	rows, err := tx.Query(ctx, "SELECT code,name,category,app_id,enabled FROM personnel.permission_catalog WHERE enabled ORDER BY category DESC,code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		value := Permission{Sources: []Source{}}
		if err := rows.Scan(&value.Code, &value.Name, &value.Category, &value.AppID, &value.Enabled); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	rows.Close()
	return result, receipt.commit(ctx, tx)
}
