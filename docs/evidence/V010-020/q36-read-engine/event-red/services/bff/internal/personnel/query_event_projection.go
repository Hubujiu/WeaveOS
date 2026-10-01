package personnel

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type EventProjectionPage struct {
	Items       []QueryActivity
	Total       int64
	Fingerprint string
}

func scanEventProjection(ctx context.Context, tx pgx.Tx, in EventQueryInput) (EventProjectionPage, error) {
	return EventProjectionPage{}, ErrNotImplemented
}
func scanEventPage(ctx context.Context, tx pgx.Tx, in EventQueryInput) ([]QueryActivity, error) {
	return nil, ErrNotImplemented
}
