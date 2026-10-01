package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"reflect"
	"unicode/utf8"
)

type DefinitionKind string

const Identity DefinitionKind = "identity"
const Template DefinitionKind = "template"

type DefinitionInput struct {
	Name, Description            string
	Version                      int64
	PermissionCodes, TemplateIDs []string
}
type RequestMetadata struct {
	RequestID, ClientIP, UserAgent string
	QueryWriteGuard
}
type PageQuery struct {
	Page, PageSize int
	Search         string
}
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func definitionTable(kind DefinitionKind) (string, error) {
	switch kind {
	case Identity:
		return "personnel.identities", nil
	case Template:
		return "personnel.permission_templates", nil
	}
	return "", ErrInvalid
}
func definitionSelect(kind DefinitionKind) string {
	base := `jsonb_build_object('id',d.id,'name',d.name,'description',d.description,'version',d.version,`
	if kind == Identity {
		return base + `
 'permissionCodes',COALESCE((SELECT jsonb_agg(permission_code ORDER BY permission_code) FROM personnel.identity_permissions WHERE identity_id=d.id),'[]'::jsonb),
 'templateIds',COALESCE((SELECT jsonb_agg(template_id ORDER BY template_id) FROM personnel.identity_templates WHERE identity_id=d.id),'[]'::jsonb),
 'affectedMembers',(SELECT count(*) FROM personnel.member_identities WHERE identity_id=d.id),'affectedIdentities',0)`
	}
	return base + `
 'permissionCodes',COALESCE((SELECT jsonb_agg(permission_code ORDER BY permission_code) FROM personnel.template_permissions WHERE template_id=d.id),'[]'::jsonb),
 'templateIds','[]'::jsonb,
 'affectedMembers',(SELECT count(DISTINCT m.user_id) FROM personnel.member_identities m JOIN personnel.identity_templates it ON it.identity_id=m.identity_id WHERE it.template_id=d.id),
 'affectedIdentities',(SELECT count(*) FROM personnel.identity_templates WHERE template_id=d.id))`
}
func getDefinition(ctx context.Context, tx pgx.Tx, kind DefinitionKind, id string) (Definition, error) {
	table, err := definitionTable(kind)
	if err != nil {
		return Definition{}, err
	}
	var raw []byte
	if err := tx.QueryRow(ctx, "SELECT "+definitionSelect(kind)+" FROM "+table+" d WHERE id=$1", id).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Definition{}, ErrMissing
		}
		return Definition{}, err
	}
	var result Definition
	if err := json.Unmarshal(raw, &result); err != nil {
		return Definition{}, err
	}
	return result, nil
}
func (a *Application) GetDefinition(ctx context.Context, p session.Principal, kind DefinitionKind, id string) (Definition, error) {
	if !validID(id) {
		return Definition{}, ErrInvalid
	}
	tx, err := a.read(ctx, p)
	if err != nil {
		return Definition{}, err
	}
	defer tx.Rollback(context.Background())
	return getDefinition(ctx, tx, kind, id)
}
func (a *Application) ListDefinitions(ctx context.Context, p session.Principal, kind DefinitionKind, query PageQuery, queryVersion ...string) (Page[Definition], error) {
	query, err := normalizedPage(query)
	if err != nil {
		return Page[Definition]{}, err
	}
	table, err := definitionTable(kind)
	if err != nil {
		return Page[Definition]{}, err
	}
	result := Page[Definition]{Items: []Definition{}, Page: query.Page, PageSize: query.PageSize}
	ctx, cancel := context.WithTimeout(ctx, queryDeadline)
	defer cancel()
	version := ""
	if len(queryVersion) > 0 {
		version = queryVersion[0]
	}
	tx, receipt, err := a.readWithQuery(ctx, p, version)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	filter := " WHERE ($1='' OR position(lower($1) in lower(name||' '||description))>0)"
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table+filter, query.Search).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, "SELECT "+definitionSelect(kind)+" FROM "+table+" d"+filter+" ORDER BY created_at,id LIMIT $2 OFFSET $3", query.Search, query.PageSize, (query.Page-1)*query.PageSize)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return result, err
		}
		var value Definition
		if err := json.Unmarshal(raw, &value); err != nil {
			return result, err
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	rows.Close()
	return result, receipt.commit(ctx, tx)
}

type safeDefinition struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	PermissionCodes []string `json:"permissionCodes"`
	TemplateIDs     []string `json:"templateIds"`
}

