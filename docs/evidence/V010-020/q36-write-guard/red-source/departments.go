package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

const departmentJSON = `jsonb_build_object('id',d.id,'parentId',d.parent_id,'name',d.name,'isRoot',d.is_root,'version',d.version,
 'memberCount',(SELECT count(*) FROM personnel.department_members WHERE department_id=d.id),
 'childrenCount',(SELECT count(*) FROM personnel.departments WHERE parent_id=d.id))`

func getDepartment(ctx context.Context, tx pgx.Tx, id string) (Department, error) {
	var raw []byte
	if err := tx.QueryRow(ctx, "SELECT "+departmentJSON+" FROM personnel.departments d WHERE id=$1", id).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Department{}, ErrMissing
		}
		return Department{}, err
	}
	var result Department
	err := json.Unmarshal(raw, &result)
	return result, err
}
func (a *Application) Departments(ctx context.Context, p session.Principal) ([]Department, error) {
	tx, err := a.read(ctx, p)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	result := []Department{}
	rows, err := tx.Query(ctx, "SELECT "+departmentJSON+" FROM personnel.departments d ORDER BY is_root DESC,created_at,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value Department
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

type safeDepartment struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

func (a *Application) SaveDepartment(ctx context.Context, p session.Principal, id string, in DepartmentInput, meta RequestMetadata) (Department, error) {
	if !validName(in.Name) || (id == "" && !validID(in.ParentID)) || (id != "" && (!validID(id) || in.Version < 1 || in.Version > maxSafeVersion || in.ParentID != "")) {
		return Department{}, ErrInvalid
	}
	tx, err := a.writeBusiness(ctx, p)
	if err != nil {
		return Department{}, err
	}
	defer tx.Rollback(context.Background())
	action := "DEPARTMENT_CREATED"
	var before any
	if id == "" {
		var parent string
		if err := tx.QueryRow(ctx, "SELECT id::text FROM personnel.departments WHERE id=$1 FOR KEY SHARE", in.ParentID).Scan(&parent); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Department{}, ErrInvalid
			}
			return Department{}, err
		}
		if err := tx.QueryRow(ctx, "INSERT INTO personnel.departments(name,parent_id) VALUES($1,$2) RETURNING id::text", in.Name, in.ParentID).Scan(&id); err != nil {
			return Department{}, databaseError(err)
		}
	} else {
		var actual int64
		if err := tx.QueryRow(ctx, "SELECT version FROM personnel.departments WHERE id=$1 FOR UPDATE", id).Scan(&actual); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Department{}, ErrMissing
			}
			return Department{}, databaseError(err)
		}
		if actual != in.Version {
			return Department{}, ErrConflict
		}
		old, err := getDepartment(ctx, tx, id)
		if err != nil {
			return Department{}, err
		}
		if old.Name == in.Name {
			return old, nil
		}
		if actual >= maxSafeVersion {
			return Department{}, ErrConflict
		}
		before = safeDepartment{old.Name, old.ParentID}
		action = "DEPARTMENT_UPDATED"
		if _, err := tx.Exec(ctx, "UPDATE personnel.departments SET name=$2,version=version+1,updated_at=now() WHERE id=$1", id, in.Name); err != nil {
			return Department{}, databaseError(err)
		}
	}
	value, err := getDepartment(ctx, tx, id)
	if err != nil {
		return Department{}, err
	}
	if err := appendChange(ctx, tx, p, meta, action, "department", id, before, safeDepartment{value.Name, value.ParentID}); err != nil {
		return Department{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Department{}, databaseError(err)
	}
	return value, nil
}
func (a *Application) DeleteDepartment(ctx context.Context, p session.Principal, id string, version int64, meta RequestMetadata) error {
	if !validID(id) || version < 1 || version > maxSafeVersion {
		return ErrInvalid
	}
	tx, err := a.writeBusiness(ctx, p)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var actual int64
	if err := tx.QueryRow(ctx, "SELECT version FROM personnel.departments WHERE id=$1 FOR UPDATE", id).Scan(&actual); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMissing
		}
		return databaseError(err)
	}
	if actual != version {
		return ErrConflict
	}
	old, err := getDepartment(ctx, tx, id)
	if err != nil {
		return err
	}
	if old.IsRoot || old.MemberCount > 0 || old.ChildrenCount > 0 {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, "DELETE FROM personnel.departments WHERE id=$1", id); err != nil {
		return databaseError(err)
	}
	if err := appendChange(ctx, tx, p, meta, "DEPARTMENT_DELETED", "department", id, safeDepartment{old.Name, old.ParentID}, nil); err != nil {
		return err
	}
	return databaseError(tx.Commit(ctx))
}
