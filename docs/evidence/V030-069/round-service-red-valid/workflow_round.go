package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowRoundRequest struct{ AppID, ViewID, RecordID, InstanceID string }
type WorkflowRoundPreview struct {
	InstanceID        string   `json:"instanceId"`
	FlowID            string   `json:"flowId"`
	RoundNumber       int64    `json:"roundNumber"`
	LatestInstanceID  string   `json:"latestInstanceId"`
	State             string   `json:"state"`
	DefinitionVersion int64    `json:"definitionVersion"`
	WorkflowRevision  int64    `json:"workflowRevision"`
	SchemaVersion     int64    `json:"schemaVersion"`
	RecordVersion     int64    `json:"recordVersion"`
	CanRework         bool     `json:"canRework"`
	CanResubmit       bool     `json:"canResubmit"`
	CanReview         bool     `json:"canReview"`
	EditableFieldIDs  []string `json:"editableFieldIds"`
}
type WorkflowRoundStartRequest struct {
	WorkflowRoundRequest
	OperationID, Kind                                                      string
	ExpectedWorkflowRevision, ExpectedSchemaVersion, ExpectedRecordVersion int64
}
type WorkflowRoundStartResult struct {
	OperationID        string `json:"operationId"`
	FlowID             string `json:"flowId"`
	PreviousInstanceID string `json:"previousInstanceId"`
	InstanceID         string `json:"instanceId"`
	RoundKind          string `json:"roundKind"`
	RoundNumber        int64  `json:"roundNumber"`
	Status             string `json:"status"`
}
type WorkflowRoundReworkRequest struct {
	WorkflowRoundRequest
	OperationID                                  string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Changes                                      map[string]any
}

func (s *Service) PreviewWorkflowRound(context.Context, session.Principal, WorkflowRoundRequest) (WorkflowRoundPreview, error) {
	return WorkflowRoundPreview{}, ErrUnavailable
}
func (s *Service) StartWorkflowRound(context.Context, session.Principal, WorkflowRoundStartRequest, applications.Metadata) (WorkflowRoundStartResult, error) {
	return WorkflowRoundStartResult{}, ErrUnavailable
}
func (s *Service) ReworkWorkflowRound(context.Context, session.Principal, WorkflowRoundReworkRequest, applications.Metadata) (MutationResult, error) {
	return MutationResult{}, ErrUnavailable
}
