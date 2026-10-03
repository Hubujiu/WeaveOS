package personnel

import (
	"context"
	"encoding/json"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// The caller owns the returned transaction and executes its business mutation
// exactly once. Only prevalidation may repeat; executed or ambiguous commits
// are never retried here. HTTP table writes separately require a context.
func (a *Application) BeginQueryWrite(ctx context.Context, p session.Principal, version string) (pgx.Tx, error) {
	for attempt := 0; attempt < 3; attempt++ {
		var verified QueryRevisions
		if version != "" {
			read, receipt, err := a.readWithQuery(ctx, p, version)
			if err != nil {
				return nil, err
			}
			verified = receipt.revisions
			if err = receipt.commit(ctx, read); err != nil {
				return nil, err
			}
		}
		if a == nil || a.Pool == nil {
			return nil, session.ErrUnavailable
		}
		tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err == nil {
			err = a.AuthorizeWrite(ctx, tx, p)
		}
		if err != nil {
			_ = tx.Rollback(context.Background())
			return nil, databaseError(err)
		}
		if version != "" {
			current, e := readQueryRevisions(ctx, tx, "members")
			if e != nil {
				_ = tx.Rollback(context.Background())
				return nil, e
			}
			if current != verified {
				_ = tx.Rollback(context.Background())
				continue
			}
		}
		return tx, nil
	}
	return nil, ErrQueryBusy
}

type queryReadReceipt struct {
	shared    querycontext.Receipt
	revisions QueryRevisions
}

// Candidate option reads use this same RR snapshot; their own parameters never
// replace the original member query. Advance only after that read commits.
func (a *Application) readWithQuery(ctx context.Context, p session.Principal, version string) (pgx.Tx, queryReadReceipt, error) {
	// Preserve the old auth-before-unavailable error priority when the optional
	// context store is absent. No-token reads never need that store.
	if version != "" && (a == nil || a.Queries == nil) {
		tx, err := a.read(ctx, p)
		if err != nil {
			return nil, queryReadReceipt{}, err
		}
		_ = tx.Rollback(context.Background())
		return nil, queryReadReceipt{}, session.ErrUnavailable
	}
	var store *querycontext.Store
	if a != nil && a.Queries != nil {
		store = a.Queries.shared
	}
	tx, shared, err := querycontext.ValidateSavedRead(ctx, store, p.SessionRef, version,
		personnelQueryStrategy{app: a, principal: p, view: "members"})
	if err != nil {
		return nil, queryReadReceipt{}, personnelContextError(err)
	}
	receipt := queryReadReceipt{shared: shared}
	if version != "" && json.Unmarshal(shared.Revision(), &receipt.revisions) != nil {
		_ = tx.Rollback(context.Background())
		return nil, queryReadReceipt{}, ErrInvalid
	}
	return tx, receipt, nil
}
func (v queryReadReceipt) commit(ctx context.Context, tx pgx.Tx) error {
	return personnelContextError(v.shared.Commit(ctx, tx))
}

func draftTarget(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
func commitBusiness(ctx context.Context, tx pgx.Tx, p session.Principal, meta RequestMetadata, kind DraftKind, target *string) error {
	if _, err := CleanupDraft(ctx, tx, p, meta.DraftRef, kind, target); err != nil {
		return err
	}
	return databaseError(tx.Commit(ctx))
}
