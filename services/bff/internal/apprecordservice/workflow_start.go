package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
)

const workflowStartZero = "00000000-0000-0000-0000-000000000000"

// NewStartAdmitter owns only a scheduling cursor, never durable workflow state.
// Multiple processes/closures are safe: the database locks and unique start
// command constraint arbitrate admission. Bad candidates do not pin the cursor.
func (s *Service) NewStartAdmitter() func(context.Context) (bool, error) {
	var mu sync.Mutex
	cursor := workflowStartZero
	return func(ctx context.Context) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		if s == nil || s.Pool == nil || ctx == nil || !validWriteLimits(s.Limits) {
			return false, ErrUnavailable
		}
		var id string
		err := s.Pool.QueryRow(ctx, `SELECT i.id::text FROM applications.workflow_instances i
   WHERE i.state='starting' AND i.id>$1::uuid
   AND NOT EXISTS(SELECT 1 FROM applications.workflow_commands c WHERE c.command_json->>'ProtocolVersion'='2' AND c.command_json->>'Action'='start' AND c.command_json->>'InstanceID'=i.id::text)
   AND NOT EXISTS(SELECT 1 FROM applications.record_command_fences f WHERE f.app_id=i.app_id AND f.table_id=i.table_id AND f.record_id=i.record_id)
   ORDER BY i.id LIMIT 1`, cursor).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			cursor = workflowStartZero
			return false, nil
		}
		if err != nil {
			return false, err
		}
		cursor = id
		return s.acceptWorkflowStart(ctx, id)
	}
}
func (s *Service) acceptWorkflowStart(ctx context.Context, instanceID string) (bool, error) {
	if s == nil || s.Pool == nil || ctx == nil || !validWriteLimits(s.Limits) || !workflowID(instanceID) {
		return false, ErrUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)", strconv.FormatInt(s.Limits.LockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(s.Limits.StatementTimeout.Milliseconds(), 10)+"ms"); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err != nil {
		return false, err
	}
	facts, err := applications.LockAcceptedWorkflowInTx(ctx, tx, instanceID)
	if err != nil {
		return false, err
	}
	// The table lock already serializes all governed record mutations. Acquire
	// the row through its existing owner capability; auth_app deliberately has
	// no direct typed-table UPDATE privilege (SELECT FOR UPDATE would violate it).
	header, err := (appstructure.RecordDML{}).LockHeader(ctx, tx, appstructure.RecordTable{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, SchemaVersion: facts.SchemaVersion}, facts.RecordID)
	if err != nil {
		return false, err
	}
	facts.RecordVersion = header.RecordVersion
	var locked string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM applications.workflow_instances WHERE id=$1 FOR UPDATE", instanceID).Scan(&locked); err != nil {
		return false, err
	}
	if facts.State != "starting" || facts.Sequence != 0 {
		return false, nil
	}
	var occupied bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_commands WHERE command_json->>'ProtocolVersion'='2' AND command_json->>'Action'='start' AND command_json->>'InstanceID'=$1)
 OR EXISTS(SELECT 1 FROM applications.record_command_fences WHERE app_id=$2 AND table_id=$3 AND record_id=$4)`, instanceID, facts.App.ID, facts.TableID, facts.RecordID).Scan(&occupied); err != nil {
		return false, err
	}
	if occupied {
		return false, nil
	}
	var graph flowgraph.Graph
	if !closedJSON(facts.Graph, &graph) {
		return false, ErrUnavailable
	}
	for i := range graph.Nodes {
		if bytes.Equal(bytes.TrimSpace(graph.Nodes[i].Condition), []byte("null")) {
			graph.Nodes[i].Condition = nil
		}
	}
	definitions, err := captureDefinitions(facts.Fields)
	if err != nil {
		return false, err
	}
	fields := make([]appquery.Field, len(definitions))
	ids := make([]string, len(definitions))
	for i, f := range definitions {
		fields[i] = appquery.Field{ID: f.ID, Kind: appquery.FieldKind(f.Kind)}
		ids[i] = f.ID
	}
	if _, err = flowgraph.Validate(graph, fields); err != nil {
		return false, err
	}
	approvers := map[string][]string{}
	checked := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind != "approval" || node.Approval == nil {
			continue
		}
		for _, actor := range node.Approval.AssigneeIDs {
			if checked[actor] {
				continue
			}
			actorFacts, e := applications.WorkflowApproverContextInTx(ctx, tx, instanceID, actor)
			if e != nil {
				return false, e
			}
			policy, menu := policyFor(actorFacts)
			if !menu || policy.VisibleScope() == appaccess.None || len(policy.ReadFields(facts.CreatedBy, ids)) == 0 {
				return false, applications.ErrDenied
			}
			checked[actor] = true
		}
		approvers[node.ID] = append([]string{}, node.Approval.AssigneeIDs...)
	}
	bundle, err := captureWorkflowEvidenceMode(ctx, tx, facts.RecordContext, facts.RecordID, true)
	if err != nil {
		return false, err
	}
	routes, err := workflowActionRoutes(ctx, tx, facts.RecordContext, facts.RecordID, facts.Graph)
	if err != nil {
		return false, err
	}
	if _, err = (ev.Store{}).PutInTx(ctx, tx, bundle); err != nil {
		return false, err
	}
	commandID, err := workflowNewCommandID()
	if err != nil {
		return false, err
	}
	var fence int64
	if err = tx.QueryRow(ctx, "SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)", facts.App.ID, facts.TableID, facts.ViewID, facts.RecordID, commandID, facts.SchemaVersion, facts.RecordVersion).Scan(&fence); err != nil {
		return false, err
	}
	payload := fc.ExecutionPayload{EvidenceHash: bundle.Hash, Start: &fc.ExecutionStart{AllowWithdraw: facts.AllowWithdraw, Approvers: approvers}, Routes: routes}
	encoded, err := fc.EncodeExecutionPayload("start", payload)
	if err != nil {
		return false, err
	}
	command := fc.Command{ProtocolVersion: 2, CommandID: commandID, AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, RecordID: facts.RecordID, InstanceID: instanceID, ActorID: facts.Actor.ID, Action: "start", SchemaVersion: facts.SchemaVersion, RecordVersion: facts.RecordVersion, FenceEpoch: fence, FlowID: facts.FlowID, VersionID: facts.VersionID, DefinitionVersion: facts.DefinitionVersion, PayloadHash: sha256.Sum256(encoded)}
	if _, err = (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(ctx, tx, command, payload); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
