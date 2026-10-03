// Package apprecords applies already-normalized values to a typed PostgreSQL
// business table. All methods use caller-owned transactions and fail closed
// when the live table gate, authorization, fence or audit port is absent.
package apprecords

import (
	"context"
	"errors"
	"regexp"
	"sort"
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

// TypedDML is supplied by the reviewed V013 runtime capability. It derives
// physical identifiers from registered metadata and only accepts canonical
// field values. The owner-controlled adapter must not allow arbitrary SQL,
// DDL or caller-selected system columns. No direct SQL fallback exists here.
type TypedDML interface {
	Insert(context.Context, pgx.Tx, Table, Create, []string) (StoredHeader, error)
	LockHeader(context.Context, pgx.Tx, Table, string) (StoredHeader, error)
	UpdateCAS(context.Context, pgx.Tx, Table, Edit, []string) (StoredHeader, error)
}
type StoredHeader struct {
	ID, CreatedBy        string
	RecordVersion        int64
	CreatedAt, UpdatedAt time.Time
}
type Writer struct {
	Gate          Gate
	Authorization Authorization
	Fence         Fence
	Audit         Audit
	DML           TypedDML
}

func (w Writer) table(table Table, expected int64) (map[string]bool, error) {
	if !table.Ready {
		return nil, ErrNotReady
	}
	if !uuid.MatchString(table.AppID) || !uuid.MatchString(table.TableID) || !uuid.MatchString(table.ViewID) || !namespace.MatchString(table.Namespace) || table.SchemaVersion < 1 || table.SchemaVersion > maxVersion || expected < 1 || expected > maxVersion {
		return nil, ErrInvalid
	}
	if expected != table.SchemaVersion {
		return nil, ErrConflict
	}
	if w.Gate == nil || w.Authorization == nil || w.Fence == nil || w.Audit == nil || w.DML == nil {
		return nil, ErrUnavailable
	}
	fields := make(map[string]bool, len(table.ActiveFieldIDs))
	for _, id := range table.ActiveFieldIDs {
		if !uuid.MatchString(id) || fields[id] {
			return nil, ErrInvalid
		}
		fields[id] = true
	}
	return fields, nil
}
func values(input map[string]any, active map[string]bool) ([]string, error) {
	ids := make([]string, 0, len(input))
	for id := range input {
		if !active[id] {
			return nil, ErrInvalid
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
func validWrite(id, op, actor string) bool {
	return uuid.MatchString(id) && uuid.MatchString(op) && uuid.MatchString(actor)
}
func (w Writer) CreateInTx(ctx context.Context, tx pgx.Tx, table Table, in Create) (MutationResult, error) {
	var result MutationResult
	fields, err := w.table(table, in.ExpectedSchemaVersion)
	if err != nil {
		return result, err
	}
	if tx == nil || !validWrite(in.ID, in.OperationID, in.ActorID) || in.Values == nil {
		return result, ErrInvalid
	}
	ids, err := values(in.Values, fields)
	if err != nil {
		return result, err
	}
	if err = w.Gate.LockTable(ctx, tx, table.TableID, table.SchemaVersion); err != nil {
		return result, err
	}
	if err = w.Authorization.Check(ctx, tx, "data.create", in.ActorID, ids); err != nil {
		return result, err
	}
	stored, err := w.DML.Insert(ctx, tx, table, in, ids)
	if err != nil {
		return MutationResult{}, err
	}
	if stored.ID != in.ID || stored.CreatedBy != in.ActorID {
		return MutationResult{}, ErrUnavailable
	}
	result.ID, result.RecordVersion, result.CreatedAt, result.UpdatedAt = stored.ID, stored.RecordVersion, stored.CreatedAt, stored.UpdatedAt
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
	fields, err := w.table(table, in.ExpectedSchemaVersion)
	if err != nil {
		return result, err
	}
	if tx == nil || !validWrite(in.ID, in.OperationID, in.ActorID) || !version(in.ExpectedRecordVersion) || in.Changes == nil {
		return result, ErrInvalid
	}
	ids, err := values(in.Changes, fields)
	if err != nil {
		return result, err
	}
	if err = w.Gate.LockTable(ctx, tx, table.TableID, table.SchemaVersion); err != nil {
		return result, err
	}
	stored, err := w.DML.LockHeader(ctx, tx, table, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return MutationResult{}, ErrMissing
	}
	if err != nil {
		return MutationResult{}, err
	}
	if stored.ID != in.ID || !uuid.MatchString(stored.CreatedBy) || !version(stored.RecordVersion) || stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		return MutationResult{}, ErrUnavailable
	}
	result.RecordVersion, result.CreatedAt, result.UpdatedAt = stored.RecordVersion, stored.CreatedAt, stored.UpdatedAt
	if result.RecordVersion != in.ExpectedRecordVersion {
		return MutationResult{}, ErrConflict
	}
	if len(ids) > 0 && result.RecordVersion == maxVersion {
		return MutationResult{}, ErrConflict
	}
	if err = w.Fence.Check(ctx, tx, table.TableID, in.ID, in.ExpectedRecordVersion); err != nil {
		return MutationResult{}, err
	}
	if err = w.Authorization.Check(ctx, tx, "data.edit", stored.CreatedBy, ids); err != nil {
		return MutationResult{}, err
	}
	result.ID = in.ID
	result.OperationID = in.OperationID
	result.SchemaVersion = table.SchemaVersion
	if len(ids) > 0 {
		updated, e := w.DML.UpdateCAS(ctx, tx, table, in, ids)
		err = e
		if errors.Is(err, pgx.ErrNoRows) {
			return MutationResult{}, ErrConflict
		}
		if err != nil {
			return MutationResult{}, err
		}
		if updated.ID != in.ID || updated.CreatedBy != stored.CreatedBy || updated.RecordVersion != stored.RecordVersion+1 || !updated.CreatedAt.Equal(stored.CreatedAt) || updated.UpdatedAt.IsZero() {
			return MutationResult{}, ErrUnavailable
		}
		result.RecordVersion, result.UpdatedAt = updated.RecordVersion, updated.UpdatedAt
	}
	if err = w.Audit.Append(ctx, tx, result, "edit", ids); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}
func version(v int64) bool { return v >= 1 && v <= maxVersion }
