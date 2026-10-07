package flowcommands

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
)

func (ledger Ledger) AcceptExecutionInTx(ctx context.Context, tx pgx.Tx, command Command, payload ExecutionPayload) (Entry, error) {
	if command.ProtocolVersion != 2 || nilExecutionPort(ctx) || nilExecutionPort(tx) || !ledger.validPort(ctx, tx) {
		return Entry{}, ErrInvalid
	}
	if _, err := Fingerprint(command); err != nil {
		return Entry{}, ErrInvalid
	}
	raw, err := EncodeExecutionPayload(command.Action, payload)
	if err != nil || sha256.Sum256(raw) != command.PayloadHash {
		return Entry{}, ErrInvalid
	}
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return Entry{}, err
	}
	entry, err := ledger.acceptInTx(ctx, savepoint, command, raw)
	if err == nil {
		// A nested pgx transaction commits only by releasing its savepoint.
		err = savepoint.Commit(ctx)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if rollbackErr := savepoint.Rollback(cleanup); rollbackErr != nil {
			return Entry{}, errors.Join(err, fmt.Errorf("rollback execution acceptance savepoint: %w", rollbackErr))
		}
		return Entry{}, err
	}
	return entry, nil
}

func (ledger Ledger) ExecutionPayloadInTx(ctx context.Context, tx pgx.Tx, command Command) (ExecutionPayload, error) {
	if command.ProtocolVersion != 2 || nilExecutionPort(ctx) || nilExecutionPort(tx) || !ledger.validPort(ctx, tx) {
		return ExecutionPayload{}, ErrInvalid
	}
	commandHash, err := Fingerprint(command)
	if err != nil {
		return ExecutionPayload{}, ErrInvalid
	}
	entry, storedHash, err := ledger.selectEntry(ctx, tx, command.CommandID, false)
	if err != nil {
		return ExecutionPayload{}, err
	}
	if entry.Command != command || !hashesEqual(storedHash, commandHash) {
		return ExecutionPayload{}, ErrConflict
	}
	raw, err := ledger.executionPayloadBytesInTx(ctx, tx, command)
	if err != nil {
		return ExecutionPayload{}, err
	}
	return DecodeExecutionPayload(command.Action, raw)
}

func (ledger Ledger) executionPayloadBytesInTx(ctx context.Context, tx pgx.Tx, command Command) ([]byte, error) {
	commandsTable, _ := ledger.tables()
	query := fmt.Sprintf(`SELECT execution_payload FROM %s WHERE command_id = $1`, commandsTable)
	var raw []byte
	if err := tx.QueryRow(ctx, query, command.CommandID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMissing
		}
		return nil, err
	}
	if raw == nil || sha256.Sum256(raw) != command.PayloadHash {
		return nil, ErrConflict
	}
	return raw, nil
}

func nilExecutionPort(port any) bool {
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
