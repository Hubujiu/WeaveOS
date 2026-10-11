package applications

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// BeginStructureCopyRead authenticates both ends in one caller-owned snapshot.
// Empty targetApp means a new application, never an omitted authorization gate.
func (a *Application) BeginStructureCopyRead(ctx context.Context, p session.Principal, sourceApp, targetApp string, targetDirectory *string) (pgx.Tx, App, int64, int64, error) {
	if sourceApp == "" {
		return nil, App{}, 0, 0, ErrInvalid
	}
	for _, id := range []string{sourceApp, targetApp} {
		if id == "" {
			continue
		}
		if v, ok := canonicalID(id); !ok || v != id || id == "00000000-0000-0000-0000-000000000000" {
			return nil, App{}, 0, 0, ErrInvalid
		}
	}
	if targetDirectory != nil {
		if v, ok := canonicalID(*targetDirectory); !ok || v != *targetDirectory || targetApp == "" {
			return nil, App{}, 0, 0, ErrInvalid
		}
	}
	tx, actor, e := a.read(ctx, p)
	if e != nil {
		return nil, App{}, 0, 0, e
	}
	fail := func(err error) (pgx.Tx, App, int64, int64, error) {
		_ = tx.Rollback(context.Background())
		return nil, App{}, 0, 0, err
	}
	source, e := loadApp(ctx, tx, sourceApp, false)
	if e != nil {
		return fail(e)
	}
	if e = registered(ctx, tx, source); e != nil {
		return fail(e)
	}
	if !manager(actor, source) {
		return fail(ErrDenied)
	}
	if targetApp == "" {
		access, err := (&personnel.Application{}).AccessForRead(ctx, tx, p)
		if err != nil {
			return fail(err)
		}
		for _, permission := range access.Permissions {
			if permission.Code == "applications.create" {
				actor.CreateApp = true
			}
		}
		if !apppolicy.CanCreate(actor) {
			return fail(ErrDenied)
		}
		return tx, source, 0, 0, nil
	}
	target, e := loadApp(ctx, tx, targetApp, false)
	if e != nil {
		return fail(e)
	}
	if e = registered(ctx, tx, target); e != nil {
		return fail(e)
	}
	if !manager(actor, target) {
		return fail(ErrDenied)
	}
	if targetDirectory != nil {
		var yes bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM applications.directories WHERE app_id=$1 AND id=$2 AND deleted_at IS NULL)", targetApp, *targetDirectory).Scan(&yes); e != nil {
			return fail(e)
		}
		if !yes {
			return fail(ErrResourceInvalid)
		}
	}
	var version int64
	if e = tx.QueryRow(ctx, "SELECT structure_version FROM applications.apps WHERE id=$1", targetApp).Scan(&version); e != nil {
		return fail(e)
	}
	return tx, source, version, target.PolicyRevision, nil
}
