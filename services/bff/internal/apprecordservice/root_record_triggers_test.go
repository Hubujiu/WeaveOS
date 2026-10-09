package apprecordservice

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

func rootConfiguredTrigger(t *testing.T, f recordFixture, event string, condition json.RawMessage) workflowcatalog.Head {
	t.Helper()
	config := []workflowcatalog.Trigger{{Event: event, Condition: condition}}
	in := rootCatalogInput(f, recordOperationID(t, f), 0, rootCatalogGraph(t, f, false))
	in.Triggers = &config
	var h workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		h, err = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
		return err
	})
	return rootCatalogEnable(t, f, rootCatalogDeploy(t, f, h))
}
func rootTriggeredCount(t *testing.T, f recordFixture, recordID string, want int) {
	t.Helper()
	var n int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1 AND record_id=$2", f.app, recordID).Scan(&n); err != nil || n != want {
		t.Fatalf("persisted trigger instances=%d want=%d err=%v", n, want, err)
	}
}
func rootTriggerCreateRequest(t *testing.T, f recordFixture, value string) CreateRequest {
	t.Helper()
	return CreateRequest{AppID: f.app, ViewID: f.view, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, Values: map[string]any{f.public: value}}
}
func TestRootRecordTriggersCreateTwoFlowsAndReplay(t *testing.T) {
	f := newRecordFixture(t)
	a := rootConfiguredTrigger(t, f, "record.created", rootTriggerCondition(f.public))
	b := rootConfiguredTrigger(t, f, "record.created", nil)
	req := rootTriggerCreateRequest(t, f, "alpha")
	meta := applications.Metadata{RequestID: "v044-two-flows"}
	got, err := f.service.Create(f.ctx, f.principal, req, meta)
	if err != nil {
		t.Fatal(err)
	}
	rootTriggeredCount(t, f, got.ID, 2)
	var valid int
	if err = f.owner.QueryRow(f.ctx, `SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1 AND record_id=$2 AND flow_id=ANY($3::uuid[]) AND initiator_id=$4 AND state='starting' AND definition_version=1 AND sequence=0`, f.app, got.ID, []string{a.FlowID, b.FlowID}, f.actor).Scan(&valid); err != nil || valid != 2 {
		t.Fatalf("wrong durable intent identity/state: %d %v", valid, err)
	}
	replay, err := f.service.Create(f.ctx, f.principal, req, meta)
	if err != nil || replay != got {
		t.Fatalf("operation replay changed: %+v %v", replay, err)
	}
	rootTriggeredCount(t, f, got.ID, 2)
}
func TestRootRecordTriggersCreateFiltersEventConditionAndDisabled(t *testing.T) {
	f := newRecordFixture(t)
	rootConfiguredTrigger(t, f, "record.created", rootTriggerCondition(f.public))
	rootConfiguredTrigger(t, f, "record.updated", nil)
	rootConfiguredTrigger(t, f, "manual", nil)
	disabled := rootConfiguredTrigger(t, f, "record.created", nil)
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		_, err := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, disabled.FlowID, disabled.Revision)
		return err
	})
	matched := rootConfiguredTrigger(t, f, "record.created", nil)
	got, err := f.service.Create(f.ctx, f.principal, rootTriggerCreateRequest(t, f, "beta"), applications.Metadata{RequestID: "v044-event-condition"})
	if err != nil {
		t.Fatal(err)
	}
	rootTriggeredCount(t, f, got.ID, 1)
	var flow string
	if err = f.owner.QueryRow(f.ctx, "SELECT flow_id::text FROM applications.workflow_instances WHERE app_id=$1 AND record_id=$2", f.app, got.ID).Scan(&flow); err != nil || flow != matched.FlowID {
		t.Fatalf("wrong match %s %v", flow, err)
	}
}
func TestRootRecordTriggersUsePublishedNotCandidateConfiguration(t *testing.T) {
	f := newRecordFixture(t)
	h := rootConfiguredTrigger(t, f, "record.created", rootTriggerCondition(f.public))
	empty := []workflowcatalog.Trigger{}
	in := rootCatalogInput(f, h.FlowID, h.Revision, rootCatalogGraph(t, f, false))
	in.Triggers = &empty
	rootCatalogTx(t, f, func(tx pgx.Tx) error { _, err := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in); return err })
	got, err := f.service.Create(f.ctx, f.principal, rootTriggerCreateRequest(t, f, "alpha"), applications.Metadata{RequestID: "v044-published-config"})
	if err != nil {
		t.Fatal(err)
	}
	rootTriggeredCount(t, f, got.ID, 1)
	var version int64
	if err = f.owner.QueryRow(f.ctx, "SELECT definition_version FROM applications.workflow_instances WHERE app_id=$1 AND record_id=$2", f.app, got.ID).Scan(&version); err != nil || version != 1 {
		t.Fatalf("candidate replaced published trigger %d %v", version, err)
	}
}
func TestRootRecordTriggersSameTableOtherView(t *testing.T) {
	f := newRecordFixture(t)
	h := rootConfiguredTrigger(t, f, "record.created", nil)
	otherView := recordOperationID(t, f)
	if _, err := f.owner.Exec(f.ctx, "INSERT INTO applications.form_views(id,app_id,table_id,name,position,view_version) VALUES($1,$2,$3,'another view',1,1)", otherView, f.app, f.table); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(f.ctx, "INSERT INTO applications.menu_resources VALUES($1,'form',$2)", f.app, otherView); err != nil {
		t.Fatal(err)
	}
	req := rootTriggerCreateRequest(t, f, "alpha")
	req.ViewID = otherView
	owner := f.principal
	owner.UserID = f.other
	owner.SessionRef = recordOperationID(t, f)
	got, err := f.service.Create(f.ctx, owner, req, applications.Metadata{RequestID: "v044-other-view"})
	if err != nil {
		t.Fatal(err)
	}
	rootTriggeredCount(t, f, got.ID, 1)
	var view, flow string
	if err = f.owner.QueryRow(f.ctx, "SELECT view_id::text,flow_id::text FROM applications.workflow_instances WHERE app_id=$1 AND record_id=$2", f.app, got.ID).Scan(&view, &flow); err != nil || view != f.view || flow != h.FlowID {
		t.Fatalf("trigger binding changed to write view: %s %s %v", view, flow, err)
	}
}
func TestRootRecordTriggersEditOnlyActualChanges(t *testing.T) {
	for _, kind := range []string{"same_text", "empty", "same_typed_uuid", "different_matching_field", "no_longer_matches"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecordFixture(t)
			rootConfiguredTrigger(t, f, "record.updated", rootTriggerCondition(f.public))
			changes := map[string]any{}
			want := 0
			switch kind {
			case "same_text":
				changes[f.public] = "alpha"
			case "same_typed_uuid":
				changes[f.reference] = f.other
			case "different_matching_field":
				changes[f.reference] = f.actor
				want = 1
			case "no_longer_matches":
				changes[f.public] = "beta"
			}
			_, err := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: changes}, applications.Metadata{RequestID: "v044-real-change-" + kind})
			if err != nil {
				t.Fatal(err)
			}
			rootTriggeredCount(t, f, f.ownRecord, want)
		})
	}
}
func TestRootRecordTriggersEditAgainWhileConditionRemainsTrue(t *testing.T) {
	f := newRecordFixture(t)
	rootConfiguredTrigger(t, f, "record.updated", rootTriggerCondition(f.public))
	var version int64 = 1
	for i, value := range []string{f.actor, f.other} {
		got, err := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: version, Changes: map[string]any{f.reference: value}}, applications.Metadata{RequestID: "v044-remains-matching"})
		if err != nil {
			t.Fatal(err)
		}
		rootTriggeredCount(t, f, f.ownRecord, i+1)
		version = got.RecordVersion
		// Owner fixture models a confirmed terminal state; not engine E2E evidence.
		if _, err = f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state='completed' WHERE app_id=$1 AND record_id=$2", f.app, f.ownRecord); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRootRecordTriggersCommitFailureRollsBackRecordIntentsAuditAndReceipt(t *testing.T) {
	f := newRecordFixture(t)
	rootConfiguredTrigger(t, f, "record.created", nil)
	rootConfiguredTrigger(t, f, "record.created", nil)
	name := "v044_trigger_fault_" + strings.ReplaceAll(f.app, "-", "")
	function := pgx.Identifier{"applications", name}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	sql := "CREATE FUNCTION " + function + "() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.app_id='" + f.app + "'::uuid THEN RAISE EXCEPTION 'isolated V044 final-commit fault' USING ERRCODE='P0001'; END IF; RETURN NEW; END $$"
	if _, err := f.owner.Exec(f.ctx, sql); err != nil {
		t.Fatal(err)
	}
	defer f.owner.Exec(f.ctx, "DROP FUNCTION "+function+"() CASCADE")
	if _, err := f.owner.Exec(f.ctx, "CREATE CONSTRAINT TRIGGER "+trigger+" AFTER INSERT ON applications.workflow_instances DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION "+function+"()"); err != nil {
		t.Fatal(err)
	}
	req := rootTriggerCreateRequest(t, f, "alpha")
	if _, err := f.service.Create(f.ctx, f.principal, req, applications.Metadata{RequestID: "v044-atomic-commit-fault"}); err == nil {
		t.Fatal("record save falsely succeeded without committing all matching trigger intents")
	}
	physical := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	var count int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM "+physical).Scan(&count); err != nil || count != 2 {
		t.Fatalf("record escaped transaction rollback: %d %v", count, err)
	}
	for _, table := range []string{"workflow_instances", "record_write_audit", "operations"} {
		if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1", f.app).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s escaped transaction rollback: %d %v", table, count, err)
		}
	}
}

