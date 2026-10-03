package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strconv"
	"strings"
)

func physicalFields(fs []appfields.Field) ([]appschema.Field, error) {
	out := make([]appschema.Field, 0, len(fs))
	for _, f := range fs {
		v := appschema.Field{ID: f.ID, Name: f.Name, Kind: f.Kind, Required: f.Required}
		switch f.Kind {
		case "text", "multiline":
			v.Type = appschema.Text
		case "number", "money":
			v.Type = appschema.Numeric
			var n appfields.DecimalConfig
			if json.Unmarshal(f.Config, &n) != nil {
				return nil, invalid()
			}
			v.Precision = n.Precision
			v.Scale = n.Scale
			v.RoundingPlaces = n.RoundingPlaces
			v.RoundingMode = n.RoundingMode
		case "date":
			v.Type = appschema.Date
		case "datetime":
			v.Type = appschema.Timestamp
			var config struct {
				Precision string `json:"precision"`
			}
			json.Unmarshal(f.Config, &config)
			v.TimePrecision = config.Precision
		case "boolean":
			v.Type = appschema.Boolean
		case "member", "department", "single_select":
			v.Type = appschema.UUID
		case "multi_select":
			v.Type = appschema.UUIDArray
		default:
			return nil, invalid()
		}
		if string(f.Default) != "null" {
			d := &appschema.Value{Type: v.Type}
			switch v.Type {
			case appschema.Boolean:
				json.Unmarshal(f.Default, &d.Boolean)
			case appschema.UUIDArray:
				json.Unmarshal(f.Default, &d.UUIDs)
			default:
				json.Unmarshal(f.Default, &d.Text)
			}
			v.Default = d
		}
		out = append(out, v)
	}
	return out, nil
}

type adapters struct {
	a      *Application
	p      session.Principal
	app    string
	before Definition
	input  Input
	plan   ChangePlan
}

