package querycontext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrChanged = errors.New("complete query projection changed")

type Page struct{ Number, Size int }
type Observation[T any] struct {
	Items       T
	Total       int64
	Fingerprint string
}
type Result[T any] struct {
	Items    T
	Total    int64
	Version  string
	Criteria json.RawMessage
}
type Strategy[T any] interface {
	Resource() string
	OpenRead(context.Context) (pgx.Tx, error)
	Prepare(context.Context, pgx.Tx, json.RawMessage, json.RawMessage) (json.RawMessage, json.RawMessage, error)
	Revisions(context.Context, pgx.Tx) (json.RawMessage, error)
	Observe(context.Context, pgx.Tx, json.RawMessage, Page) (Observation[T], error)
	Page(context.Context, pgx.Tx, json.RawMessage, Page) (T, error)
}
type Receipt struct {
	store      *Store
	sessionRef string
	token      string
	saved      Metadata
	current    json.RawMessage
}

// Revision returns the snapshot vector verified before the caller's write
// transaction. The caller must compare it again under its own revision lock.
func (r Receipt) Revision() json.RawMessage {
	return append(json.RawMessage(nil), r.current...)
}

func validObservation[T any](o Observation[T]) bool {
	return o.Total >= 0 && o.Total <= 9007199254740991 && validFingerprint(o.Fingerprint)
}

// Execute owns the common old-baseline, revision, full-observation and
// page-only lifecycle. A domain Strategy alone supplies Session access,
// canonical criteria, revision reads and authorized SQL projection.
func Execute[T any](ctx context.Context, store *Store, sessionRef, version string, incoming json.RawMessage, page Page, strategy Strategy[T]) (Result[T], error) {
	var result Result[T]
	if store == nil || strategy == nil || page.Number < 1 || page.Size < 1 {
		return result, ErrInvalid
	}
	var saved Metadata
	var loadErr error
	if version != "" {
		saved, loadErr = store.Load(ctx, sessionRef, version)
	}
	// Live authorization must win over an expired or invalid token.
	tx, err := strategy.OpenRead(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	if loadErr != nil {
		return result, loadErr
	}
	if version != "" && saved.View != strategy.Resource() {
		return result, ErrExpired
	}
	var savedCriteria json.RawMessage
	if version != "" {
		savedCriteria = saved.Criteria
	}
	old, current, err := strategy.Prepare(ctx, tx, savedCriteria, incoming)
	if err != nil {
		return result, err
	}
	if len(current) == 0 || version != "" && len(old) == 0 {
		return result, ErrInvalid
	}
	revision, err := strategy.Revisions(ctx, tx)
	if err != nil {
		return result, err
	}
	if store.policy.Forward == nil || !store.policy.Forward(strategy.Resource(), revision, revision) {
		return result, ErrInvalid
	}
	same := version != "" && bytes.Equal(saved.Criteria, current)
	var verified *Observation[T]
	if version != "" && !bytes.Equal(saved.Revision, revision) {
		if !store.policy.Forward(strategy.Resource(), saved.Revision, revision) {
			return result, ErrExpired
		}
		validationPage := Page{Number: 1, Size: 1}
		if same {
			validationPage = page
		}
		observed, e := strategy.Observe(ctx, tx, old, validationPage)
		if e != nil {
			return result, e
		}
		if !validObservation(observed) {
			return result, ErrInvalid
		}
		if observed.Fingerprint != saved.Fingerprint || observed.Total != saved.Total {
			return result, ErrChanged
		}
		verified = &observed
	}
	var projection Observation[T]
	if !same {
		projection, err = strategy.Observe(ctx, tx, current, page)
		if err != nil {
			return result, err
		}
		if !validObservation(projection) {
			return result, ErrInvalid
		}
	} else if verified != nil {
		projection = *verified
	} else {
		projection.Total = saved.Total
		projection.Fingerprint = saved.Fingerprint
		projection.Items, err = strategy.Page(ctx, tx, current, page)
		if err != nil {
			return result, err
		}
	}
	// No Redis publication may occur until the complete authorized RR read
	// has committed. Redis CAS never overwrites the immutable baseline.
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	if version != "" && !bytes.Equal(saved.Revision, revision) {
		err = store.Advance(ctx, sessionRef, version, saved.Fingerprint, saved.Revision, revision)
		if err != nil && !errors.Is(err, ErrCAS) {
			return result, err
		}
	}
	if !same {
		version, err = store.Create(ctx, sessionRef, Metadata{
			View: strategy.Resource(), Criteria: current, Total: projection.Total,
			Fingerprint: projection.Fingerprint, ProtocolVersion: 1, Revision: revision,
		})
		if err != nil {
			return result, err
		}
	}
	result.Items, result.Total, result.Version, result.Criteria = projection.Items, projection.Total, version, current
	return result, nil
}

// ValidateSavedRead leaves the same authorized RR transaction open for a
// caller's option/catalog reads. The Receipt commits it before Redis CAS.
func ValidateSavedRead[T any](ctx context.Context, store *Store, sessionRef, version string, strategy Strategy[T]) (pgx.Tx, Receipt, error) {
	var receipt Receipt
	if strategy == nil || version != "" && store == nil {
		return nil, receipt, ErrInvalid
	}
	var saved Metadata
	var loadErr error
	if version != "" {
		saved, loadErr = store.Load(ctx, sessionRef, version)
	}
	tx, err := strategy.OpenRead(ctx)
	if err != nil {
		return nil, receipt, err
	}
	fail := func(e error) (pgx.Tx, Receipt, error) {
		_ = tx.Rollback(context.Background())
		return nil, Receipt{}, e
	}
	if loadErr != nil {
		return fail(loadErr)
	}
	receipt = Receipt{store: store, sessionRef: sessionRef, token: version, saved: saved}
	if version == "" {
		return tx, receipt, nil
	}
	if saved.View != strategy.Resource() {
		return fail(ErrExpired)
	}
	old, _, err := strategy.Prepare(ctx, tx, saved.Criteria, nil)
	if err != nil {
		return fail(err)
	}
	if len(old) == 0 {
		return fail(ErrInvalid)
	}
	revision, err := strategy.Revisions(ctx, tx)
	if err != nil {
		return fail(err)
	}
	if store.policy.Forward == nil || !store.policy.Forward(strategy.Resource(), revision, revision) {
		return fail(ErrInvalid)
	}
	if !bytes.Equal(saved.Revision, revision) {
		if !store.policy.Forward(strategy.Resource(), saved.Revision, revision) {
			return fail(ErrExpired)
		}
		observed, e := strategy.Observe(ctx, tx, old, Page{Number: 1, Size: 1})
		if e != nil {
			return fail(e)
		}
		if !validObservation(observed) {
			return fail(ErrInvalid)
		}
		if observed.Fingerprint != saved.Fingerprint || observed.Total != saved.Total {
			return fail(ErrChanged)
		}
	}
	receipt.current = revision
	return tx, receipt, nil
}

func (r Receipt) Commit(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return ErrInvalid
	}
	defer tx.Rollback(context.Background())
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if r.token == "" || bytes.Equal(r.saved.Revision, r.current) {
		return nil
	}
	err := r.store.Advance(ctx, r.sessionRef, r.token, r.saved.Fingerprint, r.saved.Revision, r.current)
	if errors.Is(err, ErrCAS) {
		return nil
	}
	return err
}
