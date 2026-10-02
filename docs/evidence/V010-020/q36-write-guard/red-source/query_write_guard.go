package personnel

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// Compile-only declaration for the real transaction RED.
func (a *Application) BeginQueryWrite(ctx context.Context, p session.Principal, version string) (pgx.Tx, error) {
	return nil, ErrNotImplemented
}