func TestRootRecordTriggersTypedNumericAndNullChangeDetection(t *testing.T) {
	for _, c := range []struct {
		name, stored string
		input        any
		want         int
	}{{"same_numeric_value", "1.00", "1.000", 0}, {"different_numeric_value", "1.00", "1.01", 1}, {"null_to_null", "", nil, 0}, {"null_to_number", "", "0.00", 1}} {
		t.Run(c.name, func(t *testing.T) {
			f := newRecordFixture(t)
			base := rootEvidenceStoreFixture{recordFixture: f}
			var raw []byte
			if err := f.owner.QueryRow(f.ctx, "SELECT fields_json FROM applications.logical_tables WHERE id=$1", f.table).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var defs []appfields.Field
			if err := json.Unmarshal(raw, &defs); err != nil {
				t.Fatal(err)
			}
			amount := appfields.Field{ID: recordOperationID(t, f), Name: "Amount", Kind: "number", Default: json.RawMessage("null"), Config: json.RawMessage(`{"precision":38,"scale":2,"roundingPlaces":2,"roundingMode":"HALF_UP"}`)}
			rootCaptureAddField(t, base, amount, map[string]any{"Type": "numeric", "Precision": 38, "Scale": 2})
			rootCaptureDefinitions(t, base, append(defs, amount))
			if c.stored != "" {
				if _, err := f.owner.Exec(f.ctx, "UPDATE "+rootCaptureRelation(base)+" SET "+rootCaptureColumn(amount.ID)+"=$2::numeric WHERE id=$1", f.ownRecord, c.stored); err != nil {
					t.Fatal(err)
				}
			}
			rootConfiguredTrigger(t, f, "record.updated", nil)
			owner := f.principal
			owner.UserID = f.other
			owner.SessionRef = recordOperationID(t, f)
			_, err := f.service.Edit(f.ctx, owner, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{amount.ID: c.input}}, applications.Metadata{RequestID: "v044-typed-" + c.name})
			if err != nil {
				t.Fatal(err)
			}
			rootTriggeredCount(t, f, f.ownRecord, c.want)
		})
	}
}

