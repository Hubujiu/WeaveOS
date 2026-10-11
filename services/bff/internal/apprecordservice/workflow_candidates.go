package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

type WorkflowCandidateRequest struct {
	WorkflowTaskRequest
	FieldID, Query, PageToken string
	PageSize                  int
}
type WorkflowCandidateResult struct {
	Items      []any                   `json:"items"`
	Pagination appstructure.Pagination `json:"-"`
}

// Authorization and source enumeration share a single current RR snapshot.
// Unlike preview this operation needs no field values or approval basis token.
func (s *Service) WorkflowTaskCandidates(ctx context.Context, principal session.Principal, req WorkflowCandidateRequest) (WorkflowCandidateResult, error) {
	empty := WorkflowCandidateResult{}
	if s == nil || s.Pool == nil || s.WorkflowCandidates == nil {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) || principal.SessionRef == "" {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{req.AppID, req.ViewID, req.RecordID, req.InstanceID, req.TaskID, req.FieldID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.PageSize < 1 || req.PageSize > 50 || !utf8.ValidString(req.Query) || utf8.RuneCountInString(req.Query) > 100 || strings.ContainsRune(req.Query, 0) {
		return empty, applications.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
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
	if !facts.SchemaReady {
		return empty, &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
	}
	var creator string
	var recordVersion int64
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
	err = tx.QueryRow(ctx, "SELECT created_by::text,record_version FROM "+relation+" WHERE id=$1", req.RecordID).Scan(&creator, &recordVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	if policy.VisibleScope() == appaccess.Own && creator != facts.Actor.ID {
		return empty, applications.ErrMissing
	}
	if len(policy.ReadFields(creator, []string{req.FieldID})) != 1 || !policy.CanEdit(creator, []string{req.FieldID}) {
		return empty, applications.ErrDenied
	}
	fields, _, err := fieldsInContext(facts)
	if err != nil {
		return empty, err
	}
	kind := ""
	for _, field := range fields {
		if field.ID == req.FieldID {
			kind = field.Kind
			break
		}
	}
	if kind != "member" && kind != "department" {
		return empty, applications.ErrResourceInvalid
	}
	binding := workflowBasisBinding{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, RecordID: req.RecordID, InstanceID: req.InstanceID, TaskID: req.TaskID}
	var assignee, state string
	var closed bool
	var graph []byte
	err = tx.QueryRow(ctx, `SELECT t.node_id::text,t.assignee_id::text,t.activation_epoch,
 t.closed_command_id IS NOT NULL,i.state,i.sequence,i.definition_version,v.version_id::text,v.graph_json
 FROM applications.workflow_instances i
 JOIN applications.workflow_tasks t ON t.app_id=i.app_id AND t.instance_id=i.id
 JOIN applications.workflow_versions v ON v.app_id=i.app_id AND v.flow_id=i.flow_id AND v.version=i.definition_version
 WHERE i.app_id=$1 AND i.table_id=$2 AND i.view_id=$3 AND i.record_id=$4 AND i.id=$5 AND t.id=$6`, binding.AppID, binding.TableID, binding.ViewID, binding.RecordID, binding.InstanceID, binding.TaskID).Scan(&binding.NodeID, &assignee, &binding.ActivationEpoch, &closed, &state, &binding.Sequence, &binding.DefinitionVersion, &binding.VersionID, &graph)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	if assignee != facts.Actor.ID {
		return empty, applications.ErrDenied
	}
	if !workflowID(binding.NodeID) || !workflowID(binding.VersionID) || !workflowPositive(binding.ActivationEpoch) || !workflowPositive(binding.DefinitionVersion) || binding.Sequence < 0 || binding.Sequence > workflowMaxCounter || !workflowPositive(recordVersion) || !workflowPositive(facts.SchemaVersion) {
		return empty, ErrUnavailable
	}
	editable, valid := workflowApprovalEditableFields(graph, binding.NodeID, assignee)
	if !valid {
		return empty, ErrUnavailable
	}
	if !editable[req.FieldID] {
		return empty, applications.ErrDenied
	}
	if closed || state != "active" {
		return empty, ErrWorkflowTaskChanged
	}
	sessionHash := sha256.Sum256([]byte(principal.SessionRef))
	raw, err := json.Marshal(struct {
		Task           workflowBasisBinding
		Revision       workflowBasisRevision
		SessionHash    string
		PolicyRevision int64
	}{binding, workflowBasisRevision{SchemaVersion: facts.SchemaVersion, RecordVersion: recordVersion}, hex.EncodeToString(sessionHash[:]), facts.App.PolicyRevision})
	if err != nil {
		return empty, ErrUnavailable
	}
	digest := sha256.Sum256(raw)
	scope := appstructure.TrustedTaskCandidateScope{ActorID: facts.Actor.ID, AppID: req.AppID, ViewID: req.ViewID, RecordID: req.RecordID, FieldID: req.FieldID, Kind: kind, Binding: hex.EncodeToString(digest[:]), PolicyRevision: facts.App.PolicyRevision}
	items, page, err := s.WorkflowCandidates.TaskReferenceCandidatesInTx(ctx, tx, scope, req.Query, req.PageSize, req.PageToken)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return WorkflowCandidateResult{Items: items, Pagination: page}, nil
}
