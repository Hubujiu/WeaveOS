package applications

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// PersonalRead retains the verified actor privately for all resource loads in
// this one read-only RR transaction. No caller-supplied actor can replace it.
type PersonalRead struct {
	pgx.Tx
	actor apppolicy.TrustedActor
}

func (a *Application) BeginPersonalRead(ctx context.Context, p session.Principal) (*PersonalRead, error) {
	tx, actor, err := a.read(ctx, p)
	if err != nil {
		return nil, err
	}
	return &PersonalRead{Tx: tx, actor: actor}, nil
}
func (r *PersonalRead) RecordContext(ctx context.Context, appID, viewID string) (RecordContext, error) {
	if r == nil || r.Tx == nil {
		return RecordContext{}, ErrInvalid
	}
	for _, id := range []string{appID, viewID} {
		if canonical, ok := canonicalID(id); !ok || canonical != id {
			return RecordContext{}, ErrInvalid
		}
	}
	app, err := loadApp(ctx, r.Tx, appID, false)
	if err != nil {
		return RecordContext{}, err
	}
	if err = registered(ctx, r.Tx, app); err != nil {
		return RecordContext{}, err
	}
	return loadRecordContext(ctx, r.Tx, r.actor, app, viewID)
}
