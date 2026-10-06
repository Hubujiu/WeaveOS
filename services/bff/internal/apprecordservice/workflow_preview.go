package apprecordservice

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"time"
)

var ErrWorkflowTaskChanged = errors.New("workflow task changed")
var ErrWorkflowBasisChanged = errors.New("workflow approval basis changed")
var ErrWorkflowBasisExpired = errors.New("workflow approval basis expired")

type WorkflowTaskRequest struct{ AppID, ViewID, RecordID, InstanceID, TaskID string }
type WorkflowTaskInfo struct {
	ID              string `json:"id"`
	InstanceID      string `json:"instanceId"`
	NodeID          string `json:"nodeId"`
	ActivationEpoch int64  `json:"activationEpoch"`
	Sequence        int64  `json:"sequence"`
}
type WorkflowTaskPreview struct {
	BasisToken string            `json:"basisToken"`
	Task       WorkflowTaskInfo  `json:"task"`
	Record     Record            `json:"record"`
	Fields     []appfields.Field `json:"fields"`
}

// workflowBasisBinding contains only immutable task identity and counters.
type workflowBasisBinding struct {
	AppID             string `json:"appId"`
	TableID           string `json:"tableId"`
	ViewID            string `json:"viewId"`
	RecordID          string `json:"recordId"`
	InstanceID        string `json:"instanceId"`
	TaskID            string `json:"taskId"`
	NodeID            string `json:"nodeId"`
	VersionID         string `json:"versionId"`
	DefinitionVersion int64  `json:"definitionVersion"`
	Sequence          int64  `json:"sequence"`
	ActivationEpoch   int64  `json:"activationEpoch"`
}
type workflowBasisRevision struct {
	SchemaVersion int64 `json:"schemaVersion"`
	RecordVersion int64 `json:"recordVersion"`
}

const workflowMaxCounter = int64(9007199254740991)

func workflowID(id string) bool {
	return appfields.ValidID(id) && id != "00000000-0000-0000-0000-000000000000"
}
func workflowPositive(n int64) bool { return n > 0 && n <= workflowMaxCounter }

// workflowClosedMetadata also requires every frozen property, including sequence.
func workflowClosedMetadata(raw []byte, limit, count int, out any) bool {
	if len(raw) == 0 || len(raw) > limit || raw[0] != '{' {
		return false
	}
	var members map[string]json.RawMessage
	if !closedJSON(raw, &members) || len(members) != count {
		return false
	}
	for _, value := range members {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
	}
	return closedJSON(raw, out)
}
func newWorkflowBasisStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-basis", generation, querycontext.Policy{Validate: func(m querycontext.Metadata) bool {
		if m.View != "workflow-task" || m.Total != 0 || m.ProtocolVersion != 1 {
			return false
		}
		var b workflowBasisBinding
		var r workflowBasisRevision
		if !workflowClosedMetadata(m.Criteria, 2048, 11, &b) || !workflowClosedMetadata(m.Revision, 128, 2, &r) {
			return false
		}
		for _, id := range []string{b.AppID, b.TableID, b.ViewID, b.RecordID, b.InstanceID, b.TaskID, b.NodeID, b.VersionID} {
			if !workflowID(id) {
				return false
			}
		}
		return workflowPositive(b.DefinitionVersion) && workflowPositive(b.ActivationEpoch) && b.Sequence >= 0 && b.Sequence <= workflowMaxCounter && workflowPositive(r.SchemaVersion) && workflowPositive(r.RecordVersion)
	}})
}

