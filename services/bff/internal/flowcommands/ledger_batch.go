package flowcommands

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// GetManyInTx verifies one bounded set using the same stored identity/receipt
// decoder as GetInTx. It returns no partial set on any missing or corrupt row.
func (ledger Ledger) GetManyInTx(ctx context.Context, tx pgx.Tx, ids []string) (map[string]Entry, error) {
	if nilExecutionPort(ctx) || nilExecutionPort(tx) || !ledger.validPort(ctx, tx) || len(ids) > 101 {
		return nil, ErrInvalid
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !isCanonicalNonzeroUUID(id) || wanted[id] {
			return nil, ErrInvalid
		}
		wanted[id] = true
	}
	out := make(map[string]Entry, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	table, _ := ledger.tables()
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT command_id::text,command_json,command_hash,state,receipt_json FROM %s WHERE command_id=ANY($1::uuid[])`, table), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, state string
		var command, hash, receipt []byte
		if err = rows.Scan(&id, &command, &hash, &state, &receipt); err != nil {
			return nil, err
		}
		if !wanted[id] {
			return nil, ErrConflict
		}
		if _, ok := out[id]; ok {
			return nil, ErrConflict
		}
		entry, _, e := decodeStoredEntry(id, command, hash, state, receipt)
		if e != nil {
			return nil, e
		}
		out[id] = entry
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) != len(ids) {
		return nil, ErrMissing
	}
	return out, nil
}
