package applications

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// BeginCreateRead returns a caller-owned, read-only RR transaction. A successful
// preflight is advisory; every later write must reacquire current authority.
func (a *Application) BeginCreateRead(ctx context.Context, p session.Principal) (pgx.Tx, error) {
	tx, actor, e := a.read(ctx, p)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (pgx.Tx, error) { _ = tx.Rollback(context.Background()); return nil, e }
	access, e := (&personnel.Application{}).AccessForRead(ctx, tx, p)
	if e != nil {
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
	return tx, nil
}
