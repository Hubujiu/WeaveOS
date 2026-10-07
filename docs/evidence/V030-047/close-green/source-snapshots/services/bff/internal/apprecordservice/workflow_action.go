package apprecordservice

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"time"
)

type WorkflowTaskActionRequest struct {
	WorkflowTaskRequest
	OperationID, Action, BasisToken string
}
type WorkflowOperationResult struct {
	OperationID   string `json:"operationId"`
	CommandID     string `json:"commandId"`
	InstanceID    string `json:"instanceId"`
	Status        string `json:"status"`
	InstanceState string `json:"instanceState,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Sequence      int64  `json:"sequence,omitempty"`
}

func workflowActionToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == token
}
func workflowNewCommandID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 15) | 64
	raw[8] = (raw[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:]), nil
}
func (s *Service) AcceptWorkflowTask(ctx context.Context, principal session.Principal, req WorkflowTaskActionRequest, metadata applications.Metadata) (WorkflowOperationResult, error) {
	empty := WorkflowOperationResult{}
	if s == nil || s.Pool == nil || s.WorkflowBases == nil || !validWriteLimits(s.Limits) {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || strings.TrimSpace(metadata.RequestID) == "" || (req.Action != "agree" && req.Action != "reject") {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.InstanceID, req.TaskID, req.OperationID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	// Bound unaccepted malformed input before constructing a request fingerprint.
	if len(req.BasisToken) > 2048 {
		return empty, ErrWorkflowBasisExpired
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return empty, applications.ErrInvalid
	}
	fingerprint := sha256.Sum256(raw)
	runtimeReady := true
	if s.RuntimeReady != nil {
		runtimeReady = s.RuntimeReady(ctx) == nil
	}
	var basis querycontext.Metadata
	loadErr := querycontext.ErrExpired
	if workflowActionToken(req.BasisToken) {
		basis, loadErr = s.WorkflowBases.Load(ctx, principal.SessionRef, req.BasisToken)
	}
	kind := "workflow.task." + req.Action
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: req.OperationID, Kind: kind, Fingerprint: fingerprint,
		Authorize: func(_ context.Context, _ pgx.Tx, facts applications.RecordContext) error {
			policy, menu := policyFor(facts)
			if !menu || policy.VisibleScope() == appaccess.None {
				return applications.ErrDenied
			}
			return nil
		}}
	write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, principal, req.AppID, req.ViewID, options)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = write.Rollback(cleanup)
	}()
	old, err := write.Replay(ctx, req.OperationID, kind, fingerprint)
	if err != nil {
		return empty, err
	}
	if old != nil {
		if err = write.Commit(ctx); err != nil {
			return empty, err
		}
		return s.WorkflowOperation(ctx, principal, req.OperationID)
	}
	if loadErr != nil {
		if errors.Is(loadErr, querycontext.ErrExpired) || errors.Is(loadErr, querycontext.ErrInvalid) {
			return empty, ErrWorkflowBasisExpired
		}
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		return empty, ErrUnavailable
	}
	var binding workflowBasisBinding
	var revision workflowBasisRevision
	if !workflowClosedMetadata(basis.Criteria, 2048, 11, &binding) || !workflowClosedMetadata(basis.Revision, 128, 2, &revision) {
		return empty, ErrWorkflowBasisExpired
	}
	if binding.AppID != req.AppID || binding.ViewID != req.ViewID || binding.RecordID != req.RecordID || binding.InstanceID != req.InstanceID || binding.TaskID != req.TaskID {
		return empty, ErrWorkflowBasisChanged
	}
	if err = write.Claim(ctx, req.OperationID, kind, fingerprint); err != nil {
		return empty, err
	}
	facts := write.Context()
	tx := write.Tx()
	if binding.TableID != facts.TableID {
		return empty, ErrWorkflowBasisChanged
	}
	if err = (appstructure.RecordGate{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID}).LockTable(ctx, tx, facts.TableID, facts.SchemaVersion); err != nil {
		return empty, err
	}
	var flowID, versionID, nodeID, assignee, state string
	var definitionVersion, sequence, epoch int64
	var closed bool
	var graphJSON []byte
	err = tx.QueryRow(ctx, `SELECT i.flow_id::text,i.definition_version,i.sequence,i.state,v.version_id::text,v.graph_json,
 t.node_id::text,t.assignee_id::text,t.activation_epoch,t.closed_command_id IS NOT NULL
 FROM applications.workflow_instances i
 JOIN applications.workflow_versions v ON v.app_id=i.app_id AND v.flow_id=i.flow_id AND v.version=i.definition_version
 JOIN applications.workflow_tasks t ON t.app_id=i.app_id AND t.instance_id=i.id
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.view_id=$3 AND i.record_id=$4 AND i.id=$5 AND t.id=$6
 FOR UPDATE OF i,t`, req.AppID, facts.TableID, req.ViewID, req.RecordID, req.InstanceID, req.TaskID).Scan(&flowID, &definitionVersion, &sequence, &state, &versionID, &graphJSON, &nodeID, &assignee, &epoch, &closed)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrWorkflowTaskChanged
	}
	if err != nil {
		return empty, err
	}
	if assignee != facts.Actor.ID {
		return empty, applications.ErrDenied
	}
	if !workflowID(flowID) || !workflowApprovalRoster(graphJSON, nodeID, assignee) {
		return empty, ErrUnavailable
	}
	bundle, err := captureWorkflowEvidence(ctx, tx, facts, req.RecordID)
	if err != nil {
		return empty, err
	}
	if closed || state != "active" || epoch != binding.ActivationEpoch || sequence != binding.Sequence {
		return empty, ErrWorkflowTaskChanged
	}
	if definitionVersion != binding.DefinitionVersion || versionID != binding.VersionID || nodeID != binding.NodeID {
		return empty, ErrWorkflowTaskChanged
	}
	manifest, err := ev.DecodeManifest(bundle.Manifest)
	if err != nil {
		return empty, ErrUnavailable
	}
	h := manifest.Header
	if hex.EncodeToString(bundle.Hash[:]) != basis.Fingerprint || h.SchemaVersion != revision.SchemaVersion || h.RecordVersion != revision.RecordVersion {
		return empty, ErrWorkflowBasisChanged
	}
	if err = (appstructure.RecordFence{AppID: req.AppID, TableID: facts.TableID}).Check(ctx, tx, facts.TableID, req.RecordID, h.RecordVersion); err != nil {
		return empty, err
	}
	routes := map[string]bool{}
	if req.Action == "agree" {
		routes, err = workflowActionRoutes(ctx, tx, facts, req.RecordID, graphJSON)
		if err != nil {
			return empty, err
		}
	}
	if !runtimeReady {
		return empty, ErrUnavailable
	}
	commandID, err := workflowNewCommandID()
	if err != nil {
		return empty, err
	}
	if _, err = (ev.Store{}).PutInTx(ctx, tx, bundle); err != nil {
		return empty, err
	}
	var fenceEpoch int64
	if err = tx.QueryRow(ctx, "SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)", req.AppID, facts.TableID, req.ViewID, req.RecordID, commandID, h.SchemaVersion, h.RecordVersion).Scan(&fenceEpoch); err != nil {
		return empty, err
	}
	payload := fc.ExecutionPayload{EvidenceHash: bundle.Hash, Routes: routes}
	payloadBytes, err := fc.EncodeExecutionPayload(req.Action, payload)
	if err != nil {
		return empty, err
	}
	command := fc.Command{ProtocolVersion: 2, CommandID: commandID, AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID, RecordID: req.RecordID, InstanceID: req.InstanceID, TaskID: req.TaskID, ActorID: facts.Actor.ID, Action: req.Action, RecordVersion: h.RecordVersion, SchemaVersion: h.SchemaVersion, FenceEpoch: fenceEpoch, TaskEpoch: epoch, ExpectedSequence: sequence, FlowID: flowID, VersionID: versionID, DefinitionVersion: definitionVersion, PayloadHash: sha256.Sum256(payloadBytes)}
	if _, err = (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(ctx, tx, command, payload); err != nil {
		return empty, err
	}
	result := WorkflowOperationResult{OperationID: req.OperationID, CommandID: commandID, InstanceID: req.InstanceID, Status: "pending"}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return empty, err
	}
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 202, Location: "/api/v1/application-workflow-operations/" + req.OperationID, Data: resultBytes}); err != nil {
		return empty, err
	}
	if err = write.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}

func workflowActionRoutes(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, recordID string, raw []byte) (map[string]bool, error) {
	var graph flowgraph.Graph
	if !closedJSON(raw, &graph) {
		return nil, ErrUnavailable
	}
	definitions, err := captureDefinitions(facts.Fields)
	if err != nil {
		return nil, err
	}
	fields := make([]appquery.Field, len(definitions))
	for i, f := range definitions {
		fields[i] = appquery.Field{ID: f.ID, Kind: appquery.FieldKind(f.Kind)}
	}
	nodes := []flowgraph.Node{}
	for _, n := range graph.Nodes {
		if n.Kind == "condition" {
			nodes = append(nodes, n)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	routes := make(map[string]bool, len(nodes))
	if len(nodes) == 0 {
		return routes, nil
	}
	args := []any{recordID}
	predicates := make([]string, len(nodes))
	for i, node := range nodes {
		condition := bytes.TrimSpace(node.Condition)
		if len(condition) == 0 || bytes.Equal(condition, []byte("null")) {
			return nil, ErrUnavailable
		}
		plan, e := appquery.Compile(condition, nil, fields, len(args)+1)
		if e != nil {
			return nil, ErrUnavailable
		}
		predicates[i] = "COALESCE((" + plan.Predicate + "),false)"
		args = append(args, plan.Arguments...)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
	var results []bool
	if err = tx.QueryRow(ctx, "SELECT ARRAY["+strings.Join(predicates, ",")+"] FROM "+relation+" r WHERE r.id=$1::uuid", args...).Scan(&results); errors.Is(err, pgx.ErrNoRows) {
		return nil, applications.ErrMissing
	} else if err != nil {
		return nil, err
	}
	if len(results) != len(nodes) {
		return nil, ErrUnavailable
	}
	for i, node := range nodes {
		routes[node.ID] = results[i]
	}
	return routes, nil
}

func (s *Service) WorkflowOperation(ctx context.Context, principal session.Principal, operationID string) (WorkflowOperationResult, error) {
	empty := WorkflowOperationResult{}
	if s == nil || s.Pool == nil {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || !workflowID(operationID) {
		return empty, applications.ErrInvalid
	}
	// The existing operation lifecycle checks the live Session and original actor.
	if _, err := (&applications.Application{Pool: s.Pool}).Operation(ctx, principal, operationID); err != nil {
		return empty, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var appID, kind, location string
	var status int
	var raw, fingerprint []byte
	err = tx.QueryRow(ctx, `SELECT app_id::text,operation_kind,result_json,http_status,location,fingerprint
 FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2`, principal.UserID, operationID).Scan(&appID, &kind, &raw, &status, &location, &fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	if kind != "workflow.task.agree" && kind != "workflow.task.reject" {
		return empty, applications.ErrMissing
	}
	var result WorkflowOperationResult
	if !workflowClosedMetadata(raw, 1024, 4, &result) || result.OperationID != operationID || !workflowID(result.CommandID) || !workflowID(result.InstanceID) || result.Status != "pending" || result.InstanceState != "" || result.Reason != "" || result.Sequence != 0 || status != 202 || location != "/api/v1/application-workflow-operations/"+operationID || len(fingerprint) != 32 {
		return empty, ErrUnavailable
	}
	ledger := fc.Ledger{Namespace: "applications"}
	entry, err := ledger.GetInTx(ctx, tx, result.CommandID)
	if err != nil {
		return empty, err
	}
	command := entry.Command
	if command.ProtocolVersion != 2 || command.AppID != appID || command.ActorID != principal.UserID || command.InstanceID != result.InstanceID || kind != "workflow.task."+command.Action {
		return empty, ErrUnavailable
	}
	payload, err := ledger.ExecutionPayloadInTx(ctx, tx, command)
	if err != nil {
		return empty, err
	}
	if payload.Start != nil {
		return empty, ErrUnavailable
	}
	if entry.State == "pending" {
		if entry.Receipt != nil {
			return empty, ErrUnavailable
		}
	} else {
		if entry.Receipt == nil || (entry.State != "success" && entry.State != "no_effect") || entry.State != entry.Receipt.Outcome {
			return empty, ErrUnavailable
		}
		var eventApp, eventInstance, eventActor, eventAction, outcome, proof string
		var sequence, schemaVersion, recordVersion int64
		var evidence, payloadBytes, resultBytes []byte
		err = tx.QueryRow(ctx, `SELECT app_id::text,instance_id::text,actor_id::text,action,outcome,sequence,schema_version,record_version,evidence_hash,payload_bytes,result_bytes,proof_id::text FROM applications.workflow_execution_events WHERE command_id=$1`, command.CommandID).Scan(&eventApp, &eventInstance, &eventActor, &eventAction, &outcome, &sequence, &schemaVersion, &recordVersion, &evidence, &payloadBytes, &resultBytes, &proof)
		if err != nil {
			return empty, ErrUnavailable
		}
		encoded, e := fc.EncodeExecutionPayload(command.Action, payload)
		if e != nil || eventApp != command.AppID || eventInstance != command.InstanceID || eventActor != command.ActorID || eventAction != command.Action || outcome != entry.State || sequence != entry.Receipt.Sequence || schemaVersion != command.SchemaVersion || recordVersion != command.RecordVersion || proof != entry.Receipt.ProofID || !bytes.Equal(evidence, payload.EvidenceHash[:]) || !bytes.Equal(payloadBytes, encoded) || sha256.Sum256(payloadBytes) != command.PayloadHash {
			return empty, ErrUnavailable
		}
		decoded, e := fc.DecodeExecutionResult(command, *entry.Receipt, resultBytes)
		if e != nil {
			return empty, e
		}
		result.Status = outcome
		result.InstanceState = decoded.State
		result.Reason = decoded.Reason
		result.Sequence = sequence
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}
