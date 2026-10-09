package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type WorkflowEventRequest struct{ AppID, ViewID, RecordID, EventID string }
type WorkflowEventSummary struct {
	FlowName       string  `json:"flowName"`
	FlowNameSource string  `json:"flowNameSource"`
	ID             string  `json:"id"`
	InstanceID     string  `json:"instanceId"`
	FlowID         string  `json:"flowId"`
	NodeID         *string `json:"nodeId"`
	TargetNodeID   *string `json:"targetNodeId"`
	ActorID        string  `json:"actorId"`
	Action         string  `json:"action"`
	Outcome        string  `json:"outcome"`
	Sequence       int64   `json:"sequence"`
	SchemaVersion  int64   `json:"schemaVersion"`
	RecordVersion  int64   `json:"recordVersion"`
	OccurredAt     string  `json:"occurredAt"`
}
type WorkflowHistoricalLabel struct {
	Label   string `json:"label"`
	Deleted bool   `json:"deleted"`
}
type WorkflowHistoricalField struct {
	FieldID     string                             `json:"fieldId"`
	FieldName   string                             `json:"fieldName"`
	FieldKind   string                             `json:"fieldKind"`
	Value       json.RawMessage                    `json:"value"`
	ValueLabels map[string]WorkflowHistoricalLabel `json:"valueLabels"`
}
type WorkflowEventBasis struct {
	Status string                    `json:"status"`
	Fields []WorkflowHistoricalField `json:"fields"`
}
type WorkflowEventResult struct {
	Event WorkflowEventSummary `json:"event"`
	Basis WorkflowEventBasis   `json:"basis"`
}

func (s *Service) ReadWorkflowEvent(ctx context.Context, p session.Principal, q WorkflowEventRequest) (WorkflowEventResult, error) {
	empty := WorkflowEventResult{}
	if s == nil || s.Pool == nil {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{q.AppID, q.ViewID, q.RecordID, q.EventID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	a, err := s.beginWorkflowHistoryRead(ctx, p, q)
	if err != nil {
		return empty, err
	}
	tx, facts, creator, policy, currentIDs, full := a.tx, a.facts, a.creator, a.policy, a.currentIDs, a.full
	defer tx.Rollback(context.Background())
	result, err := readWorkflowEventInTx(ctx, tx, facts, q, creator, policy, currentIDs, full)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, ErrUnavailable
	}
	return result, nil
}

type workflowHistoryRead struct {
	tx         pgx.Tx
	facts      applications.RecordContext
	creator    string
	policy     appaccess.Policy
	currentIDs []string
	full       bool
}

func (s *Service) beginWorkflowHistoryRead(ctx context.Context, p session.Principal, q WorkflowEventRequest) (*workflowHistoryRead, error) {
	tx, facts, err := (&applications.Application{Pool: s.Pool}).BeginRecordRead(ctx, p, q.AppID, q.ViewID)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = tx.Rollback(context.Background())
		}
	}()
	policy, menu := policyFor(facts)
	if !menu {
		return nil, applications.ErrDenied
	}
	if policy.VisibleScope() == appaccess.None {
		return nil, applications.ErrMissing
	}
	if !facts.SchemaReady {
		return nil, &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
	}
	_, currentIDs, err := fieldsInContext(facts)
	if err != nil {
		return nil, ErrUnavailable
	}
	var creator string
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
	err = tx.QueryRow(ctx, "SELECT created_by::text FROM "+relation+" WHERE id=$1::uuid", q.RecordID).Scan(&creator)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, applications.ErrMissing
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	full := facts.Actor.BootstrapAdmin || facts.Actor.ID == facts.App.OwnerUserID
	if len(policy.ReadFields(creator, currentIDs)) == 0 && !full {
		return nil, applications.ErrMissing
	}
	if policy.HistoryScope() == appaccess.None || len(policy.HistoryFields(creator, currentIDs)) == 0 && !full {
		return nil, applications.ErrDenied
	}
	ok = true
	return &workflowHistoryRead{tx: tx, facts: facts, creator: creator, policy: policy, currentIDs: currentIDs, full: full}, nil
}

