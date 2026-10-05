// Package workflowprojection persists a confirmed execution result in the
// caller's transaction. It is not a user authorization or RPC entry point.
package workflowprojection

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/jackc/pgx/v5"
)

var ErrUnimplemented = errors.New("execution projection not implemented")
var ErrConflict = errors.New("execution projection conflicts with durable state")
var ErrInvalid = errors.New("invalid execution projection")

type Store struct{}
type Applied struct {
	Entry     flowcommands.Entry
	Result    flowcommands.ExecutionResult
	Duplicate bool
}

// ApplyInTx validates the original payload and result body, then uses a nested
// transaction/savepoint to make all projection writes atomic even if the caller
// mistakenly commits after an error. Success never commits the outer transaction.
// The caller must acquire any personnel authority locks before calling; this
// method does not reevaluate live identity while recovering accepted commands.
func (Store) ApplyInTx(ctx context.Context, tx pgx.Tx, command flowcommands.Command,
	payload flowcommands.ExecutionPayload, receipt flowcommands.Receipt, body []byte) (Applied, error) {
	return Applied{}, ErrUnimplemented
}
