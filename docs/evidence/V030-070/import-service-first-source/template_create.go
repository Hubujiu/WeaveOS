package applications

import (
	"bytes"
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const TemplateImportKind = "application.template.import"

type TemplateCreateOptions struct {
	OperationID                   string
	Fingerprint                   [32]byte
	LockTimeout, StatementTimeout time.Duration
	SourceGuard                   func(context.Context, pgx.Tx) error
}

// BeginTemplateCreate owns current identity, global creation authority and app
// registration. A successful return transfers transaction ownership to the caller.
func (a *Application) BeginTemplateCreate(ctx context.Context, p session.Principal, appID, name string, o TemplateCreateOptions) (*ManagerWrite, *Result, error) {
	if a == nil || a.Pool == nil {
		return nil, nil, session.ErrUnavailable
	}
	for _, id := range []string{appID, o.OperationID} {
		if v, ok := canonicalID(id); !ok || v != id || id == "00000000-0000-0000-0000-000000000000" {
			return nil, nil, ErrInvalid
		}
	}
	if name == "" || name != strings.TrimSpace(name) || strings.ContainsRune(name, 0) || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 || o.LockTimeout < time.Millisecond || o.StatementTimeout < time.Millisecond || o.SourceGuard == nil {
		return nil, nil, ErrInvalid
	}
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, nil, e
	}
	fail := func(e error) (*ManagerWrite, *Result, error) {
		_ = tx.Rollback(context.Background())
		return nil, nil, e
	}
	if _, e = tx.Exec(ctx, "SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)", strconv.FormatInt(o.LockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(o.StatementTimeout.Milliseconds(), 10)+"ms"); e != nil {
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
		if kind != TemplateImportKind || !bytes.Equal(hash, o.Fingerprint[:]) {
			return fail(ErrOperationConflict)
		}
		return &ManagerWrite{tx: tx, actor: actor, app: App{ID: oldApp}}, &Result{Status: old.HTTPStatus, Location: old.Location, Data: old.Result}, nil
	}
	if !errors.Is(e, ErrMissing) {
		return fail(e)
	}
	for _, permission := range access.Permissions {
		if permission.Code == "applications.create" {
			actor.CreateApp = true
		}
	}
	if !apppolicy.CanCreate(actor) {
		return fail(ErrDenied)
	}
	if e = o.SourceGuard(ctx, tx); e != nil {
		return fail(e)
	}
	var claimed string
	e = tx.QueryRow(ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint) VALUES($1,$2,$3,$4,$5) ON CONFLICT(actor_user_id,operation_id) DO NOTHING RETURNING operation_id::text`, actor.ID, o.OperationID, appID, TemplateImportKind, o.Fingerprint[:]).Scan(&claimed)
	if errors.Is(e, pgx.ErrNoRows) {
		return fail(ErrOperationConflict)
	}
	if e != nil {
		return fail(classify(e))
	}
	if _, e = tx.Exec(ctx, "INSERT INTO applications.apps(id,name,owner_user_id,structure_version) VALUES($1,$2,$3,1)", appID, name, actor.ID); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO applications.menu_resources(app_id,resource_kind,resource_id) VALUES($1,'application',$1)", appID); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(ctx, "SELECT applications.register_catalog_entry($1)", appID); e != nil {
		return fail(e)
	}
	return &ManagerWrite{tx: tx, actor: actor, app: App{ID: appID, Name: name, OwnerUserID: actor.ID, PolicyRevision: 1}}, nil, nil
}
