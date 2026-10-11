package applications

import (
	"bytes"
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"sort"
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

const StructureCopyKind = "structure.copy"

type StructureCopyWrite struct {
	*ManagerWrite
	Source           App
	Target           App
	StructureVersion int64
}
type StructureCopyOptions struct {
	OperationID                              string
	Fingerprint                              [32]byte
	SourceAppID, TargetAppID, NewAppID, Name string
	TargetDirectoryID                        *string
	SourceGuard                              func(context.Context, pgx.Tx) error
}

func (a *Application) BeginStructureCopyWrite(ctx context.Context, p session.Principal, o StructureCopyOptions) (*StructureCopyWrite, *Result, error) {
	if a == nil || a.Pool == nil {
		return nil, nil, session.ErrUnavailable
	}
	if o.SourceGuard == nil {
		return nil, nil, ErrInvalid
	}
	for _, id := range []string{o.SourceAppID, o.OperationID} {
		if v, ok := canonicalID(id); !ok || v != id || id == "00000000-0000-0000-0000-000000000000" {
			return nil, nil, ErrInvalid
		}
	}
	target := o.TargetAppID
	if target == "" {
		target = o.NewAppID
	}
	if v, ok := canonicalID(target); !ok || v != target || target == "00000000-0000-0000-0000-000000000000" {
		return nil, nil, ErrInvalid
	}
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, nil, e
	}
	fail := func(err error) (*StructureCopyWrite, *Result, error) {
		_ = tx.Rollback(context.Background())
		return nil, nil, err
	}
	if _, e = tx.Exec(ctx, "SELECT set_config('lock_timeout','2s',true),set_config('statement_timeout','10s',true)"); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); e != nil {
		return fail(e)
	}
	access, e := (&personnel.Application{}).AccessForWrite(ctx, tx, p)
	if e != nil {
		return fail(e)
	}
	actor := apppolicy.TrustedActor{ID: access.User.ID, BootstrapAdmin: access.BootstrapAdmin}
	old, oldApp, kind, hash, e := operation(ctx, tx, actor, o.OperationID)
	if e == nil {
		if kind != StructureCopyKind || !bytes.Equal(hash, o.Fingerprint[:]) {
			return fail(ErrOperationConflict)
		}
		return &StructureCopyWrite{ManagerWrite: &ManagerWrite{tx: tx, actor: actor, app: App{ID: oldApp}}}, &Result{Status: old.HTTPStatus, Location: old.Location, Data: old.Result}, nil
	}
	if !errors.Is(e, ErrMissing) {
		return fail(e)
	}
	if e = o.SourceGuard(ctx, tx); e != nil {
		return fail(e)
	}
	ids := []string{o.SourceAppID}
	if o.TargetAppID != "" && o.TargetAppID != o.SourceAppID {
		ids = append(ids, o.TargetAppID)
	}
	sort.Strings(ids)
	apps := map[string]App{}
	for _, id := range ids {
		app, err := loadApp(ctx, tx, id, true)
		if err != nil {
			return fail(err)
		}
		if err = registered(ctx, tx, app); err != nil {
			return fail(err)
		}
		if !manager(actor, app) {
			return fail(ErrDenied)
		}
		apps[id] = app
	}
	var targetApp App
	var version int64
	if o.TargetAppID == "" {
		for _, permission := range access.Permissions {
			if permission.Code == "applications.create" {
				actor.CreateApp = true
			}
		}
		if !apppolicy.CanCreate(actor) {
			return fail(ErrDenied)
		}
		if _, e = tx.Exec(ctx, "INSERT INTO applications.apps(id,name,owner_user_id,structure_version) VALUES($1,$2,$3,1)", target, o.Name, actor.ID); e != nil {
			return fail(e)
		}
		if _, e = tx.Exec(ctx, "INSERT INTO applications.menu_resources(app_id,resource_kind,resource_id) VALUES($1,'application',$1)", target); e != nil {
			return fail(e)
		}
		if _, e = tx.Exec(ctx, "SELECT applications.register_catalog_entry($1)", target); e != nil {
			return fail(e)
		}
		targetApp = App{ID: target, Name: o.Name, OwnerUserID: actor.ID, PolicyRevision: 1}
	} else {
		targetApp = apps[target]
		if e = tx.QueryRow(ctx, "SELECT structure_version FROM applications.apps WHERE id=$1", target).Scan(&version); e != nil {
			return fail(e)
		}
		if o.TargetDirectoryID != nil {
			var yes bool
			if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM applications.directories WHERE app_id=$1 AND id=$2 AND deleted_at IS NULL)", target, *o.TargetDirectoryID).Scan(&yes); e != nil {
				return fail(e)
			}
			if !yes {
				return fail(ErrResourceInvalid)
			}
		}
	}
	w := &ManagerWrite{tx: tx, actor: actor, app: targetApp}
	if e = w.Claim(ctx, o.OperationID, StructureCopyKind, o.Fingerprint); e != nil {
		return fail(e)
	}
	return &StructureCopyWrite{ManagerWrite: w, Source: apps[o.SourceAppID], Target: targetApp, StructureVersion: version}, nil, nil
}
