package appstructure

import (
	"context"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

func checkWorkflowCompatibility(ctx context.Context, tx pgx.Tx, appID, tableID string, proposed []appfields.Field) ([]WorkflowConflict, error) {
	fields := make([]appquery.Field, len(proposed))
	for i, field := range proposed {
		fields[i] = appquery.Field{ID: field.ID, Kind: appquery.FieldKind(field.Kind)}
	}
	conflicts, err := (workflowcatalog.Catalog{}).CheckCompatibilityInTx(ctx, tx, appID, tableID, fields)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowConflict, len(conflicts))
	for i, conflict := range conflicts {
		out[i] = WorkflowConflict{
			FlowID:  conflict.FlowID,
			Version: conflict.Version,
			NodeID:  conflict.NodeID,
			FieldID: conflict.FieldID,
			Reason:  conflict.Reason,
		}
	}
	return out, nil
}