func safeDefinitionValue(value Definition) safeDefinition {
	return safeDefinition{value.Name, value.Description, value.PermissionCodes, value.TemplateIDs}
}
func validateDefinition(ctx context.Context, tx pgx.Tx, kind DefinitionKind, in DefinitionInput) error {
	if !validName(in.Name) || utf8.RuneCountInString(in.Description) > 1000 || kind == Template && len(in.TemplateIDs) > 0 {
		return ErrInvalid
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.permission_catalog WHERE code=ANY($1::varchar[]) AND enabled", in.PermissionCodes).Scan(&count); err != nil {
		return err
	}
	if count != len(in.PermissionCodes) {
		return ErrInvalid
	}
	for _, id := range in.TemplateIDs {
		if !validID(id) {
			return ErrInvalid
		}
	}
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.permission_templates WHERE id=ANY($1::uuid[])", in.TemplateIDs).Scan(&count); err != nil {
		return err
	}
	if count != len(in.TemplateIDs) {
		return ErrInvalid
	}
	return nil
}
func replaceDefinitionRelations(ctx context.Context, tx pgx.Tx, kind DefinitionKind, id string, in DefinitionInput) error {
	relation, column := "personnel.identity_permissions", "identity_id"
	if kind == Template {
		relation, column = "personnel.template_permissions", "template_id"
	}
	if _, err := tx.Exec(ctx, "DELETE FROM "+relation+" WHERE "+column+"=$1", id); err != nil {
		return err
	}
	for _, code := range in.PermissionCodes {
		if _, err := tx.Exec(ctx, "INSERT INTO "+relation+"("+column+",permission_code) VALUES($1,$2)", id, code); err != nil {
			return err
		}
	}
	if kind == Identity {
		if _, err := tx.Exec(ctx, "DELETE FROM personnel.identity_templates WHERE identity_id=$1", id); err != nil {
			return err
		}
		for _, template := range in.TemplateIDs {
			if _, err := tx.Exec(ctx, "INSERT INTO personnel.identity_templates(identity_id,template_id) VALUES($1,$2)", id, template); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *Application) SaveDefinition(ctx context.Context, p session.Principal, kind DefinitionKind, id string, in DefinitionInput, meta RequestMetadata) (Definition, error) {
	ctx, cancel := context.WithTimeout(ctx, queryDeadline)
	defer cancel()
	originalTarget := draftTarget(id)
	table, err := definitionTable(kind)
	if err != nil {
		return Definition{}, err
	}
	if id != "" && (!validID(id) || in.Version < 1 || in.Version > maxSafeVersion) {
		return Definition{}, ErrInvalid
	}
	in.PermissionCodes = unique(in.PermissionCodes)
	in.TemplateIDs = uniqueIDs(in.TemplateIDs)
	tx, err := a.writeBusiness(ctx, p, meta.QueryVersion)
	if err != nil {
		return Definition{}, err
	}
	defer tx.Rollback(context.Background())
	if err := validateDefinition(ctx, tx, kind, in); err != nil {
		return Definition{}, err
	}
	var before any
	action := "IDENTITY_CREATED"
	if kind == Template {
		action = "TEMPLATE_CREATED"
	}
	if id == "" {
		if err := tx.QueryRow(ctx, "INSERT INTO "+table+"(name,description) VALUES($1,$2) RETURNING id::text", in.Name, in.Description).Scan(&id); err != nil {
			return Definition{}, databaseError(err)
		}
	} else {
		var version int64
		if err := tx.QueryRow(ctx, "SELECT version FROM "+table+" WHERE id=$1 FOR UPDATE", id).Scan(&version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Definition{}, ErrMissing
			}
			return Definition{}, databaseError(err)
		}
		if version != in.Version {
			return Definition{}, ErrConflict
		}
		old, err := getDefinition(ctx, tx, kind, id)
		if err != nil {
			return Definition{}, err
		}
		before = safeDefinitionValue(old)
		next := safeDefinition{in.Name, in.Description, in.PermissionCodes, in.TemplateIDs}
		if reflect.DeepEqual(before, next) {
			return old, commitBusiness(ctx, tx, p, meta, DraftKind(kind), originalTarget)
		}
		if version >= maxSafeVersion {
			return Definition{}, ErrConflict
		}
		if _, err := tx.Exec(ctx, "UPDATE "+table+" SET name=$2,description=$3,version=version+1,updated_at=now() WHERE id=$1", id, in.Name, in.Description); err != nil {
			return Definition{}, databaseError(err)
		}
		action = "IDENTITY_UPDATED"
		if kind == Template {
			action = "TEMPLATE_UPDATED"
		}
	}
	if err := replaceDefinitionRelations(ctx, tx, kind, id, in); err != nil {
		return Definition{}, databaseError(err)
	}
	value, err := getDefinition(ctx, tx, kind, id)
	if err != nil {
		return Definition{}, err
	}
	if err := appendChange(ctx, tx, p, meta, action, string(kind), id, before, safeDefinitionValue(value)); err != nil {
		return Definition{}, err
	}
	if err := commitBusiness(ctx, tx, p, meta, DraftKind(kind), originalTarget); err != nil {
		return Definition{}, databaseError(err)
	}
	return value, nil
}
func (a *Application) DeleteDefinition(ctx context.Context, p session.Principal, kind DefinitionKind, id string, version int64, meta RequestMetadata) error {
	ctx, cancel := context.WithTimeout(ctx, queryDeadline)
	defer cancel()
	if !validID(id) || version < 1 || version > maxSafeVersion {
		return ErrInvalid
	}
	table, err := definitionTable(kind)
	if err != nil {
		return err
	}
	tx, err := a.writeBusiness(ctx, p, meta.QueryVersion)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var actual int64
	if err := tx.QueryRow(ctx, "SELECT version FROM "+table+" WHERE id=$1 FOR UPDATE", id).Scan(&actual); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMissing
		}
		return databaseError(err)
	}
	if actual != version {
		return ErrConflict
	}
	old, err := getDefinition(ctx, tx, kind, id)
	if err != nil {
		return err
	}
	if kind == Identity && old.AffectedMembers > 0 || kind == Template && old.AffectedIdentities > 0 {
		return ErrConflict
	}
	if err := replaceDefinitionRelations(ctx, tx, kind, id, DefinitionInput{}); err != nil {
		return databaseError(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE id=$1", id); err != nil {
		return databaseError(err)
	}
	action := "IDENTITY_DELETED"
	if kind == Template {
		action = "TEMPLATE_DELETED"
	}
	if err := appendChange(ctx, tx, p, meta, action, string(kind), id, safeDefinitionValue(old), nil); err != nil {
		return err
	}
	return databaseError(tx.Commit(ctx))
}
