package flowcommands

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// GetManyInTx is the bounded read port; behavior is deliberately absent for RED.
func (ledger Ledger) GetManyInTx(ctx context.Context, tx pgx.Tx, ids []string) (map[string]Entry, error) {
	return nil, ErrInvalid
}
