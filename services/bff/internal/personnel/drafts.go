package personnel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

var ErrDraftConflict = errors.New("personnel draft version conflict")
var ErrDraftLimit = errors.New("personnel draft limit reached")

const draftSummaryColumns = `id::text,kind,target_id::text,base_version,draft_version,created_at,updated_at`

func scanDraftSummary(row pgx.Row) (PersonnelDraftSummary, error) {
	var d PersonnelDraftSummary
	e := row.Scan(&d.ID, &d.Kind, &d.TargetID, &d.BaseVersion, &d.Version, &d.CreatedAt, &d.UpdatedAt)
	return d, e
}
func scanDraft(row pgx.Row) (PersonnelDraft, error) {
	var d PersonnelDraft
	var raw string
	e := row.Scan(&d.ID, &d.Kind, &d.TargetID, &d.BaseVersion, &d.Version, &d.CreatedAt, &d.UpdatedAt, &raw)
	d.Payload = json.RawMessage(raw)
	return d, e
}
func ownedDraft(ctx context.Context, tx pgx.Tx, owner, id string, lock bool) (PersonnelDraft, error) {
	sql := `SELECT ` + draftSummaryColumns + `,payload_json FROM personnel.drafts WHERE id=$1 AND owner_user_id=$2`
	if lock {
		sql += ` FOR UPDATE`
	}
	d, e := scanDraft(tx.QueryRow(ctx, sql, id, owner))
	if errors.Is(e, pgx.ErrNoRows) {
		return d, ErrMissing
	}
	return d, e
}

