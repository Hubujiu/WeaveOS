package audit

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Result struct{ Archived, Expired int64 }

func Maintain(ctx context.Context, live, cold *pgxpool.Pool, now time.Time) (Result, error) {
	if live == nil || cold == nil || now.IsZero() {
		return Result{}, errors.New("audit maintenance configuration invalid")
	}
	now = now.UTC()
	connection, err := live.Acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer connection.Release()
	var locked bool
	if err := connection.QueryRow(ctx, "SELECT pg_try_advisory_lock(87310008)").Scan(&locked); err != nil {
		return Result{}, err
	}
	if !locked {
		return Result{}, errors.New("audit maintenance already running")
	}
	defer func() {
		unlock, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = connection.Exec(unlock, "SELECT pg_advisory_unlock(87310008)")
	}()
	var liveDB, coldDB string
	if err := connection.QueryRow(ctx, "SELECT current_database()").Scan(&liveDB); err != nil {
		return Result{}, err
	}
	if err := cold.QueryRow(ctx, "SELECT current_database()").Scan(&coldDB); err != nil {
		return Result{}, err
	}
	if liveDB == coldDB {
		return Result{}, errors.New("cold archive must use a separate database")
	}
	result := Result{}
	expiry, err := cold.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer expiry.Rollback(context.Background())
	if _, err = expiry.Exec(ctx, "SET LOCAL TIME ZONE 'UTC'"); err != nil {
		return Result{}, err
	}
	removed, err := expiry.Exec(ctx, "DELETE FROM archive.authentication_events WHERE occurred_at <= $1::timestamptz - interval '1 year'", now)
	if err != nil {
		return Result{}, err
	}
	if err = expiry.Commit(ctx); err != nil {
		return Result{}, err
	}
	result.Expired += removed.RowsAffected()
	for {
		hot, err := connection.Begin(ctx)
		if err != nil {
			return Result{}, err
		}
		batch, done, err := moveBatch(ctx, hot, cold, now)
		if err != nil {
			_ = hot.Rollback(context.Background())
			return Result{}, err
		}
		if err = hot.Commit(ctx); err != nil {
			return Result{}, err
		}
		result.Archived += batch.Archived
		result.Expired += batch.Expired
		if done {
			return result, nil
		}
	}
}

func moveBatch(ctx context.Context, hot pgx.Tx, cold *pgxpool.Pool, now time.Time) (Result, bool, error) {
	if _, err := hot.Exec(ctx, "SET LOCAL TIME ZONE 'UTC'"); err != nil {
		return Result{}, false, err
	}
	removed, err := hot.Exec(ctx, "DELETE FROM auth.authentication_events WHERE occurred_at <= $1::timestamptz - interval '1 year'", now)
	if err != nil {
		return Result{}, false, err
	}
	rows, err := hot.Query(ctx, "SELECT id::text,to_jsonb(e) FROM auth.authentication_events e WHERE occurred_at < date_trunc('month',$1::timestamptz) ORDER BY occurred_at,id LIMIT 100 FOR UPDATE", now)
	if err != nil {
		return Result{}, false, err
	}
	type event struct {
		id   string
		data []byte
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.id, &e.data); err != nil {
			rows.Close()
			return Result{}, false, err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Result{}, false, err
	}
	result := Result{Expired: removed.RowsAffected()}
	if len(events) == 0 {
		return result, true, nil
	}
	copyTx, err := cold.Begin(ctx)
	if err != nil {
		return Result{}, false, err
	}
	defer copyTx.Rollback(context.Background())
	if _, err := copyTx.Exec(ctx, "SET LOCAL TIME ZONE 'UTC'"); err != nil {
		return Result{}, false, err
	}
	for _, e := range events {
		if _, err := copyTx.Exec(ctx, "INSERT INTO archive.authentication_events SELECT (json_populate_record(NULL::archive.authentication_events,$1::json)).* ON CONFLICT (id) DO NOTHING", string(e.data)); err != nil {
			return Result{}, false, err
		}
		var identical bool
		if err := copyTx.QueryRow(ctx, "SELECT to_jsonb(e)=$2::jsonb FROM archive.authentication_events e WHERE id=$1", e.id, string(e.data)).Scan(&identical); err != nil {
			return Result{}, false, err
		}
		if !identical {
			return Result{}, false, errors.New("conflicting cold audit history")
		}
	}
	// Confirm cold commit before hot deletion. An ambiguous commit retains hot
	// data; matching id and all original fields make retries safe.
	if err := copyTx.Commit(ctx); err != nil {
		return Result{}, false, err
	}
	for _, e := range events {
		if _, err := hot.Exec(ctx, "DELETE FROM auth.authentication_events WHERE id=$1", e.id); err != nil {
			return Result{}, false, err
		}
	}
	result.Archived = int64(len(events))
	return result, false, nil
}
