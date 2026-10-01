package personnel

import (
	"context"
	"errors"

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
	app       *Application
	principal session.Principal
	version   string
	saved     QueryContext
	revisions QueryRevisions
}

// Candidate option reads use this same RR snapshot; their own parameters never
// replace the original member query. Advance only after that read commits.
func (a *Application) readWithQuery(ctx context.Context, p session.Principal, version string) (pgx.Tx, queryReadReceipt, error) {
	receipt := queryReadReceipt{app: a, principal: p, version: version}
	var loadErr error
	if version != "" {
		if a == nil || a.Queries == nil {
			loadErr = session.ErrUnavailable
		} else {
			receipt.saved, loadErr = a.Queries.Load(ctx, p.SessionRef, version)
		}
	}
	tx, err := a.read(ctx, p)
	if err != nil {
		return nil, receipt, err
	}
	fail := func(e error) (pgx.Tx, queryReadReceipt, error) {
		_ = tx.Rollback(context.Background())
		return nil, receipt, e
	}
	if loadErr != nil {
		return fail(loadErr)
	}
	if version == "" {
		return tx, receipt, nil
	}
	var criteria queryCriteria
	if receipt.saved.View != "members" || !decodeQueryMetadata(receipt.saved.Criteria, &criteria) {
		return fail(ErrQueryContextExpired)
	}
	receipt.revisions, err = readQueryRevisions(ctx, tx, "members")
	if err != nil {
		return fail(err)
	}
	if _, err = validateSavedQuery(ctx, tx, receipt.saved, receipt.revisions, criteria, PageQuery{Page: 1, PageSize: 1}); err != nil {
		return fail(err)
	}
	return tx, receipt, nil
}
func (v queryReadReceipt) commit(ctx context.Context, tx pgx.Tx) error {
	defer tx.Rollback(context.Background())
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if v.version == "" || v.saved.Revisions == v.revisions {
		return nil
	}
	err := v.app.Queries.Advance(ctx, v.principal.SessionRef, v.version, v.saved.Fingerprint, v.saved.Revisions, v.revisions)
	if errors.Is(err, ErrQueryContextCAS) {
		return nil
	}
	return err
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
