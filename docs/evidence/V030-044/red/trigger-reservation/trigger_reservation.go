package workflowcatalog

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type TriggerReservation struct {
	Instance Instance
	Ignored  bool
}

func (Catalog) ReserveTriggeredInTx(ctx context.Context, tx pgx.Tx, in ReserveInput) (TriggerReservation, error) {
	return TriggerReservation{}, ErrNotReady
}
