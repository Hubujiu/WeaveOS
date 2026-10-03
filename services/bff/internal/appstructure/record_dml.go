package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"sort"
	"time"
)

// These neutral shapes mirror V015's fixed TypedDML port without importing or
// modifying its exclusive core package. V015 can use ordinary Go conversions.
type RecordTable struct {
	AppID, TableID, ViewID, Namespace string
	SchemaVersion                     int64
	Ready                             bool
	ActiveFieldIDs                    []string
}
type RecordCreate struct {
	OperationID, ID, ActorID string
	ExpectedSchemaVersion    int64
	Values                   map[string]any
}
type RecordEdit struct {
	OperationID, ID, ActorID                     string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Changes                                      map[string]any
}
type StoredRecordHeader struct {
	ID, CreatedBy        string
	RecordVersion        int64
	CreatedAt, UpdatedAt time.Time
}
type RecordDML struct{}

// Published writes always compose the real history port in the caller's tx.
// The primitive capability stays private to this owner package.
type nativeRecordDML struct{}

func (d RecordDML) Insert(c context.Context, tx pgx.Tx, table RecordTable, in RecordCreate, selected []string) (StoredRecordHeader, error) {
	return (RecordHistoryDML{History: RecordHistoryStore{}, Origin: "ordinary"}).Insert(c, tx, table, in, selected)
}
func (d RecordDML) LockHeader(c context.Context, tx pgx.Tx, table RecordTable, id string) (StoredRecordHeader, error) {
	return (nativeRecordDML{}).LockHeader(c, tx, table, id)
}
func (d RecordDML) UpdateCAS(c context.Context, tx pgx.Tx, table RecordTable, in RecordEdit, selected []string) (StoredRecordHeader, error) {
	return (RecordHistoryDML{History: RecordHistoryStore{}, Origin: "ordinary"}).UpdateCAS(c, tx, table, in, selected)
}
func (d nativeRecordDML) Insert(c context.Context, tx pgx.Tx, table RecordTable, in RecordCreate, selected []string) (StoredRecordHeader, error) {
	if !appfields.ValidID(in.ActorID) || !appfields.ValidID(in.OperationID) {
		return StoredRecordHeader{}, applications.ErrResourceInvalid
	}
	return d.call(c, tx, table, in.ID, in.ActorID, "insert", in.ExpectedSchemaVersion, 0, in.Values, selected)
}
func (d nativeRecordDML) LockHeader(c context.Context, tx pgx.Tx, table RecordTable, id string) (StoredRecordHeader, error) {
	return d.call(c, tx, table, id, "", "lock_header", table.SchemaVersion, 0, map[string]any{}, []string{})
}
func (d nativeRecordDML) UpdateCAS(c context.Context, tx pgx.Tx, table RecordTable, in RecordEdit, selected []string) (StoredRecordHeader, error) {
	if !appfields.ValidID(in.ActorID) || !appfields.ValidID(in.OperationID) || in.ExpectedRecordVersion < 1 || in.ExpectedRecordVersion > 9007199254740991 {
		return StoredRecordHeader{}, applications.ErrResourceInvalid
	}
	return d.call(c, tx, table, in.ID, in.ActorID, "update", in.ExpectedSchemaVersion, in.ExpectedRecordVersion, in.Changes, selected)
}
func (nativeRecordDML) call(c context.Context, tx pgx.Tx, table RecordTable, id, actor, kind string, schema, version int64, values map[string]any, selected []string) (StoredRecordHeader, error) {
	var header StoredRecordHeader
	if tx == nil {
		return header, session.ErrUnavailable
	}
	if !appfields.ValidID(table.AppID) || !appfields.ValidID(table.ViewID) || !appfields.ValidID(table.TableID) || !appfields.ValidID(id) || schema < 1 || schema > 9007199254740991 || values == nil {
		return header, applications.ErrResourceInvalid
	}
	keys := make([]string, 0, len(values))
	for field := range values {
		if !appfields.ValidID(field) {
			return header, applications.ErrResourceInvalid
		}
		keys = append(keys, field)
	}
	sort.Strings(keys)
	ids := append([]string{}, selected...)
	sort.Strings(ids)
	if len(keys) != len(ids) {
		return header, applications.ErrResourceInvalid
	}
	for i := range keys {
		if ids[i] != keys[i] {
			return header, applications.ErrResourceInvalid
		}
	}
	if len(keys) > 0 {
		rows, e := tx.Query(c, "SELECT definition FROM applications.fields WHERE app_id=$1 AND table_id=$2 AND id=ANY($3::uuid[]) AND NOT removed ORDER BY id", table.AppID, table.TableID, keys)
		if e != nil {
			return header, recordError(e)
		}
		seen := 0
		for rows.Next() {
			var raw json.RawMessage
			var field appfields.Field
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return header, session.ErrUnavailable
			}
			if json.Unmarshal(raw, &field) != nil {
				rows.Close()
				return header, session.ErrUnavailable
			}
			candidate, e := json.Marshal(values[field.ID])
			if e != nil {
				rows.Close()
				return header, applications.ErrResourceInvalid
			}
			canonical, e := appfields.NormalizeValue(field, candidate)
			if e != nil || !jsonEqual(json.RawMessage(candidate), canonical) {
				rows.Close()
				return header, applications.ErrResourceInvalid
			}
			seen++
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return header, recordError(e)
		}
		if seen != len(keys) {
			return header, applications.ErrResourceInvalid
		}
	}
	input, e := json.Marshal(values)
	if e != nil {
		return header, applications.ErrResourceInvalid
	}
	var raw []byte
	var trustedActor any = actor
	if actor == "" {
		trustedActor = nil
	}
	e = tx.QueryRow(c, "SELECT applications.apply_record_change($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)", table.AppID, table.ViewID, table.TableID, id, trustedActor, kind, schema, version, input).Scan(&raw)
	if e != nil {
		return header, recordError(e)
	}
	if json.Unmarshal(raw, &header) != nil || header.ID != id || header.CreatedBy == "" || header.RecordVersion < 1 || header.CreatedAt.IsZero() || header.UpdatedAt.IsZero() {
		return StoredRecordHeader{}, session.ErrUnavailable
	}
	return header, nil
}
func recordError(e error) error {
	var pgerr *pgconn.PgError
	if errors.As(e, &pgerr) {
		switch pgerr.Code {
		case "W0001":
			return &Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
		case "W0002":
			return &Error{Code: "APPLICATION_SCHEMA_CONFLICT"}
		case "W0003":
			return &Error{Code: "APPLICATION_RECORD_CONFLICT"}
		case "P0002":
			return applications.ErrMissing
		case "23503", "23514", "23502", "22003", "22007", "22P02":
			return applications.ErrResourceInvalid
		}
	}
	return session.ErrUnavailable
}
