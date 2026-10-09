package workflowprojection

import (
	"context"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

// finalizeRequestedClose reuses the catalog's drain check in the projection's
// transaction after the caller has acquired its application and instance locks.
func finalizeRequestedClose(ctx context.Context, tx pgx.Tx, command flowcommands.Command) error {
	catalog := workflowcatalog.Catalog{}
	head, err := catalog.GetInTx(ctx, tx, command.AppID, command.FlowID)
	if err != nil {
		return err
	}
	if head.State != "closing" {
		return nil
	}
	_, err = catalog.FinalizeCloseInTx(ctx, tx, command.AppID, command.FlowID, head.Revision)
	return err
}
