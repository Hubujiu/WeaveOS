package workflowcatalog

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// DeletionInput is internal; the caller must hold current management authority.
type DeletionInput struct {
	AppID, ViewID, FlowID, ActorID, OperationID string
	ExpectedRevision                            int64
}

type Deletion struct {
	AppID, TableID, ViewID, FlowID, ActorID, OperationID, FlowName, Status string
	ExpectedRevision                                                       int64
	Reason                                                                 *string
	DeletedVersions                                                        *int64
	DeletedAt, CompletedAt                                                 *time.Time
}

// Compilation-only declaration for R1 RED. No acceptance or mutation exists yet.
func (Catalog) RequestDeletionInTx(ctx context.Context, tx pgx.Tx, in DeletionInput) (Deletion, error) {
	return Deletion{}, ErrNotReady
}
