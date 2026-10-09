package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"testing"
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
			var n int
			if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_id=$2", f.app, req.OperationID).Scan(&n); err != nil || n != 0 {
				t.Fatalf("failed edit receipt committed %d %v", n, err)
			}
		})
	}
}
func TestRootOrdinaryEditOtherRecordAndTerminalRemainEditable(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "other_record", true: "completed"}[terminal], func(t *testing.T) {
			f := rootCaptureSetup(t).recordFixture
			instance, record := rootStartIntent(t, f, f.actor)
			if terminal {
				if _, err := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state='completed' WHERE id=$1", instance); err != nil {
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
	var n int
	if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_id=$2", f.app, req.OperationID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("empty edit committed receipt %d %v", n, err)
	}
}
