package applications

import (
	"bytes"
	"context"
	"encoding/json"
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

// RecordContext is resolved inside the shared transaction from a live Session
// and real app/form/table metadata. HTTP JSON must never supply these facts.
type RecordContext struct {
	Actor           apppolicy.TrustedActor
	App             App
	ViewID, TableID string
	SchemaVersion   int64
	ViewVersion     int64
	SchemaReady     bool
	Fields          json.RawMessage
	Layout          json.RawMessage
	Grants          []Grant
}

// BeginRecordRead leaves the RR transaction owned by the consumer.
func (a *Application) BeginRecordRead(c context.Context, p session.Principal, appID, viewID string) (pgx.Tx, RecordContext, error) {
	if canonical, ok := canonicalID(appID); !ok || canonical != appID {
		return nil, RecordContext{}, ErrInvalid
	}
	if canonical, ok := canonicalID(viewID); !ok || canonical != viewID {
		return nil, RecordContext{}, ErrInvalid
	}
	tx, actor, e := a.read(c, p)
	if e != nil {
		return nil, RecordContext{}, e
	}
	fail := func(e error) (pgx.Tx, RecordContext, error) {
		tx.Rollback(context.Background())
		return nil, RecordContext{}, e
	}
	app, e := loadApp(c, tx, appID, false)
	if e != nil {
		return fail(e)
	}
	if e = registered(c, tx, app); e != nil {
		return fail(e)
	}
	facts, e := loadRecordContext(c, tx, actor, app, viewID)
	if e != nil {
		return fail(e)
	}
	return tx, facts, nil
}

func loadRecordContext(c context.Context, tx pgx.Tx, actor apppolicy.TrustedActor, app App, viewID string) (RecordContext, error) {
	facts := RecordContext{Actor: actor, App: app, ViewID: viewID, Grants: []Grant{}}
	e := tx.QueryRow(c, `SELECT f.table_id::text,t.schema_version,t.schema_ready,t.fields_json,f.view_version,f.layout
 FROM applications.form_views f JOIN applications.logical_tables t ON t.app_id=f.app_id AND t.id=f.table_id
 JOIN applications.menu_resources r ON r.app_id=f.app_id AND r.resource_kind='form' AND r.resource_id=f.id
 WHERE f.app_id=$1 AND f.id=$2 AND f.deleted_at IS NULL AND t.deleted_at IS NULL`, app.ID, viewID).Scan(&facts.TableID, &facts.SchemaVersion, &facts.SchemaReady, &facts.Fields, &facts.ViewVersion, &facts.Layout)
	if errors.Is(e, pgx.ErrNoRows) {
		return RecordContext{}, ErrMissing
	}
	if e != nil {
		return RecordContext{}, e
	}
	rows, e := tx.Query(c, `SELECT g.resource_kind,g.resource_id::text,g.action,g.row_scope,
 ARRAY(SELECT field_id::text FROM applications.grant_fields gf WHERE gf.app_id=g.app_id AND gf.grant_id=g.id ORDER BY gf.field_id)
 FROM applications.grants g JOIN applications.permission_groups p ON p.app_id=g.app_id AND p.id=g.group_id AND p.enabled
 JOIN applications.group_members m ON m.app_id=p.app_id AND m.group_id=p.id AND m.user_id=$2
 WHERE g.app_id=$1 AND g.resource_kind='form' AND g.resource_id=$3 ORDER BY g.id`, app.ID, actor.ID, viewID)
	if e != nil {
		return RecordContext{}, e
	}
	defer rows.Close()
	for rows.Next() {
		var grant Grant
		if e = rows.Scan(&grant.ResourceKind, &grant.ResourceID, &grant.Action, &grant.RowScope, &grant.Fields); e != nil {
			return RecordContext{}, e
		}
		facts.Grants = append(facts.Grants, grant)
	}
	return facts, rows.Err()
}

type RecordWriteOptions struct {
	LockTimeout, StatementTimeout time.Duration
	OperationID, Kind             string
	Fingerprint                   [32]byte
	SourceGuard                   func(context.Context, pgx.Tx) error
	Authorize                     func(context.Context, pgx.Tx, RecordContext) error
}
type RecordWrite struct {
	*ManagerWrite
	context   RecordContext
	authorize func(context.Context, pgx.Tx, RecordContext) error
	options   RecordWriteOptions
}

func (a *Application) BeginRecordWrite(c context.Context, p session.Principal, appID, viewID string, options RecordWriteOptions) (*RecordWrite, error) {
	if a == nil || a.Pool == nil {
		return nil, session.ErrUnavailable
	}
	for _, id := range []string{appID, viewID, options.OperationID} {
		if canonical, ok := canonicalID(id); !ok || canonical != id {
			return nil, ErrInvalid
		}
	}
	if !recordOperation(options.Kind) || options.LockTimeout < time.Millisecond || options.StatementTimeout < time.Millisecond {
		return nil, ErrInvalid
	}
	tx, e := a.Pool.BeginTx(c, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, e
	}
	fail := func(err error) (*RecordWrite, error) { tx.Rollback(context.Background()); return nil, err }
	if _, e = tx.Exec(c, "SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)", strconv.FormatInt(options.LockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(options.StatementTimeout.Milliseconds(), 10)+"ms"); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(c, "SELECT personnel.lock_query_revisions()"); e != nil {
		return fail(e)
	}
	access, e := (&personnel.Application{}).AccessForWrite(c, tx, p)
	if e != nil {
		return fail(e)
	}
	actor := apppolicy.TrustedActor{ID: access.User.ID, BootstrapAdmin: access.BootstrapAdmin}
	_, oldApp, oldKind, oldHash, oldErr := operation(c, tx, actor, options.OperationID)
	confirmed := oldErr == nil
	if confirmed && (oldApp != appID || oldKind != options.Kind || !bytes.Equal(oldHash, options.Fingerprint[:])) {
		return fail(ErrOperationConflict)
	}
	if oldErr != nil && !errors.Is(oldErr, ErrMissing) {
		return fail(oldErr)
	}
	// Confirmed minimum results recover before source/CAS/action-policy checks.
	if !confirmed && options.SourceGuard != nil {
		if e = options.SourceGuard(c, tx); e != nil {
			return fail(e)
		}
	}
	app, e := loadApp(c, tx, appID, true)
	if e != nil {
		return fail(e)
	}
	facts := RecordContext{Actor: actor, App: app, ViewID: viewID, Grants: []Grant{}}
	if !confirmed {
		if e = registered(c, tx, app); e != nil {
			return fail(e)
		}
		facts, e = loadRecordContext(c, tx, actor, app, viewID)
		if e != nil {
			return fail(e)
		}
	}
	return &RecordWrite{ManagerWrite: &ManagerWrite{tx: tx, actor: actor, app: app}, context: facts, authorize: options.Authorize, options: options}, nil
}
func (w *RecordWrite) Context() RecordContext { return w.context }

func recordOperation(kind string) bool {
	switch kind {
	case "record.delete", "record.restore":
		return true
	case "preset.create", "preset.update", "preset.discard", "workflow.round.resubmit", "workflow.round.review", "workflow.manual.start", "record.create", "record.edit", "draft.create", "draft.update", "draft.discard", "workflow.task.agree", "workflow.task.reject", "workflow.instance.withdraw", "workflow.task.return":
		return true
	}
	return false
}
func (w *RecordWrite) Claim(c context.Context, id, kind string, hash [32]byte) error {
	if id != w.options.OperationID || kind != w.options.Kind || hash != w.options.Fingerprint {
		return ErrOperationConflict
	}
	if w.authorize == nil {
		return session.ErrUnavailable
	}
	if e := w.authorize(c, w.tx, w.context); e != nil {
		return e
	}
	return w.ManagerWrite.Claim(c, id, kind, hash)
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
		// Owner is immutable. Check current registration and permission before
		// inspecting sources, then recheck after acquiring the app gate below.
		app, e := loadApp(c, tx, id, false)
		if e != nil {
			return fail(e)
		}
		if e = registered(c, tx, app); e != nil {
			return fail(e)
		}
		if !manager(actor, app) {
			return fail(ErrDenied)
		}
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

// StructureDeletionOptions binds replay to one closed destructive command.
type StructureDeletionOptions struct {
	WriteOptions
	OperationID, Kind string
	Fingerprint       [32]byte
}

func structureDeletion(kind string) bool {
	switch kind {
	case "application.delete", "directory.delete", "table.delete", "form.delete":
		return true
	}
	return false
}

// BeginStructureDeletion recovers a confirmed minimum receipt before current
// resource eligibility. New commands still require the live manager/app gate.
func (a *Application) BeginStructureDeletion(c context.Context, p session.Principal, id string, o StructureDeletionOptions) (*ManagerWrite, error) {
	if a == nil || a.Pool == nil {
		return nil, session.ErrUnavailable
	}
	for _, v := range []string{id, o.OperationID} {
		canonical, ok := canonicalID(v)
		if !ok || canonical != v || v == "00000000-0000-0000-0000-000000000000" {
			return nil, ErrInvalid
		}
	}
	if !structureDeletion(o.Kind) || o.LockTimeout < time.Millisecond || o.StatementTimeout < time.Millisecond {
		return nil, ErrInvalid
	}
	tx, e := a.Pool.BeginTx(c, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*ManagerWrite, error) { tx.Rollback(context.Background()); return nil, e }
	if _, e = tx.Exec(c, "SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)", strconv.FormatInt(o.LockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(o.StatementTimeout.Milliseconds(), 10)+"ms"); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(c, "SELECT personnel.lock_query_revisions()"); e != nil {
		return fail(e)
	}
	access, e := (&personnel.Application{}).AccessForWrite(c, tx, p)
	if e != nil {
		return fail(e)
	}
	actor := apppolicy.TrustedActor{ID: access.User.ID, BootstrapAdmin: access.BootstrapAdmin}
	_, oldApp, oldKind, oldHash, e := operation(c, tx, actor, o.OperationID)
	if e == nil {
		if oldApp != id || oldKind != o.Kind || !bytes.Equal(oldHash, o.Fingerprint[:]) {
			return fail(ErrOperationConflict)
		}
		return &ManagerWrite{tx: tx, actor: actor, app: App{ID: id}}, nil
	}
	if !errors.Is(e, ErrMissing) {
		return fail(e)
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
	return &ManagerWrite{tx: tx, actor: actor, app: app}, nil
}
