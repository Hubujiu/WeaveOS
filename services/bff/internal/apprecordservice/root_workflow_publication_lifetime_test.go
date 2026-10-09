package apprecordservice

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"reflect"
	"testing"
)

func TestRootWorkflowPublicationHistoryFullCatalogRemovalKeepsExecution(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	op := rootAction(t, f, rootActionRequest(t, f, "reject"))
	c, p := rootActionRead(t, f, op)
	r, body := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, r, body, true)
	q := WorkflowEventRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, EventID: c.CommandID}
	before := eventRead(t, f, f.principal, q)
	var receiptBefore string
	if err := f.owner.QueryRow(f.ctx, "SELECT row_to_json(d)::text FROM applications.workflow_deployments d WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.head.FlowID).Scan(&receiptBefore); err != nil {
		t.Fatal(err)
	}
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	for _, sql := range []string{"DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state='rejected'"} {
		if _, err = tx.Exec(f.ctx, sql, f.app, f.instance.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = tx.Exec(f.ctx, "DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", f.app, f.head.FlowID); err != nil {
		t.Fatalf("old deployment receipt still requires full configuration: %v", err)
	}
	if _, err = tx.Exec(f.ctx, "DELETE FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if got := eventRead(t, f, f.principal, q); !reflect.DeepEqual(got, before) {
		t.Fatal("full catalog removal changed original execution history")
	}
	if !f.apply(t, c, p, r, body, true).Duplicate {
		t.Fatal("full catalog removal broke original result replay")
	}
	var receiptAfter string
	if err = f.owner.QueryRow(f.ctx, "SELECT row_to_json(d)::text FROM applications.workflow_deployments d WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.head.FlowID).Scan(&receiptAfter); err != nil || receiptAfter != receiptBefore {
		t.Fatal("legacy deployment receipt was lost or upgraded with fabricated fields", err)
	}
	list, err := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, 20))
	if err != nil || len(list.Items) != 2 {
		t.Fatal("full catalog cleanup lost confirmed event list", err)
	}
}

func TestRootWorkflowPublicationHistoryDrainGuardIncludesExecution(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "active-instance", true: "pending-command-even-terminal-projection"}[pending], func(t *testing.T) {
			f := rootTaskSetup(t, true)
			if pending {
				rootAction(t, f, rootActionRequest(t, f, "reject"))
				// Isolate the command drain clause: a deliberately terminal local projection
				// does not turn an accepted, unconfirmed command into safe cleanup.
				eventSQL(t, f, "UPDATE applications.workflow_instances SET state='rejected' WHERE app_id=$1 AND id=$2", f.app, f.instance.ID)
			}
			for _, q := range []string{"DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", "DELETE FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2"} {
				_, err := f.owner.Exec(f.ctx, q, f.app, f.head.FlowID)
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "55000" {
					t.Fatalf("explicit execution drain guard missing: %v", err)
				}
			}
		})
	}
}
