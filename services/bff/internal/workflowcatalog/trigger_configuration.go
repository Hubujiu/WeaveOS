package workflowcatalog

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/jackc/pgx/v5"
)

type Trigger struct {
	Event     string          `json:"event"`
	Condition json.RawMessage `json:"condition"`
}

// Declaration-only placeholders for the requirement-first RED run.
func NormalizeTriggers(in []Trigger, fields []appquery.Field) ([]Trigger, error) {
	return nil, ErrNotReady
}
func (Catalog) TriggersForVersionInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, version int64) ([]Trigger, error) {
	return nil, ErrNotReady
}
