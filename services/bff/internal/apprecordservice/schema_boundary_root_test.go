package apprecordservice

// Authored by the coordinating assistant, not by the implementation worker.
// Oracle: V030-015 ADR sections 8.3-8.4 and error contract, read 2026-10-03:
// expectedSchemaVersion is a safe integer including zero; authorized unready
// schemas return APPLICATION_SCHEMA_NOT_READY; stale schema and stale row
// versions are different conflicts. A rejected request persists nothing.
// Reuses only the existing isolated PostgreSQL/Redis fixture, not its oracle.

import (
	"errors"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/jackc/pgx/v5"
)

func rootBoundaryOperation(t *testing.T, f recordFixture) string {
	t.Helper()
	var id string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func rootBoundaryRows(t *testing.T, f recordFixture) string {
	t.Helper()
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	var snapshot string
	if err := f.runtime.QueryRow(f.ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY id)::text, '[]') FROM "+relation+" r").Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func rootBoundaryNoWrite(t *testing.T, f recordFixture, operation, before string) {
	t.Helper()
	if after := rootBoundaryRows(t, f); after != before {
		t.Fatalf("rejected request changed business rows: before=%s after=%s", before, after)
	}
	for _, query := range []string{
		"SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2",
		"SELECT count(*) FROM applications.record_write_audit WHERE actor_user_id=$1 AND operation_id=$2",
	} {
		var count int
		if err := f.runtime.QueryRow(f.ctx, query, f.actor, operation).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("rejected request left persisted state: query=%s count=%d", query, count)
		}
	}
}

func rootBoundaryRequireCode(t *testing.T, err error, want string) {
	t.Helper()
	var problem *appstructure.Error
	if !errors.As(err, &problem) || problem.Code != want {
		t.Fatalf("want %s, got %T: %v", want, err, err)
	}
}

func TestRootSchemaZeroUsesLiveAuthorizationBeforeNotReady(t *testing.T) {
	for _, method := range []string{"create", "edit"} {
		for _, allowed := range []bool{true, false} {
			name := method + "/authorized"
			if !allowed {
				name = method + "/menu-only"
			}
			t.Run(name, func(t *testing.T) {
				f := newRecordFixture(t)
				if _, err := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET schema_version=0,schema_ready=false WHERE app_id=$1 AND id=$2", f.app, f.table); err != nil {
					t.Fatal(err)
				}
				if !allowed {
					action := "data." + method
					if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action=$2)", f.app, action); err != nil {
						t.Fatal(err)
					}
					if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action=$2", f.app, action); err != nil {
						t.Fatal(err)
					}
				}
				op := rootBoundaryOperation(t, f)
				before := rootBoundaryRows(t, f)
				meta := applications.Metadata{RequestID: "root-schema-zero-" + method}
				var err error
				if method == "create" {
					_, err = f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 0, Values: map[string]any{}}, meta)
				} else {
					_, err = f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: op, ExpectedSchemaVersion: 0, ExpectedRecordVersion: 1, Changes: map[string]any{}}, meta)
				}
				if allowed {
					rootBoundaryRequireCode(t, err, "APPLICATION_SCHEMA_NOT_READY")
				} else if !errors.Is(err, applications.ErrDenied) {
					t.Fatalf("menu-only request must be denied without exposing schema readiness, got %v", err)
				}
				rootBoundaryNoWrite(t, f, op, before)
			})
		}
	}
}

func TestRootSchemaConflictIsDistinctFromRecordConflict(t *testing.T) {
	for _, method := range []string{"create", "edit"} {
		t.Run(method, func(t *testing.T) {
			f := newRecordFixture(t)
			op := rootBoundaryOperation(t, f)
			before := rootBoundaryRows(t, f)
			meta := applications.Metadata{RequestID: "root-schema-conflict-" + method}
			var err error
			if method == "create" {
				_, err = f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 2, Values: map[string]any{f.public: "must not persist"}}, meta)
			} else {
				_, err = f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: op, ExpectedSchemaVersion: 2, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "must not persist"}}, meta)
			}
			rootBoundaryRequireCode(t, err, "APPLICATION_SCHEMA_CONFLICT")
			if errors.Is(err, apprecords.ErrConflict) {
				t.Fatal("schema mismatch must not masquerade as record CAS conflict")
			}
			rootBoundaryNoWrite(t, f, op, before)
		})
	}
	t.Run("stale-record-with-current-schema", func(t *testing.T) {
		f := newRecordFixture(t)
		op := rootBoundaryOperation(t, f)
		before := rootBoundaryRows(t, f)
		_, err := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: op, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 2, Changes: map[string]any{f.public: "must not persist"}}, applications.Metadata{RequestID: "root-row-conflict"})
		if !errors.Is(err, apprecords.ErrConflict) {
			t.Fatalf("stale row must retain record-CAS identity, got %T: %v", err, err)
		}
		rootBoundaryNoWrite(t, f, op, before)
	})
}
