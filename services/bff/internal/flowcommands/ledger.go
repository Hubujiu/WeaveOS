package flowcommands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrMissing = errors.New("workflow command not found")

type Entry struct {
	Command Command
	State   string
	Receipt *Receipt
}
type Ledger struct{ Namespace string }

func (ledger Ledger) AcceptInTx(ctx context.Context, tx pgx.Tx, command Command) (Entry, error) {
	return ledger.acceptInTx(ctx, tx, command, nil)
}

func (ledger Ledger) acceptInTx(ctx context.Context, tx pgx.Tx, command Command, executionPayload []byte) (Entry, error) {
	if !ledger.validPort(ctx, tx) {
		return Entry{}, ErrInvalid
	}
	commandHash, err := Fingerprint(command)
	if err != nil {
		return Entry{}, err
	}
	commandJSON, err := json.Marshal(command)
	if err != nil {
		return Entry{}, ErrInvalid
	}

	commandsTable, dispatchTable := ledger.tables()
	query := fmt.Sprintf(`
		INSERT INTO %s (command_id, command_json, command_hash, state, receipt_json)
		VALUES ($1, $2, $3, 'pending', NULL)
		ON CONFLICT (command_id) DO NOTHING`, commandsTable)
	args := []any{command.CommandID, string(commandJSON), commandHash[:]}
	if executionPayload != nil {
		query = fmt.Sprintf(`
			INSERT INTO %s (command_id, command_json, command_hash, state, receipt_json, execution_payload)
			VALUES ($1, $2, $3, 'pending', NULL, $4)
			ON CONFLICT (command_id) DO NOTHING`, commandsTable)
		args = append(args, executionPayload)
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return Entry{}, err
	}
	if tag.RowsAffected() == 1 {
		query = fmt.Sprintf(`INSERT INTO %s (command_id) VALUES ($1)`, dispatchTable)
		if executionPayload != nil {
			query = fmt.Sprintf(`INSERT INTO %s (command_id, protocol_version) VALUES ($1, 2)`, dispatchTable)
		}
		if _, err := tx.Exec(ctx, query, command.CommandID); err != nil {
			return Entry{}, err
		}
		return Entry{Command: command, State: "pending"}, nil
	}
	if tag.RowsAffected() != 0 {
		return Entry{}, ErrConflict
	}

	existing, storedHash, err := ledger.selectEntry(ctx, tx, command.CommandID, false)
	if err != nil {
		return Entry{}, err
	}
	if existing.Command != command || !hashesEqual(storedHash, commandHash) {
		return Entry{}, ErrConflict
	}
	if executionPayload != nil {
		storedPayload, err := ledger.executionPayloadBytesInTx(ctx, tx, command)
		if err != nil {
			return Entry{}, err
		}
		if !bytes.Equal(storedPayload, executionPayload) {
			return Entry{}, ErrConflict
		}
	}
	return existing, nil
}

func (ledger Ledger) GetInTx(ctx context.Context, tx pgx.Tx, commandID string) (Entry, error) {
	if !ledger.validPort(ctx, tx) || !isCanonicalNonzeroUUID(commandID) {
		return Entry{}, ErrInvalid
	}
	entry, _, err := ledger.selectEntry(ctx, tx, commandID, false)
	return entry, err
}

func (ledger Ledger) ApplyInTx(
	ctx context.Context,
	tx pgx.Tx,
	command Command,
	receipt Receipt,
	currentSequence int64,
	apply func(context.Context, pgx.Tx, ApplyPlan) error,
) (Entry, error) {
	if !ledger.validPort(ctx, tx) || apply == nil {
		return Entry{}, ErrInvalid
	}
	if _, err := Fingerprint(command); err != nil {
		return Entry{}, err
	}
	entry, _, err := ledger.selectEntry(ctx, tx, command.CommandID, true)
	if err != nil {
		return Entry{}, err
	}
	if entry.Command != command {
		return Entry{}, ErrConflict
	}

	if entry.State != "pending" {
		if entry.Receipt == nil {
			return Entry{}, ErrInvalid
		}
		plan, err := PlanReceipt(command, receipt, currentSequence, entry.Receipt)
		if err != nil {
			return Entry{}, err
		}
		if !plan.Duplicate {
			return Entry{}, ErrConflict
		}
		return entry, nil
	}

	if entry.Receipt != nil {
		return Entry{}, ErrInvalid
	}
	plan, err := PlanReceipt(command, receipt, currentSequence, nil)
	if err != nil {
		return Entry{}, err
	}
	if err := apply(ctx, tx, plan); err != nil {
		return Entry{}, err
	}

	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		return Entry{}, ErrInvalid
	}
	commandsTable, dispatchTable := ledger.tables()
	query := fmt.Sprintf(`
		UPDATE %s
		SET state = $2, receipt_json = $3
		WHERE command_id = $1 AND state = 'pending' AND receipt_json IS NULL`, commandsTable)
	tag, err := tx.Exec(ctx, query, command.CommandID, receipt.Outcome, string(receiptJSON))
	if err != nil {
		return Entry{}, err
	}
	if tag.RowsAffected() != 1 {
		return Entry{}, ErrConflict
	}

	query = fmt.Sprintf(`DELETE FROM %s WHERE command_id = $1`, dispatchTable)
	tag, err = tx.Exec(ctx, query, command.CommandID)
	if err != nil {
		return Entry{}, err
	}
	if tag.RowsAffected() != 1 {
		return Entry{}, ErrConflict
	}

	storedReceipt := receipt
	return Entry{Command: command, State: receipt.Outcome, Receipt: &storedReceipt}, nil
}

