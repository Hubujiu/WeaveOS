package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
)

func (RecordHistoryStore) Append(c context.Context, tx pgx.Tx, m HistoryMutation) error {
	if tx == nil {
		return ErrUnavailable
	}
	for _, id := range []string{m.AppID, m.TableID, m.ViewID, m.RecordID, m.ActorID, m.OperationID} {
		if !appfields.ValidID(id) {
			return applications.ErrResourceInvalid
		}
	}
	if m.RecordVersionBefore < 0 || m.RecordVersionAfter <= m.RecordVersionBefore || m.RecordVersionAfter > maxVersion || m.OccurredAt.IsZero() || m.Origin != "ordinary" && m.Origin != "task_save" || m.Changes == nil {
		return applications.ErrResourceInvalid
	}
	if len(m.Changes) == 0 {
		return nil
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, delta := range m.Changes {
		if !appfields.ValidID(delta.FieldID) || seen[delta.FieldID] || jsonEqual(delta.Before, delta.After) {
			return applications.ErrResourceInvalid
		}
		seen[delta.FieldID] = true
		ids = append(ids, delta.FieldID)
	}
	fields, e := historyFields(c, tx, m.AppID, m.TableID, ids)
	if e != nil {
		return e
	}
	for _, delta := range m.Changes {
		f := fields[delta.FieldID]
		if delta.FieldKind != f.Kind {
			return applications.ErrResourceInvalid
		}
		for _, v := range []json.RawMessage{delta.Before, delta.After} {
			canonical, e := appfields.NormalizeValue(f, v)
			if e != nil || !jsonEqual(v, canonical) {
				return applications.ErrResourceInvalid
			}
		}
	}
	var event string
	e = tx.QueryRow(c, `INSERT INTO applications.record_change_events(app_id,table_id,record_id,source_view_id,actor_user_id,operation_id,record_version_before,record_version_after,origin,opaque_task_ref,occurred_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`, m.AppID, m.TableID, m.RecordID, m.ViewID, m.ActorID, m.OperationID, m.RecordVersionBefore, m.RecordVersionAfter, m.Origin, m.OpaqueTaskRef, m.OccurredAt).Scan(&event)
	if e != nil {
		return ErrUnavailable
	}
	raw, e := json.Marshal(m.Changes)
	if e != nil {
		return applications.ErrResourceInvalid
	}
	_, e = tx.Exec(c, `INSERT INTO applications.record_change_values(event_id,app_id,table_id,field_id,field_kind,old_value,new_value)
 SELECT $1,$2,$3,(d->>'fieldId')::uuid,d->>'fieldKind',d->'before',d->'after' FROM jsonb_array_elements($4::jsonb) d`, event, m.AppID, m.TableID, raw)
	if e != nil {
		return ErrUnavailable
	}
	return nil
}

func (RecordHistoryStore) Page(c context.Context, tx pgx.Tx, in HistoryRead) (HistoryPage, error) {
	page := HistoryPage{Items: []HistoryEvent{}}
	if tx == nil {
		return page, ErrUnavailable
	}
	for _, id := range []string{in.AppID, in.TableID, in.ViewID, in.RecordID} {
		if !appfields.ValidID(id) {
			return page, applications.ErrResourceInvalid
		}
	}
	if in.PageSize < 1 || in.PageSize > 100 || (in.AfterTime == nil) != (in.AfterID == nil) || in.AfterID != nil && !appfields.ValidID(*in.AfterID) {
		return page, applications.ErrInvalid
	}
	for _, id := range in.FieldIDs {
		if !appfields.ValidID(id) {
			return page, applications.ErrResourceInvalid
		}
	}
	rows, e := tx.Query(c, `SELECT e.id::text,e.record_version_before,e.record_version_after,e.actor_user_id::text,e.occurred_at,e.origin,delta.changes
 FROM applications.record_change_events e
 CROSS JOIN LATERAL (
 SELECT jsonb_agg(jsonb_build_object('fieldId',v.field_id,'fieldKind',v.field_kind,'before',v.old_value,'after',v.new_value) ORDER BY v.field_id) changes
 FROM applications.record_change_values v JOIN applications.fields f ON f.app_id=v.app_id AND f.table_id=v.table_id AND f.id=v.field_id
 WHERE v.event_id=e.id AND ($4 OR NOT f.removed AND v.field_id=ANY($5::uuid[]))
 ) delta
 WHERE e.app_id=$1 AND e.table_id=$2 AND e.record_id=$3 AND delta.changes IS NOT NULL
 AND EXISTS(SELECT 1 FROM applications.form_views fv JOIN applications.menu_resources mr ON mr.app_id=fv.app_id AND mr.resource_kind='form' AND mr.resource_id=fv.id WHERE fv.app_id=$1 AND fv.table_id=$2 AND fv.id=$9)
 AND ($6::timestamptz IS NULL OR (e.occurred_at,e.id)<($6,$7::uuid))
 ORDER BY e.occurred_at DESC,e.id DESC LIMIT $8`, in.AppID, in.TableID, in.RecordID, in.AllFields, in.FieldIDs, in.AfterTime, in.AfterID, in.PageSize+1, in.ViewID)
	if e != nil {
		return page, ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var event HistoryEvent
		var raw json.RawMessage
		if rows.Scan(&event.ID, &event.RecordVersionBefore, &event.RecordVersionAfter, &event.ActorID, &event.OccurredAt, &event.Origin, &raw) != nil || json.Unmarshal(raw, &event.Changes) != nil {
			return HistoryPage{}, ErrUnavailable
		}
		page.Items = append(page.Items, event)
	}
	if rows.Err() != nil {
		return HistoryPage{}, ErrUnavailable
	}
	if len(page.Items) > in.PageSize {
		page.HasMore = true
		page.Items = page.Items[:in.PageSize]
	}
	return page, nil
}
