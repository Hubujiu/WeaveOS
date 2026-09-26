package audit

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Result struct{ Archived, Expired int64 }

func Maintain(ctx context.Context, live, cold *pgxpool.Pool, now time.Time) (Result, error) {
	return Result{}, nil
}
