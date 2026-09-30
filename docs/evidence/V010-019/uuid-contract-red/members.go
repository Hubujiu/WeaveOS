package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"reflect"
)

func getMember(ctx context.Context, tx pgx.Tx, id string) (Member, error) {
	var raw []byte
	var snapshot string
	sql := `SELECT jsonb_build_object('id',u.id,'account',u.account,'status',u.status,'bootstrapAdmin',u.is_bootstrap_admin,
 'version',COALESCE((SELECT version FROM personnel.member_configuration WHERE user_id=u.id),0),
 'departmentIds',COALESCE((SELECT jsonb_agg(department_id ORDER BY department_id) FROM personnel.department_members WHERE user_id=u.id),'[]'::jsonb),
 'identityIds',COALESCE((SELECT jsonb_agg(identity_id ORDER BY identity_id) FROM personnel.member_identities WHERE user_id=u.id),'[]'::jsonb),
 'departments',COALESCE((SELECT jsonb_agg(` + departmentJSON + ` ORDER BY d.id) FROM personnel.departments d JOIN personnel.department_members dm ON dm.department_id=d.id WHERE dm.user_id=u.id),'[]'::jsonb),
 'identities',COALESCE((SELECT jsonb_agg(` + definitionSelect(Identity) + ` ORDER BY d.id) FROM personnel.identities d JOIN personnel.member_identities mi ON mi.identity_id=d.id WHERE mi.user_id=u.id),'[]'::jsonb),
 'permissions','[]'::jsonb), u.auth_version::text FROM auth.users u WHERE u.id=$1`
	if err := tx.QueryRow(ctx, sql, id).Scan(&raw, &snapshot); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrMissing
		}
		return Member{}, err
	}
	var result Member
	if err := json.Unmarshal(raw, &result); err != nil {
		return Member{}, err
	}
	if result.Status == "active" {
		access, err := readAccess(ctx, tx, session.Principal{UserID: id, Record: session.Record{AuthVersion: snapshot}})
		if err != nil {
			return Member{}, err
		}
		result.Permissions = access.Permissions
	}
	return result, nil
}
func (a *Application) GetMember(ctx context.Context, p session.Principal, id string) (Member, error) {
	if !validID(id) {
		return Member{}, ErrInvalid
	}
	tx, err := a.read(ctx, p)
	if err != nil {
		return Member{}, err
	}
	defer tx.Rollback(context.Background())
	return getMember(ctx, tx, id)
}
func (a *Application) ListMembers(ctx context.Context, p session.Principal, query MemberQuery) (Page[Member], error) {
	page, err := normalizedPage(query.PageQuery)
	if err != nil {
		return Page[Member]{}, err
	}
	if query.DepartmentID != "" && !validID(query.DepartmentID) || query.IdentityID != "" && !validID(query.IdentityID) {
		return Page[Member]{}, ErrInvalid
	}
	result := Page[Member]{Items: []Member{}, Page: page.Page, PageSize: page.PageSize}
	tx, err := a.read(ctx, p)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	filter := ` WHERE ($1='' OR position(lower($1) in lower(u.account))>0)
 AND ($2='' OR EXISTS(SELECT 1 FROM personnel.department_members dm WHERE dm.user_id=u.id AND dm.department_id=NULLIF($2,'')::uuid))
 AND ($3='' OR EXISTS(SELECT 1 FROM personnel.member_identities mi WHERE mi.user_id=u.id AND mi.identity_id=NULLIF($3,'')::uuid))`
	args := []any{page.Search, query.DepartmentID, query.IdentityID}
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM auth.users u"+filter, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, "SELECT u.id::text FROM auth.users u"+filter+" ORDER BY u.created_at,u.id LIMIT $4 OFFSET $5", append(args, page.PageSize, (page.Page-1)*page.PageSize)...)
	if err != nil {
		return result, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return result, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		value, err := getMember(ctx, tx, id)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, value)
	}
	return result, nil
}
func lockMember(ctx context.Context, tx pgx.Tx, id string, version int64) (Member, error) {
	var exists string
	if err := tx.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE id=$1 FOR SHARE", id).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrMissing
		}
		return Member{}, databaseError(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO personnel.member_configuration(user_id) VALUES($1) ON CONFLICT DO NOTHING", id); err != nil {
		return Member{}, databaseError(err)
	}
	var actual int64
	if err := tx.QueryRow(ctx, "SELECT version FROM personnel.member_configuration WHERE user_id=$1 FOR UPDATE", id).Scan(&actual); err != nil {
		return Member{}, databaseError(err)
	}
	if actual != version {
		return Member{}, ErrConflict
	}
	return getMember(ctx, tx, id)
}
func finishMember(ctx context.Context, tx pgx.Tx, p session.Principal, id, action string, meta RequestMetadata, before, after any) (Member, error) {
	if _, err := tx.Exec(ctx, "UPDATE personnel.member_configuration SET version=version+1,updated_at=now() WHERE user_id=$1", id); err != nil {
		return Member{}, databaseError(err)
	}
	if err := appendChange(ctx, tx, p, meta, action, "member", id, before, after); err != nil {
		return Member{}, err
	}
	result, err := getMember(ctx, tx, id)
	if err != nil {
		return Member{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Member{}, databaseError(err)
	}
	return result, nil
}
func (a *Application) SetMemberIdentities(ctx context.Context, p session.Principal, id string, ids []string, version int64, meta RequestMetadata) (Member, error) {
	if !validID(id) || version < 0 || version > maxSafeVersion {
		return Member{}, ErrInvalid
	}
	ids = unique(ids)
	for _, identity := range ids {
		if !validID(identity) {
			return Member{}, ErrInvalid
		}
	}
	tx, err := a.write(ctx, p)
	if err != nil {
		return Member{}, err
	}
	defer tx.Rollback(context.Background())
	old, err := lockMember(ctx, tx, id, version)
	if err != nil {
		return Member{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.identities WHERE id=ANY($1::uuid[])", ids).Scan(&count); err != nil {
		return Member{}, err
	}
	if count != len(ids) {
		return Member{}, ErrInvalid
	}
	if reflect.DeepEqual(old.IdentityIDs, ids) {
		return old, nil
	}
	if version >= maxSafeVersion {
		return Member{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, "DELETE FROM personnel.member_identities WHERE user_id=$1", id); err != nil {
		return Member{}, databaseError(err)
	}
	for _, identity := range ids {
		if _, err := tx.Exec(ctx, "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", id, identity); err != nil {
			return Member{}, databaseError(err)
		}
	}
	return finishMember(ctx, tx, p, id, "MEMBER_IDENTITIES_UPDATED", meta, map[string]any{"identityIds": old.IdentityIDs}, map[string]any{"identityIds": ids})
}
func (a *Application) ChangeMemberGroups(ctx context.Context, p session.Principal, id string, in GroupInput, meta RequestMetadata) (Member, error) {
	if !validID(id) || !validID(in.DepartmentID) || in.Version < 0 || in.Version > maxSafeVersion {
		return Member{}, ErrInvalid
	}
	if in.Operation != "add" && in.Operation != "remove" && in.Operation != "move" {
		return Member{}, ErrInvalid
	}
	if in.Operation == "move" && !validID(in.SourceDepartmentID) {
		return Member{}, ErrInvalid
	}
	tx, err := a.write(ctx, p)
	if err != nil {
		return Member{}, err
	}
	defer tx.Rollback(context.Background())
	old, err := lockMember(ctx, tx, id, in.Version)
	if err != nil {
		return Member{}, err
	}
	check := []string{in.DepartmentID}
	if in.Operation == "move" {
		check = unique(append(check, in.SourceDepartmentID))
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM personnel.departments WHERE id=ANY($1::uuid[])", check).Scan(&count); err != nil {
		return Member{}, err
	}
	if count != len(check) {
		return Member{}, ErrInvalid
	}
	set := map[string]bool{}
	for _, department := range old.DepartmentIDs {
		set[department] = true
	}
	switch in.Operation {
	case "add":
		set[in.DepartmentID] = true
	case "remove":
		delete(set, in.DepartmentID)
	case "move":
		if !set[in.SourceDepartmentID] && !set[in.DepartmentID] {
			return Member{}, ErrConflict
		}
		delete(set, in.SourceDepartmentID)
		set[in.DepartmentID] = true
	}
	next := []string{}
	for department := range set {
		next = append(next, department)
	}
	next = unique(next)
	if reflect.DeepEqual(old.DepartmentIDs, next) {
		return old, nil
	}
	if in.Version >= maxSafeVersion {
		return Member{}, ErrConflict
	}
	// Touch only the explicitly requested source/target, preserving every other relation.
	if in.Operation == "remove" || in.Operation == "move" {
		source := in.DepartmentID
		if in.Operation == "move" {
			source = in.SourceDepartmentID
		}
		if _, err := tx.Exec(ctx, "DELETE FROM personnel.department_members WHERE user_id=$1 AND department_id=$2", id, source); err != nil {
			return Member{}, databaseError(err)
		}
	}
	if in.Operation == "add" || in.Operation == "move" {
		if _, err := tx.Exec(ctx, "INSERT INTO personnel.department_members(user_id,department_id) VALUES($1,$2) ON CONFLICT DO NOTHING", id, in.DepartmentID); err != nil {
			return Member{}, databaseError(err)
		}
	}
	return finishMember(ctx, tx, p, id, "MEMBER_GROUPS_UPDATED", meta, map[string]any{"departmentIds": old.DepartmentIDs}, map[string]any{"departmentIds": next})
}