// Supplemental regressions for existing authority/idempotency/Save contracts.
func TestRootRecordTriggersConcurrentOperationReplaysSingleIntentSet(t *testing.T) {
	f := newRecordFixture(t)
	rootConfiguredTrigger(t, f, "record.created", nil)
	rootConfiguredTrigger(t, f, "record.created", nil)
	req := rootTriggerCreateRequest(t, f, "concurrent")
	const workers = 4
	results := make(chan MutationResult, workers)
	failures := make(chan error, workers)
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			got, err := f.service.Create(f.ctx, f.principal, req, applications.Metadata{RequestID: "v044-concurrent-replay"})
			results <- got
			failures <- err
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first MutationResult
	for result := range results {
		if first.ID == "" {
			first = result
		} else if first != result {
			t.Fatal("concurrent operation replay changed response")
		}
	}
	rootTriggeredCount(t, f, first.ID, 2)
}
func TestRootRecordTriggersNodeSaveDoesNotTriggerOtherFlow(t *testing.T) {
	f := rootSaveSetup(t, true)
	rootConfiguredTrigger(t, f.recordFixture, "record.updated", nil)
	rootSave(t, f, rootSaveRequest(t, f, "changed by node save"))
	rootTriggeredCount(t, f.recordFixture, f.ownRecord, 1)
	rootSaveCounts(t, f, 1)
}
func TestRootRecordTriggersDeniedWriteCreatesNoIntent(t *testing.T) {
	f := newRecordFixture(t)
	rootConfiguredTrigger(t, f, "record.created", nil)
	req := rootTriggerCreateRequest(t, f, "alpha")
	req.Values[f.secret] = "forbidden"
	if _, err := f.service.Create(f.ctx, f.principal, req, applications.Metadata{RequestID: "v044-denied"}); !errors.Is(err, applications.ErrDenied) {
		t.Fatalf("denied field write: %v", err)
	}
	var count int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1", f.app).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unauthorized trigger persisted: %d %v", count, err)
	}
}

func TestRootRecordTriggersDoesNotSkipLowestValidFlowID(t *testing.T) {
	f := newRecordFixture(t)
	config := []workflowcatalog.Trigger{{Event: "record.created", Condition: json.RawMessage("null")}}
	in := rootCatalogInput(f, "00000000-0000-0000-0000-000000000001", 0, rootCatalogGraph(t, f, false))
	in.Triggers = &config
	var h workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		h, err = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
		return err
	})
	rootCatalogEnable(t, f, rootCatalogDeploy(t, f, h))
	got, err := f.service.Create(f.ctx, f.principal, rootTriggerCreateRequest(t, f, "alpha"), applications.Metadata{RequestID: "v044-lowest-uuid"})
	if err != nil {
		t.Fatal(err)
	}
	rootTriggeredCount(t, f, got.ID, 1)
}
