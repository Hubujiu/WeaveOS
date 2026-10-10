package applications

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// BeginManagerRead returns one caller-owned read-only RR snapshot. It never
// trusts a client/principal claim of Bootstrap authority without the database.
func (a *Application) BeginManagerRead(ctx context.Context, p session.Principal, id string) (pgx.Tx, App, error) {
	if canonical, ok := canonicalID(id); !ok || canonical != id {
		return nil, App{}, ErrInvalid
	}
	tx, actor, e := a.read(ctx, p)
	if e != nil {
		return nil, App{}, e
	}
	fail := func(e error) (pgx.Tx, App, error) { _ = tx.Rollback(context.Background()); return nil, App{}, e }
	app, e := loadApp(ctx, tx, id, false)
	if e != nil {
		return fail(e)
	}
	if e = registered(ctx, tx, app); e != nil {
		return fail(e)
	}
	if !manager(actor, app) {
		return fail(ErrDenied)
	}
	return tx, app, nil
}
