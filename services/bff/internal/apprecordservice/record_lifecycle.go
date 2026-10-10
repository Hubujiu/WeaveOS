package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RecordLifecycleRequest struct {
	AppID, ViewID, RecordID, OperationID         string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Deleted                                      bool
}
type RecordLifecycleResult struct {
	OperationID   string `json:"operationId"`
	ID            string `json:"id"`
	RecordVersion int64  `json:"recordVersion"`
	SchemaVersion int64  `json:"schemaVersion"`
	Deleted       bool   `json:"deleted"`
}

func lifecycleManagerAuthorization(_ context.Context, _ pgx.Tx, f applications.RecordContext) error {
	if !f.Actor.BootstrapAdmin && f.Actor.ID != f.App.OwnerUserID {
		return applications.ErrDenied
	}
	return nil
}

func (s *Service) ChangeRecordLifecycle(ctx context.Context, p session.Principal, req RecordLifecycleRequest) (RecordLifecycleResult, error) {
	var empty RecordLifecycleResult
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.OperationID} {
		if !appfields.ValidID(id) || id == "00000000-0000-0000-0000-000000000000" {
			return empty, applications.ErrResourceInvalid
		}
	}
	if req.ExpectedSchemaVersion < 0 || req.ExpectedSchemaVersion > 9007199254740991 || req.ExpectedRecordVersion < 1 || req.ExpectedRecordVersion > 9007199254740991 {
		return empty, applications.ErrResourceInvalid
	}
	kind := "record.restore"
	if req.Deleted {
		kind = "record.delete"
	}
	raw, _ := json.Marshal(req)
	hash := sha256.Sum256(raw)
	w, e := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, p, req.AppID, req.ViewID, applications.RecordWriteOptions{OperationID: req.OperationID, Kind: kind, Fingerprint: hash, LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, Authorize: lifecycleManagerAuthorization})
	if e != nil {
		return empty, e
	}
	defer w.Rollback(context.Background())
	old, e := w.Replay(ctx, req.OperationID, kind, hash)
	if e != nil {
		return empty, e
	}
	if old != nil {
		if e = json.Unmarshal(old.Data, &empty); e != nil {
			return RecordLifecycleResult{}, ErrUnavailable
		}
		return empty, w.Commit(ctx)
	}
	if e = w.Claim(ctx, req.OperationID, kind, hash); e != nil {
		return empty, e
	}
	f := w.Context()
	if !f.SchemaReady {
		return empty, apprecords.ErrNotReady
	}
	if f.SchemaVersion != req.ExpectedSchemaVersion {
		return empty, &appstructure.Error{Code: "APPLICATION_SCHEMA_CONFLICT", Data: map[string]int64{"currentSchemaVersion": f.SchemaVersion}}
	}
	var result json.RawMessage
	e = w.Tx().QueryRow(ctx, "SELECT applications.change_record_lifecycle($1,$2,$3,$4,$5,$6,$7,$8,$9)", req.AppID, req.ViewID, f.TableID, req.RecordID, f.Actor.ID, req.OperationID, req.ExpectedSchemaVersion, req.ExpectedRecordVersion, req.Deleted).Scan(&result)
	if e != nil {
		return empty, lifecycleError(e)
	}
	if e = json.Unmarshal(result, &empty); e != nil || empty.OperationID != req.OperationID || empty.ID != req.RecordID || empty.RecordVersion != req.ExpectedRecordVersion+1 || empty.SchemaVersion != req.ExpectedSchemaVersion || empty.Deleted != req.Deleted {
		return RecordLifecycleResult{}, ErrUnavailable
	}
	if e = w.Complete(ctx, req.OperationID, applications.Result{Status: 200, Data: result}); e != nil {
		return RecordLifecycleResult{}, e
	}
	if e = w.Commit(ctx); e != nil {
		return RecordLifecycleResult{}, e
	}
	return empty, nil
}

func lifecycleError(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		switch p.Code {
		case "W0001":
			return apprecords.ErrNotReady
		case "W0002":
			return &appstructure.Error{Code: "APPLICATION_SCHEMA_CONFLICT"}
		case "W0003":
			return apprecords.ErrConflict
		case "P0002":
			return applications.ErrMissing
		case "W0034":
			return &appstructure.Error{Code: "APPLICATION_RECORD_LIFECYCLE_CONFLICT"}
		case "W0035":
			return &appstructure.Error{Code: "APPLICATION_RECORD_FENCED"}
		case "W0036":
			return ErrWorkflowRecordReadOnly
		case "23514", "23503", "22P02":
			return applications.ErrResourceInvalid
		}
	}
	return ErrUnavailable
}
