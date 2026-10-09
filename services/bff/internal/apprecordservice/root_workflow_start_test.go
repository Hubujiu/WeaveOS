package apprecordservice

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"strings"
	"sync"
	"testing"
)

func rootStartIntent(t *testing.T, f recordFixture, approver string) (string, string) {
	t.Helper()
	rootTaskKeepDispatchPrivate(t, f)
	graph := rootCatalogGraph(t, f, false)
	graph.Nodes[1].Approval.AssigneeIDs = []string{approver}
	cfg := []wc.Trigger{{Event: "record.created"}}
	in := rootCatalogInput(f, recordOperationID(t, f), 0, graph)
	in.Triggers = &cfg
	var head wc.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		head, err = (wc.Catalog{}).PutVersionInTx(f.ctx, tx, in)
		return err
	})
	rootCatalogEnable(t, f, rootCatalogDeploy(t, f, head))
	record, err := f.service.Create(f.ctx, f.principal, rootTriggerCreateRequest(t, f, "alpha"), applications.Metadata{RequestID: "start-admission"})
	if err != nil {
		t.Fatal(err)
	}
	var instance string
	if err = f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3", f.app, head.FlowID, record.ID).Scan(&instance); err != nil {
		t.Fatal(err)
	}
	return instance, record.ID
}
func rootStartCommand(t *testing.T, f recordFixture, instance string) (fc.Command, fc.ExecutionPayload) {
	t.Helper()
	var raw, payload []byte
	if err := f.owner.QueryRow(f.ctx, "SELECT command_json,execution_payload FROM applications.workflow_commands WHERE command_json->>'ProtocolVersion'='2' AND command_json->>'InstanceID'=$1 AND command_json->>'Action'='start'", instance).Scan(&raw, &payload); err != nil {
		t.Fatal(err)
	}
	var command fc.Command
	if json.Unmarshal(raw, &command) != nil {
		t.Fatal("bad stored command")
	}
	p, err := fc.DecodeExecutionPayload("start", payload)
	if err != nil {
		t.Fatal(err)
	}
	return command, p
}
func TestRootWorkflowStartAcceptsDurableIntentOnce(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, record := rootStartIntent(t, f, f.actor)
	worked, err := f.service.acceptWorkflowStart(f.ctx, instance)
	if err != nil || !worked {
		t.Fatalf("durable start not accepted: %v %v", worked, err)
	}
	c, p := rootStartCommand(t, f, instance)
	if c.RecordID != record || c.ActorID != f.actor || c.ExpectedSequence != 0 || c.RecordVersion != 1 || c.FenceEpoch < 1 || p.Start == nil || !p.Start.AllowWithdraw {
		t.Fatalf("wrong trusted start: %+v %+v", c, p)
	}
	if worked, err = f.service.acceptWorkflowStart(f.ctx, instance); err != nil || worked {
		t.Fatalf("duplicate accepted: %v %v", worked, err)
	}
	var count int
	if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'InstanceID'=$1", instance).Scan(&count); err != nil || count != 1 {
		t.Fatalf("commands=%d %v", count, err)
	}
}
func TestRootWorkflowStartConcurrentAdmitters(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, _ := rootStartIntent(t, f, f.actor)
	var wg sync.WaitGroup
	results := make(chan bool, 4)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := f.service.acceptWorkflowStart(f.ctx, instance)
			results <- ok
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	n := 0
	for v := range results {
		if v {
			n++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n != 1 {
		t.Fatalf("accepted=%d want1", n)
	}
}
func TestRootWorkflowStartAcceptedInitiatorDeactivationDoesNotLoseIntent(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, _ := rootStartIntent(t, f, f.other)
	if _, err := f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE id=$1", f.actor); err != nil {
		t.Fatal(err)
	}
	ok, err := f.service.acceptWorkflowStart(f.ctx, instance)
	if err != nil || !ok {
		t.Fatalf("accepted intent tied to expired user session: %v %v", ok, err)
	}
	c, _ := rootStartCommand(t, f, instance)
	if c.ActorID != f.actor {
		t.Fatal("initiator replaced")
	}
}
func TestRootWorkflowStartApproverRevokedThenRestored(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, _ := rootStartIntent(t, f, f.actor)
	if _, err := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.service.acceptWorkflowStart(f.ctx, instance); ok {
		t.Fatal("revoked approver accepted")
	}
	var n int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'InstanceID'=$1", instance).Scan(&n); err != nil || n != 0 {
		t.Fatalf("revoked command leaked %d %v", n, err)
	}
	if _, err := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=true WHERE app_id=$1", f.app); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.service.acceptWorkflowStart(f.ctx, instance); err != nil || !ok {
		t.Fatalf("restored intent cannot resume: %v %v", ok, err)
	}
}

