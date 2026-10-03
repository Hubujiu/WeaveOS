// Package apprecords applies already-normalized values to a typed PostgreSQL
// business table. All methods use caller-owned transactions and fail closed
// when the live table gate, authorization, fence or audit port is absent.
package apprecords

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrInvalid = errors.New("invalid record input")
var ErrUnavailable = errors.New("record dependency unavailable")
var ErrConflict = errors.New("record version conflict")
var ErrNotReady = errors.New("application schema not ready")
var ErrMissing = errors.New("record not found")
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var namespace = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

const maxVersion int64 = 9007199254740991

type MutationResult struct {
	OperationID   string    `json:"operationId"`
	ID            string    `json:"id"`
	RecordVersion int64     `json:"recordVersion"`
	SchemaVersion int64     `json:"schemaVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type Table struct {
	AppID, TableID, ViewID, Namespace string
	SchemaVersion                     int64
	Ready                             bool
	// ActiveFieldIDs are trusted V030-013 metadata, not request keys.
	ActiveFieldIDs []string
}
type Create struct {
	OperationID, ID, ActorID string
	ExpectedSchemaVersion    int64
	Values                   map[string]any
}
type Edit struct {
	OperationID, ID, ActorID                     string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Changes                                      map[string]any
}
type Gate interface {
	// LockTable must acquire V030-013's real same-table gate and recheck the
	// current schema version/ready state under that gate before row access.
	LockTable(context.Context, pgx.Tx, string, int64) error
}
type Authorization interface {
	Check(context.Context, pgx.Tx, string, string, []string) error
}
type Fence interface {
	Check(context.Context, pgx.Tx, string, string, int64) error
}
type Audit interface {
	Append(context.Context, pgx.Tx, MutationResult, string, []string) error
}
type Writer struct {
	Gate          Gate
	Authorization Authorization
	Fence         Fence
	Audit         Audit
}

func (w Writer) table(table Table, expected int64) (string, map[string]string, error) {
	if !table.Ready {
		return "", nil, ErrNotReady
	}
	if !uuid.MatchString(table.AppID) || !uuid.MatchString(table.TableID) || !uuid.MatchString(table.ViewID) || !namespace.MatchString(table.Namespace) || table.SchemaVersion < 1 || table.SchemaVersion > maxVersion || expected < 1 || expected > maxVersion {
		return "", nil, ErrInvalid
	}
	if expected != table.SchemaVersion {
		return "", nil, ErrConflict
	}
	if w.Gate == nil || w.Authorization == nil || w.Fence == nil || w.Audit == nil {
		return "", nil, ErrUnavailable
	}
	fields := make(map[string]string, len(table.ActiveFieldIDs))
	for _, id := range table.ActiveFieldIDs {
		if !uuid.MatchString(id) || fields[id] != "" {
			return "", nil, ErrInvalid
		}
		fields[id] = pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
	}
	return pgx.Identifier{table.Namespace, "t_" + strings.ReplaceAll(table.TableID, "-", "")}.Sanitize(), fields, nil
}
func values(input map[string]any, active map[string]string) ([]string, []string, []any, error) {
	ids := make([]string, 0, len(input))
	for id := range input {
		if active[id] == "" {
			return nil, nil, nil, ErrInvalid
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cols := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		cols = append(cols, active[id])
		args = append(args, input[id])
	}
	return ids, cols, args, nil
}
func validWrite(id, op, actor string) bool {
	return uuid.MatchString(id) && uuid.MatchString(op) && uuid.MatchString(actor)
}
func (w Writer) CreateInTx(ctx context.Context, tx pgx.Tx, table Table, in Create) (MutationResult, error) {
	var result MutationResult
	name, fields, err := w.table(table, in.ExpectedSchemaVersion)
	if err != nil {
		return result, err
	}
	if tx == nil || !validWrite(in.ID, in.OperationID, in.ActorID) || in.Values == nil {
		return result, ErrInvalid
	}
	ids, cols, args, err := values(in.Values, fields)
	if err != nil {
		return result, err
	}
	if err = w.Gate.LockTable(ctx, tx, table.TableID, table.SchemaVersion); err != nil {
		return result, err
	}
	if err = w.Authorization.Check(ctx, tx, "data.create", in.ActorID, ids); err != nil {
		return result, err
	}
	allCols := append([]string{"id", "created_by"}, cols...)
	placeholders := make([]string, len(allCols))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	allArgs := append([]any{in.ID, in.ActorID}, args...)
	query := `INSERT INTO ` + name + `(` + strings.Join(allCols, ",") + `) VALUES(` + strings.Join(placeholders, ",") + `) RETURNING id::text,record_version,created_at,updated_at`
	err = tx.QueryRow(ctx, query, allArgs...).Scan(&result.ID, &result.RecordVersion, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return MutationResult{}, err
	}
	if !version(result.RecordVersion) || result.CreatedAt.IsZero() || result.UpdatedAt.IsZero() {
		return MutationResult{}, ErrUnavailable
	}
	result.OperationID = in.OperationID
	result.SchemaVersion = table.SchemaVersion
	if err = w.Audit.Append(ctx, tx, result, "create", ids); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}

// EditInTx takes the same physical table gate before row lock/CAS/fence.
func (w Writer) EditInTx(ctx context.Context, tx pgx.Tx, table Table, in Edit) (MutationResult, error) {
	var result MutationResult
	name, fields, err := w.table(table, in.ExpectedSchemaVersion)
	if err != nil {
		return result, err
	}
	if tx == nil || !validWrite(in.ID, in.OperationID, in.ActorID) || !version(in.ExpectedRecordVersion) || in.Changes == nil {
		return result, ErrInvalid
	}
	ids, cols, args, err := values(in.Changes, fields)
	if err != nil {
		return result, err
	}
	if err = w.Gate.LockTable(ctx, tx, table.TableID, table.SchemaVersion); err != nil {
		return result, err
	}
	var createdBy string
	err = tx.QueryRow(ctx, `SELECT created_by::text,record_version,created_at,updated_at FROM `+name+` WHERE id=$1 FOR UPDATE`, in.ID).Scan(&createdBy, &result.RecordVersion, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MutationResult{}, ErrMissing
	}
	if err != nil {
		return MutationResult{}, err
	}
	if result.RecordVersion != in.ExpectedRecordVersion {
		return MutationResult{}, ErrConflict
	}
	if len(ids) > 0 && result.RecordVersion == maxVersion {
		return MutationResult{}, ErrConflict
	}
	if err = w.Fence.Check(ctx, tx, table.TableID, in.ID, in.ExpectedRecordVersion); err != nil {
		return MutationResult{}, err
	}
	if err = w.Authorization.Check(ctx, tx, "data.edit", createdBy, ids); err != nil {
		return MutationResult{}, err
	}
	result.ID = in.ID
	result.OperationID = in.OperationID
	result.SchemaVersion = table.SchemaVersion
	if len(ids) > 0 {
		sets := make([]string, 0, len(ids)+2)
		for i, col := range cols {
			sets = append(sets, fmt.Sprintf("%s=$%d", col, i+1))
		}
		sets = append(sets, "record_version=record_version+1", "updated_at=now()")
		args = append(args, in.ID, in.ExpectedRecordVersion)
		query := `UPDATE ` + name + ` SET ` + strings.Join(sets, ",") + fmt.Sprintf(` WHERE id=$%d AND record_version=$%d RETURNING record_version,updated_at`, len(args)-1, len(args))
		err = tx.QueryRow(ctx, query, args...).Scan(&result.RecordVersion, &result.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return MutationResult{}, ErrConflict
		}
		if err != nil {
			return MutationResult{}, err
		}
	}
	if err = w.Audit.Append(ctx, tx, result, "edit", ids); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}
func version(v int64) bool { return v >= 1 && v <= maxVersion }