// Draft operations use live personnel authorization without query context,
// business-target existence checks, business audit, or query revision writes.
func (a *Application) CreateDraft(ctx context.Context, p session.Principal, in DraftCreateInput) (PersonnelDraft, error) {
	payload, e := draftCreatePayload(in)
	if e != nil {
		return PersonnelDraft{}, e
	}
	tx, e := a.write(ctx, p)
	if e != nil {
		return PersonnelDraft{}, e
	}
	defer tx.Rollback(context.Background())
	// RC statements see newly committed slot winners. ON CONFLICT avoids aborting
	// this authorized transaction, and the database unique/check constraints enforce
	// the hard quota independently of application concurrency.
	for range 20 {
		d, e := scanDraft(tx.QueryRow(ctx, `INSERT INTO personnel.drafts(owner_user_id,slot,kind,target_id,base_version,payload_json)
   SELECT $1,s,$2,$3,$4,$5 FROM generate_series(1,20) s
   WHERE NOT EXISTS(SELECT 1 FROM personnel.drafts WHERE owner_user_id=$1 AND slot=s)
   ORDER BY s LIMIT 1 ON CONFLICT (owner_user_id,slot) DO NOTHING
   RETURNING `+draftSummaryColumns+`,payload_json`, p.UserID, in.Kind, in.TargetID, in.BaseVersion, string(payload)))
		if e == nil {
			return d, tx.Commit(ctx)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return PersonnelDraft{}, e
		}
		var count int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM personnel.drafts WHERE owner_user_id=$1`, p.UserID).Scan(&count); e != nil {
			return PersonnelDraft{}, e
		}
		if count == 20 {
			return PersonnelDraft{}, ErrDraftLimit
		}
	}
	// Adversarial insert/delete churn exhausted allocation, not a proven quota.
	return PersonnelDraft{}, session.ErrUnavailable
}
func (a *Application) ListDrafts(ctx context.Context, p session.Principal) (PersonnelDraftList, error) {
	result := PersonnelDraftList{Items: []PersonnelDraftSummary{}}
	tx, e := a.read(ctx, p)
	if e != nil {
		return result, e
	}
	defer tx.Rollback(context.Background())
	rows, e := tx.Query(ctx, `SELECT `+draftSummaryColumns+` FROM personnel.drafts WHERE owner_user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 20`, p.UserID)
	if e != nil {
		return result, e
	}
	defer rows.Close()
	for rows.Next() {
		d, e := scanDraftSummary(rows)
		if e != nil {
			return result, e
		}
		result.Items = append(result.Items, d)
	}
	if e = rows.Err(); e != nil {
		return result, e
	}
	return result, tx.Commit(ctx)
}
func (a *Application) GetDraft(ctx context.Context, p session.Principal, id string) (PersonnelDraft, error) {
	if !validID(id) {
		return PersonnelDraft{}, ErrInvalid
	}
	tx, e := a.read(ctx, p)
	if e != nil {
		return PersonnelDraft{}, e
	}
	defer tx.Rollback(context.Background())
	d, e := ownedDraft(ctx, tx, p.UserID, id, false)
	if e != nil {
		return d, e
	}
	return d, tx.Commit(ctx)
}
func (a *Application) UpdateDraft(ctx context.Context, p session.Principal, id string, in DraftUpdateInput) (PersonnelDraft, error) {
	if !validID(id) || in.Version < 1 || in.Version > maxSafeVersion {
		return PersonnelDraft{}, ErrInvalid
	}
	tx, e := a.write(ctx, p)
	if e != nil {
		return PersonnelDraft{}, e
	}
	defer tx.Rollback(context.Background())
	original, e := ownedDraft(ctx, tx, p.UserID, id, true)
	if e != nil {
		return PersonnelDraft{}, e
	}
	if original.Version != in.Version || original.Version == maxSafeVersion {
		return PersonnelDraft{}, ErrDraftConflict
	}
	payload, e := canonicalDraftPayload(original.Kind, in.Payload)
	if e != nil {
		return PersonnelDraft{}, e
	}
	d, e := scanDraft(tx.QueryRow(ctx, `UPDATE personnel.drafts SET payload_json=$4,draft_version=draft_version+1,updated_at=clock_timestamp()
  WHERE id=$1 AND owner_user_id=$2 AND draft_version=$3 RETURNING `+draftSummaryColumns+`,payload_json`, id, p.UserID, in.Version, string(payload)))
	if errors.Is(e, pgx.ErrNoRows) {
		return PersonnelDraft{}, ErrDraftConflict
	}
	if e != nil {
		return PersonnelDraft{}, e
	}
	return d, tx.Commit(ctx)
}
func (a *Application) DeleteDraft(ctx context.Context, p session.Principal, id string, version int64) error {
	if !validID(id) || version < 1 || version > maxSafeVersion {
		return ErrInvalid
	}
	tx, e := a.write(ctx, p)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	d, e := ownedDraft(ctx, tx, p.UserID, id, true)
	if e != nil {
		return e
	}
	if d.Version != version {
		return ErrDraftConflict
	}
	tag, e := tx.Exec(ctx, `DELETE FROM personnel.drafts WHERE id=$1 AND owner_user_id=$2 AND draft_version=$3`, id, p.UserID, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrDraftConflict
	}
	return tx.Commit(ctx)
}

// CleanupDraft is called ONLY inside the already authorized successful business
// write transaction, before commit. It performs no independent transaction,
// permission bypass, or retry. A missing/mismatched/newer reference is a no-op.
// kind and target must be derived from the actual business operation, not ref.
// For NEW objects pass nil target (the original draft target), not the new ID.
// Rollback also rolls back deletion; a later version always survives.
func CleanupDraft(ctx context.Context, tx pgx.Tx, p session.Principal, ref *DraftReference, kind DraftKind, target *string) (bool, error) {
	if ref == nil {
		return false, nil
	}
	if tx == nil || !validID(p.UserID) || !validID(ref.ID) || ref.Version < 1 || ref.Version > maxSafeVersion || !validDraftKind(kind) || target != nil && !validID(*target) {
		return false, ErrInvalid
	}
	tag, e := tx.Exec(ctx, `DELETE FROM personnel.drafts WHERE id=$1 AND owner_user_id=$2 AND draft_version=$3 AND kind=$4 AND target_id IS NOT DISTINCT FROM $5::uuid`, ref.ID, p.UserID, ref.Version, kind, target)
	if e != nil {
		return false, e
	}
	return tag.RowsAffected() == 1, nil
}
