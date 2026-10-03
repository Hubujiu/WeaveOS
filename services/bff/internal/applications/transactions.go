package applications

import (
	"bytes"
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strconv"
	"time"
)

// ExpectedActor is solely a negative constraint on an already authenticated principal.
// An absent legacy header is compatible; it never supplies an identity or a grant.
func ExpectedActor(r *http.Request, p session.Principal) error {
	values := r.Header.Values("X-Expected-Actor-Id")
	if len(values) == 0 {
		return nil
	}
	if len(values) != 1 {
		return ErrExpectedActorInvalid
	}
	id, ok := canonicalID(values[0])
	if !ok {
		return ErrExpectedActorInvalid
	}
	if id != p.UserID {
		return ErrSessionChanged
	}
	return nil
}
func ActorHeaderViolation() any {
	return map[string]any{"violations": []any{map[string]string{"location": "header", "field": "X-Expected-Actor-Id", "code": "VALIDATION_INVALID", "message": "预期账号格式不合法"}}}
}

// ManagerWrite is the shared trusted lifecycle; domain adapters use its one tx.
type ManagerWrite struct {
	tx    pgx.Tx
	actor apppolicy.TrustedActor
	app   App
}

type WriteOptions struct {
	LockTimeout, StatementTimeout time.Duration
	SourceGuard                   func(context.Context, pgx.Tx) error
}

func (a *Application) BeginManagerWrite(c context.Context, p session.Principal, id string, options ...WriteOptions) (*ManagerWrite, error) {
	if a == nil || a.Pool == nil {
		return nil, session.ErrUnavailable
	}
	if _, ok := canonicalID(id); !ok {
		return nil, ErrInvalid
	}
	tx, e := a.Pool.BeginTx(c, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, e
	}
	fail := func(err error) (*ManagerWrite, error) { tx.Rollback(context.Background()); return nil, err }
	if len(options) > 0 {
		o := options[0]
		if o.LockTimeout < time.Millisecond || o.StatementTimeout < time.Millisecond {
			return fail(ErrInvalid)
		}
		if _, e = tx.Exec(c, "SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)", strconv.FormatInt(o.LockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(o.StatementTimeout.Milliseconds(), 10)+"ms"); e != nil {
			return fail(e)
		}
	}
	if _, e = tx.Exec(c, "SELECT personnel.lock_query_revisions()"); e != nil {
		return fail(e)
	}
	access, e := (&personnel.Application{}).AccessForWrite(c, tx, p)
	if e != nil {
		return fail(e)
	}
	actor := apppolicy.TrustedActor{ID: access.User.ID, BootstrapAdmin: access.BootstrapAdmin}
	if len(options) > 0 && options[0].SourceGuard != nil {
		if e = options[0].SourceGuard(c, tx); e != nil {
			return fail(e)
		}
	}
	app, e := loadApp(c, tx, id, true)
	if e != nil {
		return fail(e)
	}
	if e = registered(c, tx, app); e != nil {
		return fail(e)
	}
	if !manager(actor, app) {
		return fail(ErrDenied)
	}
	return &ManagerWrite{tx, actor, app}, nil
}
func (w *ManagerWrite) Tx() pgx.Tx { return w.tx }
func (w *ManagerWrite) Replay(c context.Context, id, kind string, hash [32]byte) (*Result, error) {
	o, a, k, h, e := operation(c, w.tx, w.actor, id)
	if errors.Is(e, ErrMissing) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if a != w.app.ID || k != kind || !bytes.Equal(h, hash[:]) {
		return nil, ErrOperationConflict
	}
	if e = readableOperation(c, w.tx, w.actor, a, k); e != nil {
		return nil, e
	}
	return &Result{Status: o.HTTPStatus, Location: o.Location, Data: o.Result}, nil
}
func (w *ManagerWrite) Claim(c context.Context, id, kind string, hash [32]byte) error {
	_, e := w.tx.Exec(c, "INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint) VALUES($1,$2,$3,$4,$5)", w.actor.ID, id, w.app.ID, kind, hash[:])
	return classify(e)
}
func (w *ManagerWrite) Complete(c context.Context, id string, result Result) error {
	tag, e := w.tx.Exec(c, "UPDATE applications.operations SET result_json=$3::jsonb,http_status=$4,location=$5 WHERE actor_user_id=$1 AND operation_id=$2", w.actor.ID, id, result.Data, result.Status, result.Location)
	if e == nil && tag.RowsAffected() != 1 {
		return ErrMissing
	}
	return e
}
func (w *ManagerWrite) Commit(c context.Context) error   { return commitWrite(c, w.tx) }
func (w *ManagerWrite) Rollback(c context.Context) error { return w.tx.Rollback(c) }