func (ledger Ledger) validPort(ctx context.Context, tx pgx.Tx) bool {
	return ctx != nil && tx != nil && strings.TrimSpace(ledger.Namespace) != ""
}

func (ledger Ledger) tables() (string, string) {
	return pgx.Identifier{ledger.Namespace, "workflow_commands"}.Sanitize(),
		pgx.Identifier{ledger.Namespace, "workflow_dispatch"}.Sanitize()
}

func (ledger Ledger) selectEntry(ctx context.Context, tx pgx.Tx, commandID string, forUpdate bool) (Entry, [32]byte, error) {
	commandsTable, _ := ledger.tables()
	query := fmt.Sprintf(`
		SELECT command_id::text, command_json, command_hash, state, receipt_json
		FROM %s WHERE command_id = $1`, commandsTable)
	if forUpdate {
		query += " FOR UPDATE"
	}

	var storedCommandID string
	var commandJSON []byte
	var commandHash []byte
	var state string
	var receiptJSON []byte
	err := tx.QueryRow(ctx, query, commandID).Scan(
		&storedCommandID,
		&commandJSON,
		&commandHash,
		&state,
		&receiptJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, [32]byte{}, ErrMissing
	}
	if err != nil {
		return Entry{}, [32]byte{}, err
	}
	if storedCommandID != commandID {
		return Entry{}, [32]byte{}, ErrConflict
	}
	return decodeStoredEntry(storedCommandID, commandJSON, commandHash, state, receiptJSON)
}

func decodeStoredEntry(storedCommandID string, commandJSON, commandHash []byte, state string, receiptJSON []byte) (Entry, [32]byte, error) {
	if !isCanonicalNonzeroUUID(storedCommandID) || len(commandHash) != sha256.Size {
		return Entry{}, [32]byte{}, ErrConflict
	}

	var command Command
	if err := json.Unmarshal(commandJSON, &command); err != nil {
		return Entry{}, [32]byte{}, fmt.Errorf("decode stored workflow command: %w", ErrInvalid)
	}
	computedHash, err := Fingerprint(command)
	if err != nil || !hashBytesEqual(commandHash, computedHash) {
		return Entry{}, [32]byte{}, ErrConflict
	}
	if command.CommandID != storedCommandID {
		return Entry{}, [32]byte{}, ErrConflict
	}

	entry := Entry{Command: command, State: state}
	switch state {
	case "pending":
		if receiptJSON != nil {
			return Entry{}, [32]byte{}, ErrInvalid
		}
	case "success", "no_effect":
		if receiptJSON == nil {
			return Entry{}, [32]byte{}, ErrInvalid
		}
		var receipt Receipt
		if err := json.Unmarshal(receiptJSON, &receipt); err != nil {
			return Entry{}, [32]byte{}, fmt.Errorf("decode stored workflow receipt: %w", ErrInvalid)
		}
		if _, err := PlanReceipt(command, receipt, command.ExpectedSequence, &receipt); err != nil {
			return Entry{}, [32]byte{}, fmt.Errorf("validate stored workflow receipt: %w", err)
		}
		if receipt.Outcome != state {
			return Entry{}, [32]byte{}, ErrConflict
		}
		storedReceipt := receipt
		entry.Receipt = &storedReceipt
	default:
		return Entry{}, [32]byte{}, ErrInvalid
	}
	return entry, computedHash, nil
}

func hashesEqual(left, right [32]byte) bool {
	return left == right
}

func hashBytesEqual(leftBytes []byte, right [32]byte) bool {
	if len(leftBytes) != len(right) {
		return false
	}
	for index := range right {
		if leftBytes[index] != right[index] {
			return false
		}
	}
	return true
}
