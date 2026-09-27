package audit

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func Scheduled(ctx context.Context, live, cold *pgxpool.Pool, every time.Duration, clock func() time.Time, report func(Result, error)) error {
	if every <= 0 || clock == nil || report == nil {
		return errors.New("maintenance schedule invalid")
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		operation, cancel := context.WithTimeout(ctx, 5*time.Minute)
		result, err := Maintain(operation, live, cold, clock())
		cancel()
		report(result, err)
		timer := time.NewTimer(every)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