func (s *Service) PreviewWorkflowTask(ctx context.Context, principal session.Principal, req WorkflowTaskRequest) (WorkflowTaskPreview, error) {
	empty := WorkflowTaskPreview{}
	if s == nil || s.Pool == nil || s.WorkflowBases == nil || captureNilPort(ctx) {
		return empty, ErrUnavailable
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.InstanceID, req.TaskID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	tx, facts, err := (&applications.Application{Pool: s.Pool}).BeginRecordRead(ctx, principal, req.AppID, req.ViewID)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	policy, menu := policyFor(facts)
	if !menu || policy.VisibleScope() == appaccess.None {
		return empty, applications.ErrDenied
	}
	binding := workflowBasisBinding{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, RecordID: req.RecordID, InstanceID: req.InstanceID, TaskID: req.TaskID}
	var assignee, state string
	var closed bool
	var graphJSON []byte
	err = tx.QueryRow(ctx, `SELECT t.node_id::text,t.assignee_id::text,t.activation_epoch,
 t.closed_command_id IS NOT NULL,i.state,i.sequence,i.definition_version,v.version_id::text,v.graph_json
 FROM applications.workflow_instances i
 JOIN applications.workflow_tasks t ON t.app_id=i.app_id AND t.instance_id=i.id
 JOIN applications.workflow_versions v ON v.app_id=i.app_id AND v.flow_id=i.flow_id AND v.version=i.definition_version
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.view_id=$3 AND i.record_id=$4 AND i.id=$5 AND t.id=$6`,
		binding.AppID, binding.TableID, binding.ViewID, binding.RecordID, binding.InstanceID, binding.TaskID).Scan(
		&binding.NodeID, &assignee, &binding.ActivationEpoch, &closed, &state, &binding.Sequence, &binding.DefinitionVersion, &binding.VersionID, &graphJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	if assignee != facts.Actor.ID {
		return empty, applications.ErrDenied
	}
	if !workflowID(binding.NodeID) || !workflowID(binding.VersionID) || !workflowID(assignee) || !workflowPositive(binding.ActivationEpoch) || !workflowPositive(binding.DefinitionVersion) || binding.Sequence < 0 || binding.Sequence > workflowMaxCounter {
		return empty, ErrUnavailable
	}
	if !workflowApprovalRoster(graphJSON, binding.NodeID, assignee) {
		return empty, ErrUnavailable
	}
	bundle, err := captureWorkflowEvidence(ctx, tx, facts, req.RecordID)
	if err != nil {
		return empty, err
	}
	if closed || state != "active" {
		return empty, ErrWorkflowTaskChanged
	}
	preview, revision, err := workflowVisiblePreview(bundle)
	if err != nil {
		return empty, err
	}
	preview.Task = WorkflowTaskInfo{ID: binding.TaskID, InstanceID: binding.InstanceID, NodeID: binding.NodeID, ActivationEpoch: binding.ActivationEpoch, Sequence: binding.Sequence}
	criteria, err := json.Marshal(binding)
	if err != nil {
		return empty, ErrUnavailable
	}
	revisionJSON, err := json.Marshal(revision)
	if err != nil {
		return empty, ErrUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	token, err := s.WorkflowBases.Create(ctx, principal.SessionRef, querycontext.Metadata{View: "workflow-task", Criteria: criteria, Revision: revisionJSON, Total: 0, Fingerprint: hex.EncodeToString(bundle.Hash[:]), ProtocolVersion: 1})
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		return empty, ErrUnavailable
	}
	preview.BasisToken = token
	return preview, nil
}

// Validate the pinned graph's task node and roster without consulting a newer
// definition or revalidating its field references against today's schema.
func workflowApprovalRoster(raw []byte, nodeID, assignee string) bool {
	var graph flowgraph.Graph
	if !closedJSON(raw, &graph) || graph.Version != 1 || len(graph.Nodes) < 2 || len(graph.Nodes) > 100 || len(graph.Edges) < 1 || len(graph.Edges) > 200 {
		return false
	}
	seen := make(map[string]bool, len(graph.Nodes))
	found := false
	for _, node := range graph.Nodes {
		if !workflowID(node.ID) || seen[node.ID] {
			return false
		}
		seen[node.ID] = true
		switch node.Kind {
		case "start", "end", "condition":
			if node.Approval != nil {
				return false
			}
		case "approval":
			if node.Approval == nil || (node.Approval.Mode != "all" && node.Approval.Mode != "any") || len(node.Approval.AssigneeIDs) < 1 || len(node.Approval.AssigneeIDs) > 50 || len(node.Approval.EditableFieldIDs) > 200 {
				return false
			}
			if len(bytes.TrimSpace(node.Condition)) != 0 && !bytes.Equal(bytes.TrimSpace(node.Condition), []byte("null")) {
				return false
			}
			roster := make(map[string]bool, len(node.Approval.AssigneeIDs))
			for _, id := range node.Approval.AssigneeIDs {
				if !workflowID(id) || roster[id] {
					return false
				}
				roster[id] = true
			}
			fields := make(map[string]bool, len(node.Approval.EditableFieldIDs))
			for _, id := range node.Approval.EditableFieldIDs {
				if !workflowID(id) || fields[id] {
					return false
				}
				fields[id] = true
			}
			if node.ID == nodeID {
				found = roster[assignee]
			}
		default:
			return false
		}
	}
	for _, edge := range graph.Edges {
		if !seen[edge.From] || !seen[edge.To] {
			return false
		}
	}
	return found
}

func workflowVisiblePreview(bundle ev.Bundle) (WorkflowTaskPreview, workflowBasisRevision, error) {
	empty := WorkflowTaskPreview{}
	manifest, err := ev.DecodeManifest(bundle.Manifest)
	if err != nil {
		return empty, workflowBasisRevision{}, ErrUnavailable
	}
	h := manifest.Header
	preview := WorkflowTaskPreview{Fields: make([]appfields.Field, 0, len(manifest.VisibleFieldIDs)), Record: Record{ID: h.RecordID, AppID: h.AppID, TableID: h.TableID, ViewID: h.ViewID, CreatedBy: h.CreatedBy, CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt, RecordVersion: h.RecordVersion, SchemaVersion: h.SchemaVersion, Values: make(map[string]any, len(manifest.VisibleFieldIDs)), ReferenceDisplays: make(map[string]map[string]ReferenceDisplay)}}
	visible := make(map[string]bool, len(manifest.VisibleFieldIDs))
	for _, id := range manifest.VisibleFieldIDs {
		visible[id] = true
	}
	blobs := make(map[string]ev.Blob, len(bundle.Fields))
	for _, blob := range bundle.Fields {
		blobs[blob.FieldID] = blob
	}
	// Manifest field references are in deterministic field-ID order.
	for _, ref := range manifest.Fields {
		if !visible[ref.FieldID] {
			continue
		}
		blob, exists := blobs[ref.FieldID]
		if !exists || blob.Hash != ref.Hash {
			return empty, workflowBasisRevision{}, ErrUnavailable
		}
		field, e := ev.DecodeField(blob.Body)
		if e != nil || field.Definition.ID != ref.FieldID {
			return empty, workflowBasisRevision{}, ErrUnavailable
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(field.Value))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			return empty, workflowBasisRevision{}, ErrUnavailable
		}
		preview.Fields = append(preview.Fields, field.Definition)
		preview.Record.Values[ref.FieldID] = value
		displays := make(map[string]ReferenceDisplay)
		if field.Reference != nil {
			r := field.Reference
			displays[r.ID] = ReferenceDisplay{ID: r.ID, Label: r.Label, Deleted: r.Deleted}
		}
		for _, r := range field.OptionDisplays {
			displays[r.ID] = ReferenceDisplay{ID: r.ID, Label: r.Label, Deleted: r.Deleted}
		}
		if len(displays) != 0 {
			preview.Record.ReferenceDisplays[ref.FieldID] = displays
		}
	}
	if len(preview.Fields) != len(visible) {
		return empty, workflowBasisRevision{}, ErrUnavailable
	}
	return preview, workflowBasisRevision{SchemaVersion: h.SchemaVersion, RecordVersion: h.RecordVersion}, nil
}
