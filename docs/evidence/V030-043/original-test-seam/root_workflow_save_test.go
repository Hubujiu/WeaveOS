package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/jackc/pgx/v5"
)

// Independent FLOW-02/06/07/12 oracles. No Flowable RPC is needed for Save;
// the existing fixture creates one already-confirmed current task.
func rootSaveSetup(t *testing.T, editable bool) rootTaskFixture {
	t.Helper()
	base := rootCaptureSetup(t)
	f := base.recordFixture
	rootTaskKeepDispatchPrivate(t, f)
	graph := rootCatalogGraph(t, f, editable)
	h := rootCatalogReady(t, f, graph)
	instance := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var version string
	if e := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); e != nil {
		t.Fatal(e)
	}
	p := rootProjection{recordFixture: f, head: h, instance: instance, version: version, first: graph.Nodes[1].ID, second: graph.Nodes[1].ID}
	c, payload := rootRecoveryAccept(t, p)
	task := p.task(t, p.first, p.actor, 1)
	receipt, body := p.receipt(t, c, "active", "", task)
	p.apply(t, c, payload, receipt, body, true)
	return rootTaskFixture{rootProjection: p, task: task}
}
func rootSaveRequest(t *testing.T, f rootTaskFixture, value string) WorkflowTaskSaveRequest {
	t.Helper()
	return WorkflowTaskSaveRequest{WorkflowTaskRequest: f.request(), OperationID: recordOperationID(t, f.recordFixture), BasisToken: rootTaskPreview(t, f).BasisToken, Changes: map[string]any{f.public: value}}
}
func rootSave(t *testing.T, f rootTaskFixture, req WorkflowTaskSaveRequest) MutationResult {
	t.Helper()
	result, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "root-v043-save"})
	if e != nil {
		t.Fatal(e)
	}
	return result
}
func rootSaveRow(t *testing.T, f rootTaskFixture, want string, version int64) {
	t.Helper()
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	var got string
	var n int64
	if e := f.owner.QueryRow(f.ctx, "SELECT "+column+",record_version FROM "+relation+" WHERE id=$1", f.ownRecord).Scan(&got, &n); e != nil || got != want || n != version {
		t.Fatalf("real row got %q/v%d want %q/v%d: %v", got, n, want, version, e)
	}
}
func rootSaveCounts(t *testing.T, f rootTaskFixture, writes int) {
	t.Helper()
	for _, table := range []string{"record_change_events", "record_write_audit", "operations"} {
		var n int
		if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1", f.app).Scan(&n); e != nil || n != writes {
			t.Fatalf("%s count %d want %d: %v", table, n, writes, e)
		}
	}
	var state string
	var sequence int64
	var closed bool
	if e := f.owner.QueryRow(f.ctx, "SELECT i.state,i.sequence,t.closed_command_id IS NOT NULL FROM applications.workflow_instances i JOIN applications.workflow_tasks t ON t.instance_id=i.id WHERE i.id=$1 AND t.id=$2", f.instance.ID, f.task.ID).Scan(&state, &sequence, &closed); e != nil || state != "active" || sequence != 1 || closed {
		t.Fatalf("Save advanced workflow: %s/%d/%v %v", state, sequence, closed, e)
	}
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1", f.app).Scan(&n); e != nil || n != 1 {
		t.Fatalf("Save emitted engine command: %d %v", n, e)
	}
}
func rootSaveExec(t *testing.T, f rootTaskFixture, query string, args ...any) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, query, args...); e != nil {
		t.Fatal(e)
	}
}
func TestRootWorkflowSaveWritesOriginalHistoryButNeverAdvancesTask(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "revised")
	got := rootSave(t, f, req)
	if got.ID != f.ownRecord || got.OperationID != req.OperationID || got.RecordVersion != 2 || got.SchemaVersion != 1 || got.UpdatedAt == "" {
		t.Fatalf("bad Save receipt %+v", got)
	}
	rootSaveRow(t, f, "revised", 2)
	rootSaveCounts(t, f, 1)
	var origin, actor, rawRef string
	var before, after int64
	var oldValue, newValue []byte
	e := f.owner.QueryRow(f.ctx, `SELECT e.origin,e.actor_user_id::text,e.opaque_task_ref,e.record_version_before,e.record_version_after,v.old_value,v.new_value FROM applications.record_change_events e JOIN applications.record_change_values v ON v.event_id=e.id WHERE e.app_id=$1 AND e.operation_id=$2 AND v.field_id=$3`, f.app, req.OperationID, f.public).Scan(&origin, &actor, &rawRef, &before, &after, &oldValue, &newValue)
	if e != nil || origin != "task_save" || actor != f.actor || before != 1 || after != 2 || string(oldValue) != `"alpha"` || string(newValue) != `"revised"` {
		t.Fatalf("actual history missing/wrong: %s %s %d/%d %s/%s %v", origin, actor, before, after, oldValue, newValue, e)
	}
	var ref map[string]any
	if json.Unmarshal([]byte(rawRef), &ref) != nil || ref["version"] != float64(1) || ref["flowId"] != f.head.FlowID || ref["instanceId"] != f.instance.ID || ref["taskId"] != f.task.ID || ref["nodeId"] != f.first || ref["definitionVersion"] != float64(1) || ref["activationEpoch"] != float64(1) {
		t.Fatal("task history lacks immutable server identity")
	}
}
func TestRootWorkflowSaveReadOnlyNodeCannotWrite(t *testing.T) {
	f := rootSaveSetup(t, false)
	req := rootSaveRequest(t, f, "no")
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-readonly"})
	if !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("readonly node: %v", e)
	}
	rootSaveRow(t, f, "alpha", 1)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSaveCannotEditOutsideNodeWhitelist(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	req.Changes[f.reference] = nil
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-fields"})
	if !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("mixed forbidden field: %v", e)
	}
	rootSaveRow(t, f, "alpha", 1)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSaveCurrentFieldPermissionStillRequired(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	rootSaveExec(t, f, "DELETE FROM applications.grant_fields gf USING applications.grants g WHERE gf.grant_id=g.id AND g.app_id=$1 AND g.action='data.edit'", f.app)
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-revoked-edit"})
	if !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("node did not preserve revoked edit: %v", e)
	}
	rootSaveRow(t, f, "alpha", 1)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSaveOtherActorCannotUseCurrentTask(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	p := f.principal
	p.UserID = f.other
	_, e := f.service.SaveWorkflowTask(f.ctx, p, req, applications.Metadata{RequestID: "v043-other-actor"})
	if !errors.Is(e, applications.ErrDenied) && !errors.Is(e, ErrWorkflowBasisExpired) {
		t.Fatalf("foreign actor: %v", e)
	}
	rootSaveRow(t, f, "alpha", 1)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSaveStaleRecordBasisRejectsWholeMutation(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	rootSaveExec(t, f, "UPDATE "+relation+" SET record_version=2 WHERE id=$1", f.ownRecord)
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-stale"})
	if !errors.Is(e, ErrWorkflowBasisChanged) {
		t.Fatalf("stale basis: %v", e)
	}
	rootSaveRow(t, f, "alpha", 2)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSaveTaskSequenceChangeRequiresRefresh(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	rootSaveExec(t, f, "UPDATE applications.workflow_instances SET sequence=2 WHERE id=$1", f.instance.ID)
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-task-change"})
	if !errors.Is(e, ErrWorkflowTaskChanged) {
		t.Fatalf("task changed: %v", e)
	}
	rootSaveRow(t, f, "alpha", 1)
}
func TestRootWorkflowSaveExpiredAndCrossSessionBasis(t *testing.T) {
	for _, which := range []string{"expired", "session"} {
		t.Run(which, func(t *testing.T) {
			f := rootSaveSetup(t, true)
			req := rootSaveRequest(t, f, "no")
			p := f.principal
			if which == "expired" {
				req.BasisToken = strings.Repeat("A", 43)
			} else {
				p.SessionRef = recordOperationID(t, f.recordFixture)
			}
			_, e := f.service.SaveWorkflowTask(f.ctx, p, req, applications.Metadata{RequestID: "v043-token"})
			if !errors.Is(e, ErrWorkflowBasisExpired) {
				t.Fatalf("basis %s: %v", which, e)
			}
			rootSaveRow(t, f, "alpha", 1)
			rootSaveCounts(t, f, 0)
		})
	}
}
func TestRootWorkflowSaveIdenticalReplayAndConflictingPayload(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "once")
	first := rootSave(t, f, req)
	again := rootSave(t, f, req)
	if first != again {
		t.Fatal("replay changed minimum receipt")
	}
	rootSaveCounts(t, f, 1)
	req.Changes = map[string]any{f.public: "twice"}
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-conflict"})
	if !errors.Is(e, applications.ErrOperationConflict) {
		t.Fatalf("different request reused operation: %v", e)
	}
	rootSaveRow(t, f, "once", 2)
	rootSaveCounts(t, f, 1)
}
func TestRootWorkflowSaveOldApprovalBasisCannotApproveNewValue(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "saved")
	rootSave(t, f, req)
	action := WorkflowTaskActionRequest{WorkflowTaskRequest: f.request(), OperationID: recordOperationID(t, f.recordFixture), Action: "agree", BasisToken: req.BasisToken}
	_, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, action, applications.Metadata{RequestID: "v043-old-approve"})
	if !errors.Is(e, ErrWorkflowBasisChanged) {
		t.Fatalf("old approval basis accepted: %v", e)
	}
	rootSaveRow(t, f, "saved", 2)
	rootSaveCounts(t, f, 1)
	refreshed := rootActionRequest(t, f, "agree")
	r := rootAction(t, f, refreshed)
	if r.Status != "pending" {
		t.Fatal("explicit refreshed approval unavailable")
	}
	rootSaveRow(t, f, "saved", 2)
}
func TestRootWorkflowSavePendingCommandFenceRejectsSave(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	rootAction(t, f, rootActionRequest(t, f, "agree"))
	_, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-fenced"})
	var typed *appstructure.Error
	if !errors.As(e, &typed) || typed.Code != "APPLICATION_RECORD_FENCED" {
		t.Fatalf("pending command did not fence save: %v", e)
	}
	rootSaveRow(t, f, "alpha", 1)
}
func TestRootWorkflowSaveConcurrentWritersHaveOneActualVersion(t *testing.T) {
	f := rootSaveSetup(t, true)
	a := rootSaveRequest(t, f, "a")
	b := a
	b.OperationID = recordOperationID(t, f.recordFixture)
	b.Changes = map[string]any{f.public: "b"}
	requests := []WorkflowTaskSaveRequest{a, b}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = f.service.SaveWorkflowTask(f.ctx, f.principal, requests[i], applications.Metadata{RequestID: "v043-race"})
		}(i)
	}
	wg.Wait()
	successes := 0
	for _, e := range errs {
		if e == nil {
			successes++
		} else if !errors.Is(e, ErrWorkflowBasisChanged) && !errors.Is(e, apprecords.ErrConflict) {
			t.Fatalf("unexpected race error: %v", e)
		}
	}
	if successes != 1 {
		t.Fatalf("expected one committed writer: %v", errs)
	}
	rootSaveCounts(t, f, 1)
}
func TestRootWorkflowSaveActualCommitRollbackIsAtomic(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	abort := &abortRecordBeforeCommit{}
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, nil, abort)
	result, e := faulty.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-rollback"})
	if !abort.injected.Load() || !errors.Is(e, pgx.ErrTxCommitRollback) || result.ID != "" {
		t.Fatalf("rollback not preserved %+v %v", result, e)
	}
	rootSaveRow(t, f, "alpha", 1)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSaveLostCommitReplyReplaysOneRealSave(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "committed")
	var dropped atomic.Bool
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, &dropped, nil)
	result, e := faulty.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-lost-commit"})
	if !dropped.Load() || !errors.Is(e, applications.ErrUnconfirmed) || result.ID != "" {
		t.Fatalf("lost commit reported as certain %+v %v", result, e)
	}
	rootSaveRow(t, f, "committed", 2)
	rootSaveCounts(t, f, 1)
	r := rootSave(t, f, req)
	if r.RecordVersion != 2 {
		t.Fatal("retry rewrote committed data")
	}
	rootSaveCounts(t, f, 1)
}
func TestRootWorkflowSaveHistoryFailureRollsBackTypedRowAndOperation(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "no")
	fn := "save_fail_" + strings.ReplaceAll(f.app, "-", "")
	rootSaveExec(t, f, fmt.Sprintf(`CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic history failure'; END $$; CREATE TRIGGER %s BEFORE INSERT ON applications.record_change_events FOR EACH ROW WHEN (NEW.app_id='%s'::uuid) EXECUTE FUNCTION applications.%s()`, fn, fn, f.app, fn))
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER %s ON applications.record_change_events; DROP FUNCTION applications.%s()", fn, fn))
	})
	result, e := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "v043-history-fail"})
	if e == nil || result.ID != "" {
		t.Fatalf("history failure became success %+v %v", result, e)
	}
	rootSaveRow(t, f, "alpha", 1)
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowSavePreviewExposesOnlyEditableVisibleIntersection(t *testing.T) {
	f := rootSaveSetup(t, true)
	preview := rootTaskPreview(t, f)
	raw, e := json.Marshal(preview)
	if e != nil {
		t.Fatal(e)
	}
	var dto struct {
		Editable []string `json:"editableFieldIds"`
	}
	if json.Unmarshal(raw, &dto) != nil {
		t.Fatal("invalid preview")
	}
	sort.Strings(dto.Editable)
	if !reflect.DeepEqual(dto.Editable, []string{f.public}) {
		t.Fatalf("editable intersection: %v", dto.Editable)
	}
}
func TestRootWorkflowSaveDefinitionRevisionDoesNotChangeInflightWhitelist(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "old node still editable")
	graph := rootCatalogGraph(t, f.recordFixture, false)
	graph.Nodes[1].ID = f.first
	graph.Edges[0].To = f.first
	graph.Edges[1].From = f.first
	rootCatalogPut(t, f.recordFixture, f.head.FlowID, f.head.Revision, graph)
	rootSave(t, f, req)
	rootSaveRow(t, f, "old node still editable", 2)
	rootSaveCounts(t, f, 1)
}
func TestRootWorkflowSaveDoesNotDependOnEngineReadiness(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "local save")
	f.service.RuntimeReady = func(context.Context) error { return errors.New("synthetic engine offline") }
	rootSave(t, f, req)
	rootSaveRow(t, f, "local save", 2)
	rootSaveCounts(t, f, 1)
}
