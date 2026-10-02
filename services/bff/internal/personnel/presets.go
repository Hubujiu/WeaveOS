package personnel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrPresetName = errors.New("personal table preset name conflict")
var ErrPresetLimit = errors.New("personal table preset limit reached")
var ErrPresetConflict = errors.New("personal table preset version conflict")
var ErrPresetSchema = errors.New("unsupported personal table preset schema")

const presetResultColumns = `id::text,view_key,name,filter_json,hidden_column_ids::text,schema_version,version,created_at,updated_at`

func scanPreset(row pgx.Row) (TablePreset, error) {
	var p TablePreset
	var filter *string
	var hidden string
	e := row.Scan(&p.ID, &p.View, &p.Name, &filter, &hidden, &p.SchemaVersion, &p.Version, &p.CreatedAt, &p.UpdatedAt)
	if e != nil {
		return p, e
	}
	p.Filter = json.RawMessage("null")
	if filter != nil {
		p.Filter = json.RawMessage(*filter)
	}
	if json.Unmarshal([]byte(hidden), &p.HiddenColumnIDs) != nil {
		return p, session.ErrUnavailable
	}
	return p, nil
}
func presetStorageError(e error) error {
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" && pg.ConstraintName == "uq_table_presets_owner_view_name" {
		return ErrPresetName
	}
	return databaseError(e)
}
func presetFilterValue(raw json.RawMessage) any {
	if string(raw) == "null" {
		return nil
	}
	return string(raw)
}
func ownedPreset(ctx context.Context, tx pgx.Tx, owner, id string, lock bool) (TablePreset, error) {
	sql := `SELECT ` + presetResultColumns + ` FROM personnel.table_presets WHERE id=$1 AND owner_id=$2`
	if lock {
		sql += ` FOR UPDATE`
	}
	p, e := scanPreset(tx.QueryRow(ctx, sql, id, owner))
	if errors.Is(e, pgx.ErrNoRows) {
		e = ErrMissing
	}
	return p, e
}
func (a *Application) CreatePreset(ctx context.Context, p session.Principal, in PresetCreateInput) (TablePreset, error) {
	name, filter, e := canonicalPreset(in.View, in.Name, in.Filter, in.HiddenColumnIDs, in.SchemaVersion)
	if e != nil {
		return TablePreset{}, e
	}
	tx, e := a.write(ctx, p)
	if e != nil {
		return TablePreset{}, e
	}
	defer tx.Rollback(context.Background())
	if e = validatePresetReferences(ctx, tx, filter); e != nil {
		return TablePreset{}, e
	}
	hidden, _ := json.Marshal(in.HiddenColumnIDs)
	// Reuse the approved draft slot allocation: RC sees committed winners;
	// owner/view slot uniqueness enforces the quota without a global lock.
	for range 20 {
		result, e := scanPreset(tx.QueryRow(ctx, `INSERT INTO personnel.table_presets(owner_id,view_key,name,slot,filter_json,hidden_column_ids,schema_version)
   SELECT $1,$2,$3,s,$4,$5::jsonb,$6 FROM generate_series(1,20) s
   WHERE NOT EXISTS(SELECT 1 FROM personnel.table_presets WHERE owner_id=$1 AND view_key=$2 AND slot=s)
   ORDER BY s LIMIT 1 ON CONFLICT(owner_id,view_key,slot) DO NOTHING RETURNING `+presetResultColumns, p.UserID, in.View, name, presetFilterValue(filter), string(hidden), in.SchemaVersion))
		if e == nil {
			return result, tx.Commit(ctx)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return TablePreset{}, presetStorageError(e)
		}
		var count int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM personnel.table_presets WHERE owner_id=$1 AND view_key=$2`, p.UserID, in.View).Scan(&count); e != nil {
			return TablePreset{}, e
		}
		if count == 20 {
			return TablePreset{}, ErrPresetLimit
		}
	}
	return TablePreset{}, session.ErrUnavailable
}
func (a *Application) ListPresets(ctx context.Context, p session.Principal, view string) (TablePresetList, error) {
	result := TablePresetList{Items: []TablePreset{}}
	if !validPresetView(view) {
		return result, ErrInvalid
	}
	tx, e := a.read(ctx, p)
	if e != nil {
		return result, e
	}
	defer tx.Rollback(context.Background())
	rows, e := tx.Query(ctx, `SELECT `+presetResultColumns+` FROM personnel.table_presets WHERE owner_id=$1 AND view_key=$2 ORDER BY updated_at DESC,id ASC LIMIT 20`, p.UserID, view)
	if e != nil {
		return result, e
	}
	defer rows.Close()
	for rows.Next() {
		preset, e := scanPreset(rows)
		if e != nil {
			return result, e
		}
		result.Items = append(result.Items, preset)
	}
	if e = rows.Err(); e != nil {
		return result, e
	}
	return result, tx.Commit(ctx)
}
func (a *Application) GetPreset(ctx context.Context, p session.Principal, id string) (TablePreset, error) {
	if !validID(id) {
		return TablePreset{}, ErrInvalid
	}
	tx, e := a.read(ctx, p)
	if e != nil {
		return TablePreset{}, e
	}
	defer tx.Rollback(context.Background())
	result, e := ownedPreset(ctx, tx, p.UserID, id, false)
	if e != nil {
		return result, e
	}
	return result, tx.Commit(ctx)
}
func (a *Application) UpdatePreset(ctx context.Context, p session.Principal, id string, in PresetUpdateInput) (TablePreset, error) {
	if !validID(id) || in.Version < 1 || in.Version > maxSafeVersion {
		return TablePreset{}, ErrInvalid
	}
	tx, e := a.write(ctx, p)
	if e != nil {
		return TablePreset{}, e
	}
	defer tx.Rollback(context.Background())
	old, e := ownedPreset(ctx, tx, p.UserID, id, true)
	if e != nil {
		return TablePreset{}, e
	}
	if old.Version != in.Version || old.Version == maxSafeVersion {
		return TablePreset{}, ErrPresetConflict
	}
	name, filter, e := canonicalPreset(old.View, in.Name, in.Filter, in.HiddenColumnIDs, in.SchemaVersion)
	if e != nil {
		return TablePreset{}, e
	}
	if e = validatePresetReferences(ctx, tx, filter); e != nil {
		return TablePreset{}, e
	}
	hidden, _ := json.Marshal(in.HiddenColumnIDs)
	result, e := scanPreset(tx.QueryRow(ctx, `UPDATE personnel.table_presets SET name=$4,filter_json=$5,hidden_column_ids=$6::jsonb,schema_version=$7,version=version+1,updated_at=clock_timestamp()
  WHERE id=$1 AND owner_id=$2 AND version=$3 RETURNING `+presetResultColumns, id, p.UserID, in.Version, name, presetFilterValue(filter), string(hidden), in.SchemaVersion))
	if errors.Is(e, pgx.ErrNoRows) {
		return TablePreset{}, ErrPresetConflict
	}
	if e != nil {
		return TablePreset{}, presetStorageError(e)
	}
	return result, tx.Commit(ctx)
}
func (a *Application) DeletePreset(ctx context.Context, p session.Principal, id string, version int64) error {
	if !validID(id) || version < 1 || version > maxSafeVersion {
		return ErrInvalid
	}
	tx, e := a.write(ctx, p)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	old, e := ownedPreset(ctx, tx, p.UserID, id, true)
	if e != nil {
		return e
	}
	if old.Version != version {
		return ErrPresetConflict
	}
	tag, e := tx.Exec(ctx, `DELETE FROM personnel.table_presets WHERE id=$1 AND owner_id=$2 AND version=$3`, id, p.UserID, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrPresetConflict
	}
	return tx.Commit(ctx)
}