func readWorkflowEventInTx(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, q WorkflowEventRequest, creator string, policy appaccess.Policy, currentIDs []string, full bool) (WorkflowEventResult, error) {
	empty := WorkflowEventResult{}
	out := WorkflowEventResult{Basis: WorkflowEventBasis{Status: "unavailable", Fields: []WorkflowHistoricalField{}}}
	e := &out.Event
	var sourceView, proof, versionID string
	var taskID *string
	var taskEpoch int64
	var definitionVersion int64
	var occurred time.Time
	var hash, payloadRaw, resultRaw []byte
	err := tx.QueryRow(ctx, `SELECT e.command_id::text,e.instance_id::text,e.flow_id::text,e.view_id::text,
 e.definition_version,e.actor_id::text,e.action,e.outcome,e.sequence,e.schema_version,e.record_version,
 e.created_at,e.evidence_hash,e.payload_bytes,e.result_bytes,e.proof_id::text,
 e.version_id::text,e.task_id::text,e.task_epoch,e.node_id::text,e.target_node_id::text,e.flow_name,e.flow_name_source
 FROM applications.workflow_execution_events e
 WHERE e.app_id=$1::uuid AND e.command_id=$2::uuid AND e.table_id=$3::uuid AND e.record_id=$4::uuid`, q.AppID, q.EventID, facts.TableID, q.RecordID).Scan(
		&e.ID, &e.InstanceID, &e.FlowID, &sourceView, &definitionVersion, &e.ActorID, &e.Action, &e.Outcome, &e.Sequence, &e.SchemaVersion, &e.RecordVersion, &occurred, &hash, &payloadRaw, &resultRaw, &proof, &versionID, &taskID, &taskEpoch, &e.NodeID, &e.TargetNodeID, &e.FlowName, &e.FlowNameSource)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, ErrUnavailable
	}
	entry, err := (fc.Ledger{Namespace: "applications"}).GetInTx(ctx, tx, q.EventID)
	if err != nil {
		return empty, ErrUnavailable
	}
	c, r := entry.Command, entry.Receipt
	if !validWorkflowJournalMetadata(*e, c, versionID, taskID, taskEpoch) || c.ProtocolVersion != 2 || c.AppID != q.AppID || c.TableID != facts.TableID || c.ViewID != sourceView || c.RecordID != q.RecordID || c.InstanceID != e.InstanceID || c.FlowID != e.FlowID || c.DefinitionVersion != definitionVersion || c.ActorID != e.ActorID || c.Action != e.Action || c.SchemaVersion != e.SchemaVersion || c.RecordVersion != e.RecordVersion || r == nil || entry.State != e.Outcome || r.Outcome != e.Outcome || r.Sequence != e.Sequence || r.ProofID != proof || len(hash) != 32 || occurred.IsZero() {
		return empty, ErrUnavailable
	}
	if sha256.Sum256(payloadRaw) != c.PayloadHash {
		return empty, ErrUnavailable
	}
	payload, err := fc.DecodeExecutionPayload(c.Action, payloadRaw)
	if err != nil || !bytes.Equal(hash, payload.EvidenceHash[:]) {
		return empty, ErrUnavailable
	}
	if _, err = fc.DecodeExecutionResult(c, *r, resultRaw); err != nil {
		return empty, ErrUnavailable
	}
	e.OccurredAt = occurred.UTC().Format(time.RFC3339Nano)
	store := ev.Store{}
	m, err := store.ManifestInTx(ctx, tx, q.AppID, payload.EvidenceHash)
	if errors.Is(err, ev.ErrMissing) {
		return out, nil
	}
	if err != nil {
		return empty, ErrUnavailable
	}
	h := m.Header
	if h.AppID != q.AppID || h.TableID != facts.TableID || h.ViewID != sourceView || h.RecordID != q.RecordID || h.CreatedBy != creator || h.SchemaVersion != e.SchemaVersion || h.RecordVersion != e.RecordVersion {
		return empty, ErrUnavailable
	}
	current := make(map[string]bool, len(currentIDs))
	for _, id := range currentIDs {
		current[id] = true
	}
	eligible := make([]string, 0, len(m.VisibleFieldIDs))
	for _, id := range m.VisibleFieldIDs {
		if full || current[id] {
			eligible = append(eligible, id)
		}
	}
	ids := policy.HistoryFields(creator, eligible)
	fields, err := store.FieldsInTx(ctx, tx, q.AppID, payload.EvidenceHash, ids)
	if err != nil {
		return empty, ErrUnavailable
	}
	out.Basis.Status = "available"
	for _, f := range fields {
		v := WorkflowHistoricalField{FieldID: f.Definition.ID, FieldName: f.Definition.Name, FieldKind: f.Definition.Kind, Value: f.Value, ValueLabels: map[string]WorkflowHistoricalLabel{}}
		if f.Reference != nil {
			v.ValueLabels[f.Reference.ID] = WorkflowHistoricalLabel{Label: f.Reference.Label, Deleted: f.Reference.Deleted}
		}
		for _, label := range f.OptionDisplays {
			v.ValueLabels[label.ID] = WorkflowHistoricalLabel{Label: label.Label, Deleted: label.Deleted}
		}
		out.Basis.Fields = append(out.Basis.Fields, v)
	}
	return out, nil
}

// Checks stable journal metadata against the original canonical command. Current
// catalog/task rows are deliberately not part of historical interpretation.
func validWorkflowJournalMetadata(e WorkflowEventSummary, c fc.Command, version string, task *string, epoch int64) bool {
	if c.VersionID != version || !workflowID(version) || epoch != c.TaskEpoch || strings.TrimSpace(e.FlowName) == "" || len([]rune(e.FlowName)) > 100 || (e.FlowNameSource != "captured" && e.FlowNameSource != "legacy_last_known") {
		return false
	}
	if c.TaskID == "" {
		if task != nil || e.NodeID != nil || epoch != 0 {
			return false
		}
	} else {
		if task == nil || *task != c.TaskID || !workflowID(*task) || epoch < 1 {
			return false
		}
		if e.NodeID == nil {
			if e.Outcome != "no_effect" {
				return false
			}
		} else if !workflowID(*e.NodeID) {
			return false
		}
	}
	if c.TargetNodeID == "" {
		return e.TargetNodeID == nil
	}
	return e.TargetNodeID != nil && *e.TargetNodeID == c.TargetNodeID && workflowID(*e.TargetNodeID)
}
