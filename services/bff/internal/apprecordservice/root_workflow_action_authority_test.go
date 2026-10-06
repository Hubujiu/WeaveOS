package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"testing"
)

func TestRootWorkflowActionLostActualRowAccessDoesNotRevealClosedTask(t *testing.T) {
	f := rootTaskSetup(t, true)
	req := rootActionRequest(t, f, "agree")
	c, p := f.accept(t, "agree", f.task.ID, "", 1, 1)
	r, b := f.receipt(t, c, "completed", "")
	f.apply(t, c, p, r, b, true)
	// Remove all-row read; retain the existing Own-only grant, which cannot see this foreign row.
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all')", f.app); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'", f.app); e != nil {
		t.Fatal(e)
	}
	result, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "no-row-no-task-state"})
	if !errors.Is(e, applications.ErrMissing) || result.CommandID != "" {
		t.Fatalf("row-invisible actor learned closed task state: %v", e)
	}
	var count int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, req.OperationID).Scan(&count); e != nil || count != 0 {
		t.Fatal("row-invisible action claimed operation", e)
	}
}
func TestRootWorkflowActionSQLNullIsFalseButMissingConditionIsNotTrue(t *testing.T) {
	t.Run("actual-null-value", func(t *testing.T) {
		f, node := rootTaskConditionSetup(t)
		base := rootEvidenceStoreFixture{recordFixture: f.recordFixture}
		if _, e := f.owner.Exec(f.ctx, "UPDATE "+rootCaptureRelation(base)+" SET "+rootCaptureColumn(f.secret)+"=NULL WHERE id=$1", f.ownRecord); e != nil {
			t.Fatal(e)
		}
		req := rootActionRequest(t, f, "agree")
		r := rootAction(t, f, req)
		_, p := rootActionRead(t, f, r)
		v, exists := p.Routes[node]
		if !exists || v {
			t.Fatal("SQL NULL predicate did not take false branch")
		}
	})
	t.Run("corrupt-null-predicate", func(t *testing.T) {
		f, node := rootTaskConditionSetup(t)
		req := rootActionRequest(t, f, "agree")
		// Owner-only corruption in this synthetic scope; runtime versions are immutable.
		if _, e := f.owner.Exec(f.ctx, `UPDATE applications.workflow_versions SET graph_json=jsonb_set(graph_json,'{Nodes}',(SELECT jsonb_agg(CASE WHEN n->>'ID'=$3 THEN jsonb_set(n,'{Condition}','null'::jsonb) ELSE n END ORDER BY ordinal) FROM jsonb_array_elements(graph_json->'Nodes') WITH ORDINALITY AS parts(n,ordinal))) WHERE app_id=$1 AND flow_id=$2 AND version=1`, f.app, f.head.FlowID, node); e != nil {
			t.Fatal(e)
		}
		result, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "invalid-condition-not-match-all"})
		if !errors.Is(e, ErrUnavailable) || result.CommandID != "" {
			t.Fatalf("missing stored condition silently matched all: %v", e)
		}
		rootActionCount(t, f, 0, 0, 0)
	})
}
