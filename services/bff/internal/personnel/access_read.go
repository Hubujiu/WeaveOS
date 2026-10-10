package personnel

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// AccessForRead resolves live access in the caller's snapshot without creating
// member configuration rows or taking write locks. It grants no write authority.
func (a *Application) AccessForRead(ctx context.Context, tx pgx.Tx, p session.Principal) (Access, error) {
	if tx == nil {
		return Access{}, ErrInvalid
	}
	return readAccess(ctx, tx, p)
}