func (m *adapters) Check(c context.Context, tx pgx.Tx, id string) error {
	if id != m.before.Table.ID || m.before.AppID != m.app {
		return applications.ErrResourceInvalid
	}
	var ok bool
	e := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM auth.users u JOIN applications.apps a ON a.id=$2 WHERE u.id=$1 AND u.status='active' AND u.auth_version::text=$3 AND (u.is_bootstrap_admin OR a.owner_user_id=u.id))", m.p.UserID, m.app, m.p.Record.AuthVersion).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return applications.ErrDenied
	}
	return nil
}
func (m *adapters) Lock(c context.Context, tx pgx.Tx, id string) (appschema.Snapshot, error) {
	t, fs, e := loadTable(c, tx, m.app, id, true)
	if e != nil {
		return appschema.Snapshot{}, e
	}
	physical, e := physicalFields(fs)
	return appschema.Snapshot{Exists: t.SchemaReady, Revision: strconv.FormatInt(t.SchemaVersion, 10), Fields: physical}, e
}
func (m *adapters) Store(c context.Context, tx pgx.Tx, id string, before appschema.Snapshot, fields []appschema.Field) (string, error) {
	changed := !jsonEqual(m.before.Fields, m.input.Fields) || !m.before.Table.SchemaReady
	layoutChanged := !jsonEqual(m.before.Layout, m.input.Layout) || m.before.Form.ViewVersion == 0
	schema, view := m.before.Table.SchemaVersion, m.before.Form.ViewVersion
	if changed {
		schema++
	}
	if layoutChanged {
		view++
	}
	for _, f := range m.input.Fields {
		raw, _ := json.Marshal(f)
		tag, e := tx.Exec(c, "INSERT INTO applications.fields(id,app_id,table_id,definition) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET definition=EXCLUDED.definition WHERE applications.fields.app_id=EXCLUDED.app_id AND applications.fields.table_id=EXCLUDED.table_id AND NOT applications.fields.removed", f.ID, m.app, id, raw)
		if e != nil {
			return "", e
		}
		if tag.RowsAffected() != 1 {
			return "", applications.ErrResourceInvalid
		}
	}
	ids := []string{}
	for _, f := range m.input.Fields {
		ids = append(ids, f.ID)
	}
	if _, e := tx.Exec(c, "UPDATE applications.fields SET removed=true WHERE app_id=$1 AND table_id=$2 AND NOT id=ANY($3::uuid[])", m.app, id, ids); e != nil {
		return "", e
	}
	raw, _ := json.Marshal(m.input.Fields)
	if _, e := tx.Exec(c, "UPDATE applications.logical_tables SET fields_json=$3,schema_version=$4,schema_ready=true WHERE app_id=$1 AND id=$2", m.app, id, raw, schema); e != nil {
		return "", e
	}
	layout, _ := json.Marshal(m.input.Layout)
	if _, e := tx.Exec(c, "UPDATE applications.form_views SET layout=$3,view_version=$4 WHERE app_id=$1 AND id=$2", m.app, m.before.Form.ID, layout, view); e != nil {
		return "", e
	}
	var version int64
	if e := tx.QueryRow(c, "SELECT structure_version FROM applications.apps WHERE id=$1", m.app).Scan(&version); e != nil {
		return "", e
	}
	if e := audit(c, tx, m.p, m.app, m.input.OperationID, "definition.save", m.before.Form.ID, "form", version, schema, view, len(m.plan.SchemaChanges)); e != nil {
		return "", e
	}
	return strconv.FormatInt(schema, 10), nil
}
func (m *adapters) Create(c context.Context, tx pgx.Tx, p appschema.Plan) error {
	if _, e := tx.Exec(c, "SELECT applications.apply_schema_change($1,$2,$3,'create_table',NULL,NULL)", m.p.UserID, m.app, p.TableID); e != nil {
		return e
	}
	for _, f := range p.Fields {
		a := f
		if e := m.Change(c, tx, p.TableID, appschema.Change{Operation: appschema.AddColumn, After: &a}, nil); e != nil {
			return e
		}
	}
	return nil
}
func (m *adapters) LockPhysical(c context.Context, tx pgx.Tx, id string) error {
	_, e := tx.Exec(c, "SELECT applications.apply_schema_change($1,$2,$3,'lock_table',NULL,NULL)", m.p.UserID, m.app, id)
	return e
}
func (m *adapters) Change(c context.Context, tx pgx.Tx, id string, change appschema.Change, backfills map[string]appschema.Value) error {
	b, a := []byte("null"), []byte("null")
	if change.Before != nil {
		b, _ = json.Marshal(change.Before)
	}
	if change.After != nil {
		a, _ = json.Marshal(change.After)
	}
	_, e := tx.Exec(c, "SELECT applications.apply_schema_change($1,$2,$3,$4,$5,$6)", m.p.UserID, m.app, id, string(change.Operation), b, a)
	return e
}
func (m *adapters) Protect(c context.Context, tx pgx.Tx, id string, fields []string) ([]appschema.Reference, error) {
	removed := []string{}
	for _, change := range m.plan.SchemaChanges {
		if change.Kind == "remove" {
			removed = append(removed, change.FieldID)
		}
	}
	deps, e := m.a.dependencies(c, tx, m.before, fields, removed)
	if e != nil {
		return nil, e
	}
	out := make([]appschema.Reference, len(deps))
	for i, d := range deps {
		out[i] = appschema.Reference{FieldID: d.FieldID, Kind: d.Kind, ID: d.ResourceID}
	}
	return out, nil
}
func (m *adapters) Verify(c context.Context, tx pgx.Tx, id string, impacts []appschema.ColumnImpact) error {
	return m.a.verifyConfirmation(c, tx, m.p, m.before, m.input)
}
func (a *Application) dependencies(c context.Context, tx pgx.Tx, d Definition, ids, removed []string) ([]Dependency, error) {
	if a.Dependencies == nil {
		return nil, ErrUnavailable
	}
	if e := a.Dependencies.Available(c, tx, d.Table.ID); e != nil {
		return nil, e
	}
	out := []Dependency{}
	rows, e := tx.Query(c, "SELECT field_id::text,kind,resource_id::text FROM applications.table_field_dependencies WHERE app_id=$1 AND table_id=$2 AND field_id=ANY($3::uuid[]) ORDER BY field_id,kind,resource_id", d.AppID, d.Table.ID, ids)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var v Dependency
		if e = rows.Scan(&v.FieldID, &v.Kind, &v.ResourceID); e != nil {
			break
		}
		out = append(out, v)
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(removed) > 0 {
		rows, e = tx.Query(c, `SELECT gf.field_id::text,'data_grant',g.id::text FROM applications.grant_fields gf
 JOIN applications.grants g ON g.app_id=gf.app_id AND g.id=gf.grant_id
 WHERE gf.app_id=$1 AND gf.table_id=$2 AND gf.field_id=ANY($3::uuid[]) ORDER BY gf.field_id,g.id`, d.AppID, d.Table.ID, removed)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var v Dependency
			if e = rows.Scan(&v.FieldID, &v.Kind, &v.ResourceID); e != nil {
				break
			}
			out = append(out, v)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	rows, e = tx.Query(c, "SELECT id::text,layout FROM applications.form_views WHERE app_id=$1 AND table_id=$2 AND id<>$3 ORDER BY id", d.AppID, d.Table.ID, d.Form.ID)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id string
		var raw []byte
		if e = rows.Scan(&id, &raw); e != nil {
			break
		}
		var ns []appfields.LayoutNode
		if e = json.Unmarshal(raw, &ns); e != nil {
			break
		}
		var visit func([]appfields.LayoutNode)
		visit = func(nodes []appfields.LayoutNode) {
			for _, n := range nodes {
				for _, f := range removed {
					if n.Kind == "field" && n.FieldID == f {
						out = append(out, Dependency{f, "view_layout", id})
					}
				}
				visit(n.Children)
			}
		}
		visit(ns)
	}
	rows.Close()
	return out, e
}
func planFor(d Definition, in Input) ChangePlan {
	p := ChangePlan{SchemaChanges: []SchemaChange{}, MetadataChanged: !jsonEqual(d.Fields, in.Fields), LayoutChanged: !jsonEqual(d.Layout, in.Layout)}
	old := map[string]appfields.Field{}
	for _, f := range d.Fields {
		old[f.ID] = f
	}
	for _, f := range in.Fields {
		previous, ok := old[f.ID]
		kind := ""
		if !ok {
			kind = "add"
		} else if previous.Kind != f.Kind {
			kind = "change_type"
		} else if !jsonEqual(previous.Config, f.Config) {
			kind = "change_config"
		} else if !jsonEqual(previous.Default, f.Default) {
			kind = "change_default"
		} else if previous.Required != f.Required {
			kind = "change_required"
		}
		if kind != "" {
			k := f.Kind
			var prev *string
			if ok {
				v := previous.Kind
				prev = &v
			}
			p.SchemaChanges = append(p.SchemaChanges, SchemaChange{kind, f.ID, prev, &k})
		}
		delete(old, f.ID)
	}
	for _, f := range d.Fields {
		if _, ok := old[f.ID]; ok {
			k := f.Kind
			p.SchemaChanges = append(p.SchemaChanges, SchemaChange{"remove", f.ID, &k, nil})
		}
	}
	return p
}
func (a *Application) save(c context.Context, tx pgx.Tx, p session.Principal, app, id string, in Input) (applications.Result, error) {
	d, e := definition(c, tx, app, id, true)
	if e != nil {
		return applications.Result{}, e
	}
	if d.Table.SchemaVersion != in.ExpectedSchemaVersion {
		return applications.Result{}, &Error{"APPLICATION_SCHEMA_CONFLICT", map[string]any{"currentSchemaVersion": d.Table.SchemaVersion}}
	}
	if d.Form.ViewVersion != in.ExpectedViewVersion {
		return applications.Result{}, &Error{"APPLICATION_VIEW_CONFLICT", map[string]any{"currentViewVersion": d.Form.ViewVersion}}
	}
	if a.Limits.LockTimeout <= 0 || a.Limits.StatementTimeout <= 0 {
		return applications.Result{}, ErrUnavailable
	}
	if d.Table.SchemaReady {
		if _, e = tx.Exec(c, "SELECT applications.apply_schema_change($1,$2,$3,'lock_table',NULL,NULL)", p.UserID, app, d.Table.ID); e != nil {
			return applications.Result{}, e
		}
	}
	pre, e := a.inspect(c, tx, p, d, in, false)
	if e != nil {
		return applications.Result{}, e
	}
	if len(pre.Dependencies) > 0 {
		return applications.Result{}, &Error{"APPLICATION_SCHEMA_DEPENDENCY_BLOCKED", map[string]any{"dependencies": pre.Dependencies}}
	}
	if len(pre.BlockingIssues) > 0 {
		issue := pre.BlockingIssues[0]
		return applications.Result{}, &Error{issue.Code, map[string]any{"fieldIds": issue.FieldIDs}}
	}
	if len(pre.Impacts) > 0 {
		if in.ConfirmationToken == nil {
			return applications.Result{}, &Error{"APPLICATION_SCHEMA_CONFIRMATION_REQUIRED", map[string]any{"impacts": pre.Impacts}}
		}
		if e = a.verifyConfirmation(c, tx, p, d, in); e != nil {
			return applications.Result{}, e
		}
	}
	fields, e := physicalFields(in.Fields)
	if e != nil {
		return applications.Result{}, e
	}
	for _, old := range d.Fields {
		if old.Kind != "single_select" && old.Kind != "multi_select" {
			continue
		}
		var target *appfields.Field
		for i := range in.Fields {
			if in.Fields[i].ID == old.ID {
				target = &in.Fields[i]
				break
			}
		}
		if target == nil || target.Kind != "single_select" && target.Kind != "multi_select" {
			continue
		}
		mappings := []OptionMapping{}
		for _, mapping := range in.OptionMappings {
			if mapping.FieldID == old.ID {
				mappings = append(mappings, mapping)
			}
		}
		if len(mappings) > 0 {
			raw, _ := json.Marshal(mappings)
			if _, e = tx.Exec(c, "SELECT applications.apply_option_mapping($1,$2,$3,$4,$5,$6,$7)", p.UserID, app, d.Table.ID, old.ID, old.Kind, target.Config, raw); e != nil {
				return applications.Result{}, e
			}
		}
	}
	m := &adapters{a, p, app, d, in, pre.Plan}
	executor := appschema.Executor{Namespace: "appdata", Guard: m, Metadata: m, Dependencies: m, DDL: m, Limits: a.Limits}
	_, e = executor.ApplyInTx(c, tx, appschema.Request{TableID: d.Table.ID, ExpectedRevision: strconv.FormatInt(d.Table.SchemaVersion, 10), Fields: fields, Confirmation: m})
	if e != nil {
		var blocked *appschema.DependencyError
		if errors.As(e, &blocked) {
			deps := []Dependency{}
			for _, r := range blocked.References {
				deps = append(deps, Dependency{r.FieldID, r.Kind, r.ID})
			}
			return applications.Result{}, &Error{"APPLICATION_SCHEMA_DEPENDENCY_BLOCKED", map[string]any{"dependencies": deps}}
		}
		if errors.Is(e, appschema.ErrRequiredBackfill) {
			return applications.Result{}, &Error{"APPLICATION_SCHEMA_REQUIRED_BACKFILL", map[string]any{"fieldIds": []string{}}}
		}
		var pg *pgconn.PgError
		if errors.As(e, &pg) && (strings.HasPrefix(pg.Code, "22") || pg.Code == "23502") {
			ids := []string{}
			for _, change := range pre.Plan.SchemaChanges {
				ids = append(ids, change.FieldID)
			}
			return applications.Result{}, &Error{"APPLICATION_SCHEMA_CONVERSION_FAILED", map[string]any{"fieldIds": ids}}
		}
		return applications.Result{}, e
	}
	after, e := definition(c, tx, app, id, false)
	if e != nil {
		return applications.Result{}, e
	}
	raw, _ := json.Marshal(map[string]any{"operationId": in.OperationID, "definition": after, "appliedPlan": pre.Plan})
	return applications.Result{Status: 200, Data: raw}, nil
}
