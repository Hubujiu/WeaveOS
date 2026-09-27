package audit

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func Scheduled(ctx context.Context, live, cold *pgxpool.Pool, every time.Duration, clock func() time.Time, report func(Result, error)) error {
	return nil
}
