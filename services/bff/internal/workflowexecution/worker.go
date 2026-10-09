package workflowexecution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowprojection"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionClient interface {
	Lookup(context.Context, flowcommands.Command) (*workflowrpc.ExecutionConfirmed, bool, error)
	Execute(context.Context, flowcommands.Command, flowcommands.ExecutionPayload) (*workflowrpc.ExecutionConfirmed, error)
}
type Worker struct {
	AdmitStart func(context.Context) (bool, error)
	Pool       *pgxpool.Pool
	Client     ExecutionClient
	RPCTimeout time.Duration
	Limits     appschema.Limits
}

func (w *Worker) DispatchOne(ctx context.Context) (bool, error) {
	if w == nil || w.Pool == nil || nilPort(w.Client) || nilPort(ctx) ||
		w.RPCTimeout <= 0 || w.RPCTimeout > 30*time.Second ||
		w.Limits.LockTimeout < time.Millisecond || w.Limits.StatementTimeout < time.Millisecond {
		return false, flowcommands.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	claim, worked, err := w.claim(ctx)
	if err != nil {
		if worked {
			return true, w.retry(claim, "DEPENDENCY_UNAVAILABLE", err)
		}
		return false, err
	}
	if !worked {
		return false, nil
	}
	command, payload, pending, err := w.load(ctx, claim.commandID)
	if err != nil {
		reason := "DEPENDENCY_UNAVAILABLE"
		if errors.Is(err, flowcommands.ErrInvalid) || errors.Is(err, flowcommands.ErrConflict) || errors.Is(err, flowcommands.ErrMissing) {
			reason = "COMMAND_INVALID"
		}
		return true, w.retry(claim, reason, err)
	}
	if !pending {
		return true, nil
	}
	rpcContext, cancel := context.WithTimeout(ctx, w.RPCTimeout)
	defer cancel()
	confirmed, found, err := w.Client.Lookup(rpcContext, command)
	if err != nil {
		return true, w.retry(claim, "DEPENDENCY_UNAVAILABLE", err)
	}
	if found && confirmed == nil || !found && confirmed != nil {
		return true, w.retry(claim, "ENGINE_REPLY_INVALID", flowcommands.ErrInvalid)
	}
	if !found {
		if err = rpcContext.Err(); err != nil {
			return true, w.retry(claim, "DEPENDENCY_UNAVAILABLE", err)
		}
		confirmed, err = w.Client.Execute(rpcContext, command, payload)
		if err != nil {
			return true, w.retry(claim, "DEPENDENCY_UNAVAILABLE", err)
		}
	}
	if confirmed == nil || len(confirmed.ResultBytes) < 8 || len(confirmed.ResultBytes) > 65536 {
		return true, w.retry(claim, "ENGINE_REPLY_INVALID", flowcommands.ErrInvalid)
	}
	receipt := confirmed.Receipt
	body := bytes.Clone(confirmed.ResultBytes)
	if _, err = flowcommands.DecodeExecutionResult(command, receipt, body); err != nil {
		return true, w.retry(claim, "ENGINE_REPLY_INVALID", err)
	}
	err = w.transaction(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := (workflowprojection.Store{}).ApplyInTx(ctx, tx, command, payload, receipt, body)
		return err
	})
	if err != nil {
		return true, w.retry(claim, "APPLICATION_CONFIRMATION_FAILED", err)
	}
	return true, nil
}

// The partial due index selects one candidate before the short dispatch-row lock.
const claimSQL = `WITH due AS (
	SELECT d.command_id
	FROM applications.workflow_dispatch AS d
	WHERE d.protocol_version=2 AND d.next_attempt_at<=statement_timestamp()
	ORDER BY d.next_attempt_at,d.created_at,d.command_id
	LIMIT 1 FOR UPDATE OF d SKIP LOCKED
)
UPDATE applications.workflow_dispatch AS d
SET lease_token=gen_random_uuid(),
	lease_until=statement_timestamp()+interval '45 seconds',
	next_attempt_at=statement_timestamp()+interval '45 seconds',
	attempts=LEAST(d.attempts::bigint+1,2147483647)::integer
FROM due WHERE d.command_id=due.command_id
RETURNING d.command_id::text,d.lease_token::text,d.attempts`

type dispatchClaim struct {
	commandID string
	token     string
	attempts  int
}

func (w *Worker) claim(ctx context.Context) (claim dispatchClaim, worked bool, err error) {
	err = w.transaction(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, claimSQL).Scan(&claim.commandID, &claim.token, &claim.attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		worked = err == nil
		return err
	})
	return claim, worked, err
}

func (w *Worker) load(ctx context.Context, commandID string) (command flowcommands.Command, payload flowcommands.ExecutionPayload, pending bool, err error) {
	err = w.transaction(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(ctx context.Context, tx pgx.Tx) error {
		ledger := flowcommands.Ledger{Namespace: "applications"}
		entry, err := ledger.GetInTx(ctx, tx, commandID)
		if err != nil {
			return err
		}
		command = entry.Command
		if entry.State != "pending" {
			return nil
		}
		payload, err = ledger.ExecutionPayloadInTx(ctx, tx, command)
		pending = err == nil
		return err
	})
	return command, payload, pending, err
}

func (w *Worker) transaction(ctx context.Context, options pgx.TxOptions, action func(context.Context, pgx.Tx) error) (err error) {
	bounded, cancel := context.WithTimeout(ctx, w.Limits.StatementTimeout)
	defer cancel()
	tx, err := w.Pool.BeginTx(bounded, options)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if rollbackErr := tx.Rollback(cleanup); rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback execution worker transaction: %w", rollbackErr))
		}
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)`,
		fmt.Sprintf("%dms", w.Limits.LockTimeout.Milliseconds()), fmt.Sprintf("%dms", w.Limits.StatementTimeout.Milliseconds())); err != nil {
		return err
	}
	if err = action(bounded, tx); err != nil {
		return err
	}
	if err = tx.Commit(bounded); err != nil {
		return err
	}
	committed = true
	return nil
}

func (w *Worker) retry(claim dispatchClaim, reason string, original error) error {
	seconds := 60
	if claim.attempts >= 1 && claim.attempts <= 6 {
		seconds = 1 << (claim.attempts - 1)
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := w.Pool.Exec(cleanup, `UPDATE applications.workflow_dispatch
		SET next_attempt_at=statement_timestamp()+($3::bigint*interval '1 second'),
			lease_token=NULL,lease_until=NULL,last_error=$4
		WHERE command_id=$1 AND protocol_version=2 AND lease_token=$2
			AND lease_until>statement_timestamp()`, claim.commandID, claim.token, seconds, reason)
	if err != nil {
		return errors.Join(original, fmt.Errorf("reschedule execution command: %w", err))
	}
	return original
}

func nilPort(port any) bool {
	if port == nil {
		return true
	}
	value := reflect.ValueOf(port)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

var _ ExecutionClient = (*workflowrpc.ExecutionClient)(nil)