func TestRootWorkflowStartSystemEvidenceHasNoUserVisibleMask(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, _ := rootStartIntent(t, f, f.other)
	// Creation needs no data.read grant. Removing read after acceptance cannot
	// manufacture a user-visible read mask for system background evidence.
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields gf USING applications.grants g WHERE gf.app_id=g.app_id AND gf.grant_id=g.id AND g.app_id=$1 AND g.action='data.read'", f.app); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read'", f.app); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.service.acceptWorkflowStart(f.ctx, instance); err != nil || !ok {
		t.Fatalf("create-only intent blocked %v %v", ok, err)
	}
	_, payload := rootStartCommand(t, f, instance)
	var body []byte
	if err := f.owner.QueryRow(f.ctx, "SELECT body FROM applications.workflow_evidence_documents WHERE app_id=$1 AND evidence_hash=$2", f.app, payload.EvidenceHash[:]).Scan(&body); err != nil {
		t.Fatal(err)
	}
	manifest, err := ev.DecodeManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.VisibleFieldIDs) != 0 || len(manifest.Fields) != 3 {
		t.Fatalf("system evidence changed user read access or omitted audit fields: %+v", manifest)
	}
}
func TestRootWorkflowStartOwnReadCannotApproveForeignRecord(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	// Keep the existing own grant; remove all-scope reads to avoid duplicate tuples.
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields gf USING applications.grants g WHERE gf.app_id=g.app_id AND gf.grant_id=g.id AND g.app_id=$1 AND g.action='data.read' AND g.row_scope='all'", f.app); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'", f.app); err != nil {
		t.Fatal(err)
	}
	f.principal.UserID = f.other
	instance, _ := rootStartIntent(t, f, f.actor)
	if ok, _ := f.service.acceptWorkflowStart(f.ctx, instance); ok {
		t.Fatal("own-scope approver admitted on foreign-owned record")
	}
	if _, err := f.owner.Exec(f.ctx, "UPDATE applications.grants SET row_scope='all' WHERE app_id=$1 AND action='data.read'", f.app); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.service.acceptWorkflowStart(f.ctx, instance); err != nil || !ok {
		t.Fatalf("restored foreign read did not resume %v %v", ok, err)
	}
}
func TestRootWorkflowStartUsesLatestRecordEvidence(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, record := rootStartIntent(t, f, f.actor)
	// The owner fixture models a later authorized save from another workflow.
	// It does not assert that ordinary editing is permitted while in flight.
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	col := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	if _, err := f.owner.Exec(f.ctx, "UPDATE "+relation+" SET "+col+"='latest',record_version=2 WHERE id=$1", record); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.service.acceptWorkflowStart(f.ctx, instance); err != nil || !ok {
		t.Fatalf("latest acceptance %v %v", ok, err)
	}
	command, payload := rootStartCommand(t, f, instance)
	var body []byte
	if err := f.owner.QueryRow(f.ctx, "SELECT body FROM applications.workflow_evidence_documents WHERE app_id=$1 AND evidence_hash=$2", f.app, payload.EvidenceHash[:]).Scan(&body); err != nil {
		t.Fatal(err)
	}
	manifest, err := ev.DecodeManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if command.RecordVersion != 2 || manifest.Header.RecordVersion != 2 {
		t.Fatal("start pinned stale business record")
	}
}
func TestRootWorkflowStartCommitFailureLeavesRecoverableIntent(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, _ := rootStartIntent(t, f, f.actor)
	name := "v044_start_fault_" + strings.ReplaceAll(f.app, "-", "")
	fn := pgx.Identifier{"applications", name}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	sql := "CREATE FUNCTION " + fn + "() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_json->>'AppID'='" + f.app + "' THEN RAISE EXCEPTION 'isolated start commit fault' USING ERRCODE='P0001'; END IF; RETURN NEW; END $$"
	if _, err := f.owner.Exec(f.ctx, sql); err != nil {
		t.Fatal(err)
	}
	defer f.owner.Exec(f.ctx, "DROP FUNCTION "+fn+"() CASCADE")
	if _, err := f.owner.Exec(f.ctx, "CREATE CONSTRAINT TRIGGER "+trigger+" AFTER INSERT ON applications.workflow_commands DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION "+fn+"()"); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.service.acceptWorkflowStart(f.ctx, instance); ok || err == nil {
		t.Fatalf("commit fault was treated as accepted %v %v", ok, err)
	}
	for _, table := range []string{"record_command_fences", "workflow_evidence_documents", "workflow_evidence_blobs"} {
		var n int
		if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1", f.app).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s escaped rollback: %d %v", table, n, err)
		}
	}
	var n int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'InstanceID'=$1", instance).Scan(&n); err != nil || n != 0 {
		t.Fatalf("ledger escaped rollback %d %v", n, err)
	}
	var state string
	if err := f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE id=$1", instance).Scan(&state); err != nil || state != "starting" {
		t.Fatalf("accepted intent lost: %s %v", state, err)
	}
}
