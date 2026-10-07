package apprecordservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
)

// captureWorkflowEvidence is internal only: full values are retained for audit,
// while VisibleFieldIDs records the live row-specific original read mask.
// facts must be loaded by BeginRecordRead (same RR transaction) or by the
// locked write lifecycle. It does not commit, store, authorize a workflow task,
// or expose this complete bundle through HTTP.
func captureWorkflowEvidence(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, recordID string) (ev.Bundle, error) {
	if captureNilPort(ctx) || captureNilPort(tx) || facts.SchemaVersion < 1 {
		return ev.Bundle{}, ev.ErrInvalid
	}
	for _, id := range []string{recordID, facts.App.ID, facts.ViewID, facts.TableID, facts.Actor.ID} {
		if !appfields.ValidID(id) {
			return ev.Bundle{}, ev.ErrInvalid
		}
	}
	if len(facts.Fields) > ev.MaxBundleBytes {
		return ev.Bundle{}, ev.ErrTooLarge
	}
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None {
		return ev.Bundle{}, applications.ErrDenied
	}
	var schema int64
	var ready bool
	err := tx.QueryRow(ctx, `SELECT t.schema_version,t.schema_ready
 FROM applications.apps a
 JOIN applications.form_views v ON v.app_id=a.id
 JOIN applications.logical_tables t ON t.app_id=v.app_id AND t.id=v.table_id
 JOIN applications.menu_resources m ON m.app_id=v.app_id AND m.resource_kind='form' AND m.resource_id=v.id
 WHERE a.id=$1 AND v.id=$2 AND t.id=$3`, facts.App.ID, facts.ViewID, facts.TableID).Scan(&schema, &ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return ev.Bundle{}, applications.ErrMissing
	}
	if err != nil {
		return ev.Bundle{}, err
	}
	if !ready {
		return ev.Bundle{}, &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
	}
	if schema != facts.SchemaVersion {
		return ev.Bundle{}, &appstructure.Error{Code: "APPLICATION_SCHEMA_CONFLICT", Data: map[string]int64{"currentSchemaVersion": schema}}
	}
	definitions, err := captureDefinitions(facts.Fields)
	if err != nil {
		return ev.Bundle{}, err
	}
	headerColumns := []string{"r.id::text", "r.created_by::text", "r.record_version", "r.created_at", "r.updated_at"}
	ids := make([]string, len(definitions))
	values := make([][]byte, len(definitions))
	valueColumns := make([]string, len(definitions))
	lengthRows := make([]string, len(definitions))
	valuePorts := make([]any, len(definitions))
	var actualID, createdBy string
	var version int64
	var createdAt, updatedAt time.Time
	var lengths []int64
	for i, field := range definitions {
		valueSQL, e := appquery.StoredValueSQL(appquery.Field{ID: field.ID, Kind: appquery.FieldKind(field.Kind)})
		if e != nil {
			return ev.Bundle{}, fmt.Errorf("%w: %w", ErrUnavailable, e)
		}
		valueColumns[i] = "to_jsonb(" + valueSQL + ")"
		lengthSQL := "COALESCE(octet_length(" + valueColumns[i] + "::text),4)::bigint"
		if field.Kind == "text" || field.Kind == "multiline" {
			// An already oversized text needs no JSON serialization to reject it.
			lengthSQL = fmt.Sprintf("CASE WHEN octet_length(%s)>%d THEN %d::bigint ELSE %s END", valueSQL, ev.MaxFieldBytes, ev.MaxFieldBytes+1, lengthSQL)
		}
		lengthRows[i] = fmt.Sprintf("(%d,%s)", i, lengthSQL)
		ids[i] = field.ID
		valuePorts[i] = &values[i]
	}
	lengthArray := "ARRAY[]::bigint[]"
	if len(lengthRows) != 0 {
		lengthArray = "ARRAY(SELECT n FROM (VALUES " + strings.Join(lengthRows, ",") + ") AS budget(position,n) ORDER BY position)"
	}
	headerColumns = append(headerColumns, lengthArray)
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
	rowClause := " FROM " + relation + " r WHERE r.id=$1::uuid"
	args := []any{recordID}
	if policy.VisibleScope() == appaccess.Own {
		rowClause += " AND r.created_by=$2::uuid"
		args = append(args, facts.Actor.ID)
	}
	// The RR snapshot or caller's existing write locks span both queries.
	// Only small sizes cross the wire until every field and aggregate fit.
	if err = tx.QueryRow(ctx, "SELECT "+strings.Join(headerColumns, ",")+rowClause, args...).Scan(&actualID, &createdBy, &version, &createdAt, &updatedAt, &lengths); errors.Is(err, pgx.ErrNoRows) {
		return ev.Bundle{}, applications.ErrMissing
	} else if err != nil {
		return ev.Bundle{}, err
	}
	if len(lengths) != len(definitions) {
		return ev.Bundle{}, ErrUnavailable
	}
	budget := int64(ev.MaxBundleBytes - len(facts.Fields))
	for _, size := range lengths {
		if size < 0 {
			return ev.Bundle{}, ErrUnavailable
		}
		if size > ev.MaxFieldBytes || size > budget {
			return ev.Bundle{}, ev.ErrTooLarge
		}
		budget -= size
	}
	if len(valueColumns) != 0 {
		if err = tx.QueryRow(ctx, "SELECT "+strings.Join(valueColumns, ",")+rowClause, args...).Scan(valuePorts...); errors.Is(err, pgx.ErrNoRows) {
			return ev.Bundle{}, applications.ErrMissing
		} else if err != nil {
			return ev.Bundle{}, err
		}
	}
	remaining := ev.MaxBundleBytes - len(facts.Fields)
	for i, value := range values {
		size := len(value)
		if value == nil {
			size = 4
		}
		if int64(size) != lengths[i] {
			return ev.Bundle{}, ErrUnavailable
		}
		if size > ev.MaxFieldBytes || size > remaining {
			return ev.Bundle{}, ev.ErrTooLarge
		}
		remaining -= size
	}
	fields := make([]ev.Field, len(definitions))
	references := map[string]map[string]bool{"member": {}, "department": {}}
	selected := make(map[string][]string)
	var optionFields, optionIDs []string
	for i, definition := range definitions {
		value := values[i]
		if value == nil {
			value = []byte("null")
		}
		fields[i] = ev.Field{Definition: definition, Value: append([]byte(nil), value...)}
		if bytes.Equal(value, []byte("null")) {
			continue
		}
		switch definition.Kind {
		case "member", "department", "single_select", "multi_select":
			var selectedIDs []string
			if definition.Kind == "multi_select" {
				if json.Unmarshal(value, &selectedIDs) != nil {
					return ev.Bundle{}, ErrUnavailable
				}
			} else {
				var id string
				if json.Unmarshal(value, &id) != nil {
					return ev.Bundle{}, ErrUnavailable
				}
				selectedIDs = []string{id}
			}
			unique := make(map[string]bool)
			for _, id := range selectedIDs {
				if !appfields.ValidID(id) {
					return ev.Bundle{}, ErrUnavailable
				}
				if unique[id] {
					continue
				}
				unique[id] = true
				if definition.Kind == "member" || definition.Kind == "department" {
					references[definition.Kind][id] = true
				} else {
					selected[definition.ID] = append(selected[definition.ID], id)
					optionFields = append(optionFields, definition.ID)
					optionIDs = append(optionIDs, id)
				}
			}
		}
	}
	sources := make(map[string]map[string]ev.Reference)
	for _, kind := range []string{"member", "department"} {
		if len(references[kind]) == 0 {
			continue
		}
		sources[kind], err = captureReferences(ctx, tx, kind, references[kind])
		if err != nil {
			return ev.Bundle{}, err
		}
	}
	options := make(map[string]map[string]ev.Reference)
	if len(optionIDs) != 0 {
		rows, e := tx.Query(ctx, `SELECT t.field_id::text,t.option_id::text,t.label,t.removed
 FROM applications.field_option_tombstones t
 JOIN unnest($3::uuid[],$4::uuid[]) AS selected(field_id,option_id)
 ON selected.field_id=t.field_id AND selected.option_id=t.option_id
 WHERE t.app_id=$1 AND t.table_id=$2`, facts.App.ID, facts.TableID, optionFields, optionIDs)
		if e != nil {
			return ev.Bundle{}, e
		}
		for rows.Next() {
			var fieldID string
			var display ev.Reference
			if e = rows.Scan(&fieldID, &display.ID, &display.Label, &display.Deleted); e != nil {
				break
			}
			if options[fieldID] == nil {
				options[fieldID] = make(map[string]ev.Reference)
			}
			if _, exists := options[fieldID][display.ID]; exists {
				e = ErrUnavailable
				break
			}
			if len(display.Label) > remaining || len(display.Label) > ev.MaxFieldBytes {
				e = ev.ErrTooLarge
				break
			}
			remaining -= len(display.Label)
			options[fieldID][display.ID] = display
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return ev.Bundle{}, e
		}
	}
	for i := range fields {
		field := &fields[i]
		switch field.Definition.Kind {
		case "member", "department":
			if bytes.Equal(field.Value, []byte("null")) {
				continue
			}
			var id string
			if json.Unmarshal(field.Value, &id) != nil {
				return ev.Bundle{}, ErrUnavailable
			}
			display := sources[field.Definition.Kind][id]
			if len(display.Label) > remaining || len(display.Label) > ev.MaxFieldBytes {
				return ev.Bundle{}, ev.ErrTooLarge
			}
			remaining -= len(display.Label)
			field.Reference = &display
		case "single_select", "multi_select":
			for _, id := range selected[field.Definition.ID] {
				display, exists := options[field.Definition.ID][id]
				if !exists {
					return ev.Bundle{}, ErrUnavailable
				}
				field.OptionDisplays = append(field.OptionDisplays, display)
			}
		}
	}
	header := ev.Header{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, RecordID: actualID, CreatedBy: createdBy,
		SchemaVersion: schema, RecordVersion: version, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano), UpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano)}
	bundle, err := ev.Build(header, fields, policy.ReadFields(createdBy, ids))
	if err != nil {
		if errors.Is(err, ev.ErrTooLarge) {
			return ev.Bundle{}, err
		}
		return ev.Bundle{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return bundle, nil
}

func captureNilPort(port any) bool {
	if port == nil {
		return true
	}
	v := reflect.ValueOf(port)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func captureDefinitions(raw []byte) ([]appfields.Field, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if token, err := d.Token(); err != nil || token != json.Delim('[') {
		return nil, ErrUnavailable
	}
	var fields []appfields.Field
	seen := make(map[string]bool)
	for d.More() {
		if len(fields) == ev.MaxFields {
			return nil, ev.ErrTooLarge
		}
		var field appfields.Field
		if d.Decode(&field) != nil || !appfields.ValidID(field.ID) || seen[field.ID] {
			return nil, ErrUnavailable
		}
		seen[field.ID] = true
		fields = append(fields, field)
	}
	if token, err := d.Token(); err != nil || token != json.Delim(']') {
		return nil, ErrUnavailable
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, ErrUnavailable
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].ID < fields[j].ID })
	return fields, nil
}

func captureReferences(ctx context.Context, tx pgx.Tx, kind string, requested map[string]bool) (map[string]ev.Reference, error) {
	var relation string
	switch kind {
	case "member":
		relation = "applications.member_sources"
	case "department":
		relation = "applications.department_sources"
	default:
		return nil, ErrUnavailable
	}
	ids := make([]string, 0, len(requested))
	for id := range requested {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows, err := tx.Query(ctx, "SELECT id::text,label,status FROM "+relation+" WHERE id=ANY($1::uuid[])", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	displays := make(map[string]ev.Reference, len(ids))
	for rows.Next() {
		var display ev.Reference
		var status string
		if err = rows.Scan(&display.ID, &display.Label, &status); err != nil {
			return nil, err
		}
		if _, exists := displays[display.ID]; exists || !requested[display.ID] || strings.TrimSpace(display.Label) == "" ||
			(status != "active" && status != "deleted" && (kind != "member" || status != "disabled")) {
			return nil, ErrUnavailable
		}
		display.Deleted = status == "deleted"
		displays[display.ID] = display
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(displays) != len(requested) {
		return nil, ErrUnavailable
	}
	return displays, nil
}
