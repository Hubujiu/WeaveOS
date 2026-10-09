package apprecordservice

import (
	"errors"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
)

func TestRootOrdinaryEditRejectsInFlightWorkflow(t *testing.T) {
	for _, state := range []string{"starting", "active"} {
		t.Run(state, func(t *testing.T) {
			f := rootCaptureSetup(t).recordFixture
			instance, record := rootStartIntent(t, f, f.actor)
			// Pure application policy fixture; actual engine states are tested separately.
			if _, err := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state=$2 WHERE id=$1", instance, state); err != nil {
				t.Fatal(err)
			}
			req := EditRequest{AppID: f.app, ViewID: f.view, RecordID: record, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "forbidden ordinary change"}}
			result, err := f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "in-flight-read-only"})
			if !errors.Is(err, ErrWorkflowRecordReadOnly) || result.RecordVersion != 0 {
				t.Fatalf("ordinary editor bypassed workflow: %+v %v", result, err)
			}
			rootReadonlyUnchanged(t, f, record, req.OperationID)
			var n int
			if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_id=$2", f.app, req.OperationID).Scan(&n); err != nil || n != 0 {
				t.Fatalf("failed edit receipt committed %d %v", n, err)
			}
		})
	}
}
func TestRootOrdinaryEditOtherRecordAndTerminalRemainEditable(t *testing.T) {
	for _, state := range []string{"other_record", "completed", "rejected", "withdrawn", "no_effect"} {
		t.Run(state, func(t *testing.T) {
			f := rootCaptureSetup(t).recordFixture
			instance, record := rootStartIntent(t, f, f.actor)
			if state != "other_record" {
				if _, err := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state=$2 WHERE id=$1", instance, state); err != nil {
					t.Fatal(err)
				}
			} else {
				record = f.ownRecord
			}
			req := EditRequest{AppID: f.app, ViewID: f.view, RecordID: record, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "allowed ordinary change"}}
			result, err := f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "outside-in-flight"})
			if err != nil || result.RecordVersion != 2 {
				t.Fatalf("unrelated or terminal record blocked %+v %v", result, err)
			}
		})
	}
}

func TestRootOrdinaryEditEmptyPatchCannotWriteReceiptWhileInFlight(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	_, record := rootStartIntent(t, f, f.actor)
	req := EditRequest{AppID: f.app, ViewID: f.view, RecordID: record, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{}}
	got, err := f.service.Edit(f.ctx, f.principal, req, applications.Metadata{RequestID: "empty-in-flight"})
	if !errors.Is(err, ErrWorkflowRecordReadOnly) || got.RecordVersion != 0 {
		t.Fatalf("empty edit bypassed readonly: %+v %v", got, err)
	}
	rootReadonlyUnchanged(t, f, record, req.OperationID)
	var n int
	if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_id=$2", f.app, req.OperationID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("empty edit committed receipt %d %v", n, err)
	}
}

func rootReadonlyUnchanged(t *testing.T, f recordFixture, record, operation string) {
	t.Helper()
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	col := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	var value string
	var version int64
	if err := f.owner.QueryRow(f.ctx, "SELECT "+col+",record_version FROM "+relation+" WHERE id=$1", record).Scan(&value, &version); err != nil || value != "alpha" || version != 1 {
		t.Fatalf("denied edit changed row %q/v%d %v", value, version, err)
	}
	for _, table := range []string{"operations", "record_write_audit", "record_change_events"} {
		var n int
		if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1 AND operation_id=$2", f.app, operation).Scan(&n); err != nil || n != 0 {
			t.Fatalf("denied edit wrote %s: %d %v", table, n, err)
		}
	}
}
func TestRootOrdinaryEditReplayPreservesReceiptAfterItsTriggerStarts(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	rootTaskKeepDispatchPrivate(t, f)
	rootConfiguredTrigger(t, f, "record.updated", nil)
	req := EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "changed"}}
	meta := applications.Metadata{RequestID: "readonly-replay"}
	original, err := f.service.Edit(f.ctx, f.principal, req, meta)
	if err != nil {
		t.Fatal(err)
	}
	rootTriggeredCount(t, f, f.ownRecord, 1)
	replay, err := f.service.Edit(f.ctx, f.principal, req, meta)
	if err != nil || replay != original {
		t.Fatalf("in-flight policy rejected confirmed replay %+v %v", replay, err)
	}
	rootTriggeredCount(t, f, f.ownRecord, 1)
	req.OperationID = recordOperationID(t, f)
	req.ExpectedRecordVersion = 2
	if _, err = f.service.Edit(f.ctx, f.principal, req, meta); !errors.Is(err, ErrWorkflowRecordReadOnly) {
		t.Fatalf("fresh same-value edit bypassed policy %v", err)
	}
}
