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
		for i, v := range []json.RawMessage{delta.Before, delta.After} {
			if i == 0 && m.RecordVersionBefore == 0 && string(v) == "null" {
				// Before creation there is no record; required applies to its
				// resulting value, not to this history absence marker.
				continue
			}
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
 SELECT jsonb_agg(jsonb_build_object('fieldId',v.field_id,'fieldKind',v.field_kind,'before',v.old_value,'after',v.new_value,'fieldLabel',f.definition->>'name','fieldDeleted',f.removed,'valueLabels','{}'::jsonb) ORDER BY v.field_id) changes
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
	rows.Close()
	if len(page.Items) > in.PageSize {
		page.HasMore = true
		page.Items = page.Items[:in.PageSize]
	}
	if e = historyLabels(c, tx, in.AppID, in.TableID, page.Items); e != nil {
		return HistoryPage{}, e
	}
	return page, nil
}

// Labels are hydrated after permitted deltas and event pagination are fixed,
// in the same read transaction. No hidden field or whole candidate set is read.
func historyLabels(c context.Context, tx pgx.Tx, app, table string, events []HistoryEvent) error {
	requested := map[string]map[string]bool{"member": {}, "department": {}, "option": {}}
	fieldIDs := map[string]bool{}
	idsFor := func(change HistoryChange) ([]string, error) {
		ids := []string{}
		for _, raw := range []json.RawMessage{change.Before, change.After} {
			if string(raw) == "null" {
				continue
			}
			if change.FieldKind == "multi_select" {
				var list []string
				if json.Unmarshal(raw, &list) != nil {
					return nil, ErrUnavailable
				}
				ids = append(ids, list...)
			} else {
				var id string
				if json.Unmarshal(raw, &id) != nil {
					return nil, ErrUnavailable
				}
				ids = append(ids, id)
			}
		}
		for _, id := range ids {
			if !appfields.ValidID(id) {
				return nil, ErrUnavailable
			}
		}
		return ids, nil
	}
	kindFor := func(kind string) string {
		if kind == "single_select" || kind == "multi_select" {
			return "option"
		}
		if kind == "member" || kind == "department" {
			return kind
		}
		return ""
	}
	for _, event := range events {
		for _, change := range event.Changes {
			kind := kindFor(change.FieldKind)
			if kind == "" {
				continue
			}
			ids, e := idsFor(change)
			if e != nil {
				return e
			}
			for _, id := range ids {
				requested[kind][id] = true
			}
			if kind == "option" {
				fieldIDs[change.FieldID] = true
			}
		}
	}
	labels := map[string]map[string]HistoryValueLabel{"member": {}, "department": {}, "option": {}}
	keys := func(set map[string]bool) []string {
		ids := []string{}
		for id := range set {
			ids = append(ids, id)
		}
		return ids
	}
	for _, kind := range []string{"member", "department", "option"} {
		if len(requested[kind]) == 0 {
			continue
		}
		var rows pgx.Rows
		var e error
		if kind == "option" {
			rows, e = tx.Query(c, "SELECT field_id::text||':'||option_id::text,label,removed FROM applications.field_option_tombstones WHERE app_id=$1 AND table_id=$2 AND field_id=ANY($3::uuid[]) AND option_id=ANY($4::uuid[])", app, table, keys(fieldIDs), keys(requested[kind]))
		} else {
			source := "applications.member_sources"
			if kind == "department" {
				source = "applications.department_sources"
			}
			rows, e = tx.Query(c, "SELECT id::text,label,status='deleted' FROM "+source+" WHERE id=ANY($1::uuid[])", keys(requested[kind]))
		}
		if e != nil {
			return ErrUnavailable
		}
		for rows.Next() {
			var id, label string
			var deleted bool
			if rows.Scan(&id, &label, &deleted) != nil {
				rows.Close()
				return ErrUnavailable
			}
			copy := label
			labels[kind][id] = HistoryValueLabel{Label: &copy, Deleted: deleted}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return ErrUnavailable
		}
	}
	for i := range events {
		for j := range events[i].Changes {
			change := &events[i].Changes[j]
			change.ValueLabels = map[string]HistoryValueLabel{}
			kind := kindFor(change.FieldKind)
			if kind == "" {
				continue
			}
			ids, e := idsFor(*change)
			if e != nil {
				return e
			}
			for _, id := range ids {
				key := id
				if kind == "option" {
					key = change.FieldID + ":" + id
				}
				label, ok := labels[kind][key]
				if !ok {
					label = HistoryValueLabel{LabelUnavailable: true}
				}
				change.ValueLabels[id] = label
			}
		}
	}
	return nil
}
