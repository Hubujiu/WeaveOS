package appstructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appmeta"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxVersion = int64(9007199254740991)

func invalid() error {
	return &Error{Code: "COMMON_VALIDATION_FAILED", Data: map[string]any{"violations": []any{map[string]string{"location": "body", "field": "definition", "code": "VALIDATION_INVALID", "message": "定义或配置不合法"}}}}
}
func missing(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return applications.ErrMissing
	}
	return e
}
func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	var left, right any
	d := json.NewDecoder(bytes.NewReader(x))
	d.UseNumber()
	if d.Decode(&left) != nil {
		return false
	}
	d = json.NewDecoder(bytes.NewReader(y))
	d.UseNumber()
	if d.Decode(&right) != nil {
		return false
	}
	x, _ = json.Marshal(left)
	y, _ = json.Marshal(right)
	return bytes.Equal(x, y)
}
func newID(c context.Context, tx pgx.Tx) (string, error) {
	var id string
	e := tx.QueryRow(c, "SELECT gen_random_uuid()::text").Scan(&id)
	return id, e
}
func normalize(kind string, in Input) (Input, error) {
	if in.OperationID != "" && !appfields.ValidID(in.OperationID) {
		return in, invalid()
	}
	if in.ExpectedSchemaVersion < 0 || in.ExpectedSchemaVersion > maxVersion || in.ExpectedViewVersion < 0 || in.ExpectedViewVersion > maxVersion || in.ExpectedStructureVersion < 0 || in.ExpectedStructureVersion > maxVersion {
		return in, invalid()
	}
	if strings.HasPrefix(kind, "definition.") {
		var e error
		in.Fields, e = appfields.NormalizeFields(in.Fields)
		if e != nil {
			return in, invalid()
		}
		in.Layout, e = appfields.NormalizeLayout(in.Layout, in.Fields)
		if e != nil || in.OptionMappings == nil {
			return in, invalid()
		}
		seen := map[string]bool{}
		for _, m := range in.OptionMappings {
			key := m.FieldID + "/" + m.FromOptionID
			if !appfields.ValidID(m.FieldID) || !appfields.ValidID(m.FromOptionID) || m.ToOptionID != nil && !appfields.ValidID(*m.ToOptionID) || seen[key] {
				return in, invalid()
			}
			seen[key] = true
		}
		sort.Slice(in.OptionMappings, func(i, j int) bool {
			a, b := in.OptionMappings[i], in.OptionMappings[j]
			return a.FieldID < b.FieldID || a.FieldID == b.FieldID && a.FromOptionID < b.FromOptionID
		})
		return in, nil
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 || strings.ContainsRune(in.Name, 0) || in.Position < 0 || int64(in.Position) > 2147483647 {
		return in, invalid()
	}
	for _, id := range []*string{in.ParentID, in.DirectoryID} {
		if id != nil && !appfields.ValidID(*id) {
			return in, invalid()
		}
	}
	if kind == "form.create" {
		if in.Source == nil || in.Source.Kind != "new_table" && in.Source.Kind != "existing_table" || in.Source.Kind == "new_table" && in.Source.TableID != "" || in.Source.Kind == "existing_table" && !appfields.ValidID(in.Source.TableID) {
			return in, invalid()
		}
	}
	return in, nil
}
func (a *Application) read(c context.Context, p session.Principal, app string) (pgx.Tx, error) {
	if a == nil || a.Pool == nil {
		return nil, ErrUnavailable
	}
	tx, e := a.Pool.BeginTx(c, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return nil, e
	}
	var owner string
	var root bool
	e = tx.QueryRow(c, "SELECT a.owner_user_id::text,u.is_bootstrap_admin FROM applications.apps a JOIN auth.users u ON u.id=$2 AND u.status='active' AND u.auth_version::text=$3 JOIN personnel.permission_catalog pc ON pc.code='app.'||a.id::text||'.access' AND pc.enabled WHERE a.id=$1", app, p.UserID, p.Record.AuthVersion).Scan(&owner, &root)
	if e == nil && owner != p.UserID && !root {
		e = applications.ErrDenied
	}
	if e != nil {
		tx.Rollback(context.Background())
		return nil, missing(e)
	}
	return tx, nil
}
func loadTable(c context.Context, tx pgx.Tx, app, id string, lock bool) (Table, []appfields.Field, error) {
	q := "SELECT id::text,app_id::text,name,directory_id::text,position,schema_version,schema_ready,fields_json FROM applications.logical_tables WHERE app_id=$1 AND id=$2"
	if lock {
		q += " FOR UPDATE"
	}
	var t Table
	var raw []byte
	e := tx.QueryRow(c, q, app, id).Scan(&t.ID, &t.AppID, &t.Name, &t.DirectoryID, &t.Position, &t.SchemaVersion, &t.SchemaReady, &raw)
	var f []appfields.Field
	if e == nil {
		e = json.Unmarshal(raw, &f)
	}
	return t, f, missing(e)
}
func loadForm(c context.Context, tx pgx.Tx, app, id string, lock bool) (Form, []appfields.LayoutNode, error) {
	q := "SELECT id::text,app_id::text,table_id::text,name,directory_id::text,position,view_version,layout FROM applications.form_views WHERE app_id=$1 AND id=$2"
	if lock {
		q += " FOR UPDATE"
	}
	var v Form
	var raw []byte
	e := tx.QueryRow(c, q, app, id).Scan(&v.ID, &v.AppID, &v.TableID, &v.Name, &v.DirectoryID, &v.Position, &v.ViewVersion, &raw)
	var l []appfields.LayoutNode
	if e == nil {
		e = json.Unmarshal(raw, &l)
	}
	return v, l, missing(e)
}
func definition(c context.Context, tx pgx.Tx, app, id string, lock bool) (Definition, error) {
	v, l, e := loadForm(c, tx, app, id, false)
	if e != nil {
		return Definition{}, e
	}
	t, f, e := loadTable(c, tx, app, v.TableID, lock)
	if e != nil {
		return Definition{}, e
	}
	if lock {
		if _, e = tx.Exec(c, "SELECT id FROM applications.form_views WHERE app_id=$1 AND table_id=$2 ORDER BY id FOR UPDATE", app, v.TableID); e != nil {
			return Definition{}, e
		}
		v, l, e = loadForm(c, tx, app, id, true)
		if e != nil {
			return Definition{}, e
		}
	}
	var exists bool
	e = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM applications.menu_resources WHERE app_id=$1 AND resource_kind='form' AND resource_id=$2)", app, id).Scan(&exists)
	if e != nil {
		return Definition{}, e
	}
	if !exists {
		return Definition{}, applications.ErrResourceInvalid
	}
	sys := []SystemField{{"id", "uuid", true}, {"createdBy", "member", true}, {"createdAt", "datetime", true}, {"updatedAt", "datetime", true}, {"recordVersion", "number", true}}
	return Definition{app, t, v, f, sys, l, Capabilities{true}}, nil
}
func structure(c context.Context, tx pgx.Tx, app string) (Structure, error) {
	s := Structure{AppID: app, Directories: []Directory{}, Tables: []Table{}, Forms: []Form{}, Capabilities: Capabilities{true}}
	e := tx.QueryRow(c, "SELECT structure_version FROM applications.apps WHERE id=$1", app).Scan(&s.StructureVersion)
	if e != nil {
		return s, e
	}
	rows, e := tx.Query(c, "SELECT id::text,name,parent_id::text,position FROM applications.directories WHERE app_id=$1 ORDER BY position,id", app)
	if e != nil {
		return s, e
	}
	for rows.Next() {
		d := Directory{AppID: app}
		e = rows.Scan(&d.ID, &d.Name, &d.ParentID, &d.Position)
		if e != nil {
			break
		}
		s.Directories = append(s.Directories, d)
	}
	rows.Close()
	if e != nil {
		return s, e
	}
	rows, e = tx.Query(c, "SELECT id::text,name,directory_id::text,position,schema_version,schema_ready FROM applications.logical_tables WHERE app_id=$1 ORDER BY position,id", app)
	if e != nil {
		return s, e
	}
	for rows.Next() {
		v := Table{AppID: app}
		e = rows.Scan(&v.ID, &v.Name, &v.DirectoryID, &v.Position, &v.SchemaVersion, &v.SchemaReady)
		if e != nil {
			break
		}
		s.Tables = append(s.Tables, v)
	}
	rows.Close()
	if e != nil {
		return s, e
	}
	rows, e = tx.Query(c, "SELECT id::text,table_id::text,name,directory_id::text,position,view_version FROM applications.form_views WHERE app_id=$1 ORDER BY position,id", app)
	if e != nil {
		return s, e
	}
	for rows.Next() {
		v := Form{AppID: app}
		e = rows.Scan(&v.ID, &v.TableID, &v.Name, &v.DirectoryID, &v.Position, &v.ViewVersion)
		if e != nil {
			break
		}
		s.Forms = append(s.Forms, v)
	}
	rows.Close()
	return s, e
}
func checkDirectory(c context.Context, tx pgx.Tx, app string, id *string) error {
	if id == nil {
		return nil
	}
	var yes bool
	e := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM applications.directories WHERE app_id=$1 AND id=$2)", app, *id).Scan(&yes)
	if e != nil {
		return e
	}
	if !yes {
		return applications.ErrResourceInvalid
	}
	return nil
}
func checkTree(c context.Context, tx pgx.Tx, app, id string, in Input) error {
	s, e := structure(c, tx, app)
	if e != nil {
		return e
	}
	m := appmeta.Structure{Application: appmeta.Application{ID: appmeta.ID(app)}}
	for _, d := range s.Directories {
		if d.ID == id {
			continue
		}
		parent := ""
		if d.ParentID != nil {
			parent = *d.ParentID
		}
		m.Groups = append(m.Groups, appmeta.Group{ID: appmeta.ID(d.ID), ApplicationID: appmeta.ID(app), ParentID: appmeta.ID(parent)})
	}
	parent := ""
	if in.ParentID != nil {
		parent = *in.ParentID
	}
	m.Groups = append(m.Groups, appmeta.Group{ID: appmeta.ID(id), ApplicationID: appmeta.ID(app), ParentID: appmeta.ID(parent)})
	if appmeta.ValidateStructure(m) != nil {
		return invalid()
	}
	return nil
}
func audit(c context.Context, tx pgx.Tx, p session.Principal, app, op, kind, id, object string, version, schema, view int64, count int) error {
	summary, _ := json.Marshal(map[string]any{"appId": app, "operationId": op, "structureVersion": version, "schemaVersion": schema, "viewVersion": view, "changeCount": count})
	_, e := tx.Exec(c, "INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,session_ref,reason_code,request_id,object_type,object_id,change_summary) VALUES('application_structure_changed','success',$1,NULLIF($2,'')::uuid,$3,'definition',$4,$5,$6)", p.UserID, p.SessionRef, strings.ToUpper(strings.ReplaceAll(kind, ".", "_")), object, id, summary)
	return e
}
func (a *Application) write(c context.Context, p session.Principal, app, id, kind string, in Input) (applications.Result, error) {
	in, e := normalize(kind, in)
	if e != nil {
		return applications.Result{}, e
	}
	if !appfields.ValidID(in.OperationID) {
		return applications.Result{}, invalid()
	}
	fingerprintRaw, _ := json.Marshal(struct {
		App, ID, Kind string
		Input         Input
	}{app, id, kind, in})
	hash := sha256.Sum256(fingerprintRaw)
	if a == nil || a.Pool == nil || a.Limits.LockTimeout <= 0 || a.Limits.StatementTimeout <= 0 {
		return applications.Result{}, ErrUnavailable
	}
	guard := func(c context.Context, tx pgx.Tx) error {
		var replay bool
		if e := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2)", p.UserID, in.OperationID).Scan(&replay); e != nil {
			return e
		}
		if replay {
			return nil
		}
		return a.references(c, tx, app, id, in)
	}
	w, e := (&applications.Application{Pool: a.Pool}).BeginManagerWrite(c, p, app, applications.WriteOptions{LockTimeout: a.Limits.LockTimeout, StatementTimeout: a.Limits.StatementTimeout, SourceGuard: guard})
	if e != nil {
		return applications.Result{}, e
	}
	defer w.Rollback(context.Background())
	if old, e := w.Replay(c, in.OperationID, kind, hash); e != nil {
		return applications.Result{}, e
	} else if old != nil {
		if e = w.Commit(c); e != nil {
			return applications.Result{}, e
		}
		return *old, nil
	}
	if e = w.Claim(c, in.OperationID, kind, hash); e != nil {
		return applications.Result{}, e
	}
	tx := w.Tx()
	if kind == "definition.save" {
		result, e := a.save(c, tx, p, app, id, in)
		if e != nil {
			return result, e
		}
		if e = w.Complete(c, in.OperationID, result); e != nil {
			return result, e
		}
		if e = w.Commit(c); e != nil {
			return result, e
		}
		return result, nil
	}
	var version int64
	if e = tx.QueryRow(c, "SELECT structure_version FROM applications.apps WHERE id=$1", app).Scan(&version); e != nil {
		return applications.Result{}, e
	}
	if version != in.ExpectedStructureVersion {
		return applications.Result{}, &Error{"APPLICATION_STRUCTURE_CONFLICT", map[string]any{"currentStructureVersion": version}}
	}
	status := 200
	location := ""
	object := strings.Split(kind, ".")[0]
	if strings.HasSuffix(kind, ".create") {
		id, e = newID(c, tx)
		if e != nil {
			return applications.Result{}, e
		}
		status = 201
		location = "/api/v1/applications/" + app + "/" + map[string]string{"directory": "directories", "table": "tables", "form": "forms"}[object] + "/" + id
	}
	if e = checkDirectory(c, tx, app, in.DirectoryID); e != nil {
		return applications.Result{}, e
	}
	if e = checkDirectory(c, tx, app, in.ParentID); e != nil {
		return applications.Result{}, e
	}
	var value any
	switch kind {
	case "directory.create", "directory.update":
		if e = checkTree(c, tx, app, id, in); e != nil {
			return applications.Result{}, e
		}
		if status == 201 {
			_, e = tx.Exec(c, "INSERT INTO applications.directories(id,app_id,name,parent_id,position) VALUES($1,$2,$3,$4,$5)", id, app, in.Name, in.ParentID, in.Position)
		} else {
			var tag interface{ RowsAffected() int64 }
			tag, e = tx.Exec(c, "UPDATE applications.directories SET name=$3,parent_id=$4,position=$5 WHERE id=$1 AND app_id=$2", id, app, in.Name, in.ParentID, in.Position)
			if e == nil && tag.RowsAffected() != 1 {
				e = applications.ErrMissing
			}
		}
		if e == nil && status == 201 {
			_, e = tx.Exec(c, "INSERT INTO applications.menu_resources VALUES($1,'directory',$2)", app, id)
		}
		value = map[string]any{"directory": Directory{id, app, in.Name, in.ParentID, in.Position}, "structureVersion": version + 1}
	case "table.create", "table.update":
		if status == 201 {
			_, e = tx.Exec(c, "INSERT INTO applications.logical_tables(id,app_id,name,directory_id,position) VALUES($1,$2,$3,$4,$5)", id, app, in.Name, in.DirectoryID, in.Position)
		} else {
			tag, err := tx.Exec(c, "UPDATE applications.logical_tables SET name=$3,directory_id=$4,position=$5 WHERE id=$1 AND app_id=$2", id, app, in.Name, in.DirectoryID, in.Position)
			e = err
			if e == nil && tag.RowsAffected() != 1 {
				e = applications.ErrMissing
			}
		}
		if e == nil {
			t, _, err := loadTable(c, tx, app, id, false)
			e = err
			value = map[string]any{"table": t, "structureVersion": version + 1}
		}
	case "form.create", "form.update":
		tableID := ""
		if status == 201 {
			if in.Source.Kind == "new_table" {
				tableID, e = newID(c, tx)
				if e == nil {
					_, e = tx.Exec(c, "INSERT INTO applications.logical_tables(id,app_id,name,directory_id,position) VALUES($1,$2,$3,$4,$5)", tableID, app, in.Name, in.DirectoryID, in.Position)
				}
			} else {
				tableID = in.Source.TableID
				_, _, e = loadTable(c, tx, app, tableID, false)
			}
			if e == nil {
				_, e = tx.Exec(c, "INSERT INTO applications.form_views(id,app_id,table_id,name,directory_id,position) VALUES($1,$2,$3,$4,$5,$6)", id, app, tableID, in.Name, in.DirectoryID, in.Position)
			}
			if e == nil {
				_, e = tx.Exec(c, "INSERT INTO applications.menu_resources VALUES($1,'form',$2)", app, id)
			}
		} else {
			v, _, err := loadForm(c, tx, app, id, false)
			e = err
			tableID = v.TableID
			if e == nil {
				_, e = tx.Exec(c, "UPDATE applications.form_views SET name=$3,directory_id=$4,position=$5 WHERE id=$1 AND app_id=$2", id, app, in.Name, in.DirectoryID, in.Position)
			}
		}
		if e == nil {
			v, _, err := loadForm(c, tx, app, id, false)
			e = err
			t, _, err := loadTable(c, tx, app, tableID, false)
			if e == nil {
				e = err
			}
			value = map[string]any{"form": v, "table": t, "structureVersion": version + 1}
		}
	default:
		return applications.Result{}, invalid()
	}
	if e != nil {
		return applications.Result{}, missing(e)
	}
	if _, e = tx.Exec(c, "UPDATE applications.apps SET structure_version=structure_version+1 WHERE id=$1", app); e != nil {
		return applications.Result{}, e
	}
	if e = audit(c, tx, p, app, in.OperationID, kind, id, object, version+1, 0, 0, 1); e != nil {
		return applications.Result{}, e
	}
	raw, _ := json.Marshal(value)
	result := applications.Result{Status: status, Location: location, Data: raw}
	if e = w.Complete(c, in.OperationID, result); e != nil {
		return result, e
	}
	if e = w.Commit(c); e != nil {
		return result, e
	}
	return result, nil
}
func tablePhysical(id string) string {
	return pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
}
func columnPhysical(id string) string {
	return pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
}
func schemaError(code string, field string) error {
	return &Error{code, map[string]any{"fieldIds": []string{field}}}
}
func describeError(e error) string { return fmt.Sprintf("%T", e) }
