// Package appschema plans and executes PostgreSQL structure changes. It has no
// HTTP endpoint, permission policy, application metadata schema, or flow store.
package appschema

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Type is a physical primitive, not a complete business-field type registry.
// Numeric precision, time zones, selectors and reference semantics remain with
// the integration owner. Unsupported types fail closed.
type Type string

const (
	Text      Type = "text"
	Boolean   Type = "boolean"
	Numeric   Type = "numeric"
	Date      Type = "date"
	Timestamp Type = "timestamptz"
	UUID      Type = "uuid"
	UUIDArray Type = "uuid[]"
)

type Value struct {
	Type    Type
	Text    string
	Boolean bool
	UUIDs   []string
}

type Field struct {
	ID             string
	Name           string
	Type           Type
	Required       bool
	Precision      int
	Scale          int
	Kind           string
	RoundingPlaces int
	RoundingMode   string
	TimePrecision  string
	Default        *Value
}

type Operation string

const (
	AddColumn     Operation = "add_column"
	DropColumn    Operation = "drop_column"
	AlterType     Operation = "alter_type"
	AlterDefault  Operation = "alter_default"
	AlterRequired Operation = "alter_required"
)

type Change struct {
	Operation Operation
	Column    string
	Before    *Field
	After     *Field
}

type Plan struct {
	TableID   string
	TableName string
	Fields    []Field
	Changes   []Change
}

type Snapshot struct {
	Exists   bool
	Revision string // Opaque storage token; no public schemaVersion format implied.
	Fields   []Field
}

type Metadata interface {
	// Lock must serialize structure saves and relevant dependency registration
	// for this logical table until tx ends. Missing/new tables need the same gate.
	Lock(context.Context, pgx.Tx, string) (Snapshot, error)
	// Store must persist fields and any integrated layout/audit in this same tx.
	// It owns the revision format and returns the new opaque revision.
	Store(context.Context, pgx.Tx, string, Snapshot, []Field) (string, error)
}

type SaveGuard interface {
	// Check validates the caller and ownership/fences in tx. No permission
	// policy or bootstrap bypass is supplied by this package.
	Check(context.Context, pgx.Tx, string) error
}

type Reference struct {
	FieldID string
	Kind    string
	ID      string
}

type DependencyProtector interface {
	// Protect returns enabled-flow/in-flight references and protects against
	// new registrations until tx ends. A mere unlocked remote read is insufficient.
	Protect(context.Context, pgx.Tx, string, []string) ([]Reference, error)
}

type ColumnImpact struct {
	FieldID     string
	NonNullRows int64
}

type DeletionConfirmation interface {
	// Verify must bind actual user consent to the CURRENT affected data under
	// tx's table lock. Counts alone cannot detect same-count record changes.
	// The integration owner supplies its record/version proof and token protocol.
	Verify(context.Context, pgx.Tx, string, []ColumnImpact) error
}

type Beginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type Limits struct {
	LockTimeout      time.Duration
	StatementTimeout time.Duration
}

type DDL interface {
	LockPhysical(context.Context, pgx.Tx, string) error
	Create(context.Context, pgx.Tx, Plan) error
	Change(context.Context, pgx.Tx, string, Change, map[string]Value) error
}
type Executor struct {
	DDL          DDL
	DB           Beginner
	Namespace    string // Explicit caller-owned PostgreSQL namespace, never a display name.
	Metadata     Metadata
	Guard        SaveGuard
	Dependencies DependencyProtector
	Limits       Limits // Required explicit configuration; no product timeout defaults.
}

type Request struct {
	TableID          string
	ExpectedRevision string
	Fields           []Field
	Backfills        map[string]Value // Typed constants for newly added columns only.
	Confirmation     DeletionConfirmation
}

type Result struct {
	Revision string
	Plan     Plan
}

var (
	ErrInvalid                 = errors.New("invalid schema request or executor configuration")
	ErrUnsupportedType         = errors.New("physical primitive not supported")
	ErrRevisionConflict        = errors.New("schema revision conflict")
	ErrRequiredBackfill        = errors.New("existing rows require a default or backfill")
	ErrDependenciesUnavailable = errors.New("dependency protection unavailable")
	ErrConfirmationRequired    = errors.New("current deletion impact requires confirmation")
	ErrDependenciesBlocked     = errors.New("enabled flow or in-flight field dependencies block change")
	ErrCommitUnknown           = errors.New("schema commit outcome unknown; reconcile before retry")
)

type ConfirmationError struct{ Impacts []ColumnImpact }

func (e *ConfirmationError) Error() string { return ErrConfirmationRequired.Error() }
func (e *ConfirmationError) Unwrap() error { return ErrConfirmationRequired }

type DependencyError struct{ References []Reference }

func (e *DependencyError) Error() string { return ErrDependenciesBlocked.Error() }
func (e *DependencyError) Unwrap() error { return ErrDependenciesBlocked }

type CommitError struct{ Cause error }

func (e *CommitError) Error() string   { return ErrCommitUnknown.Error() }
func (e *CommitError) Unwrap() []error { return []error{ErrCommitUnknown, e.Cause} }
