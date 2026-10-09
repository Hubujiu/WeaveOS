package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowManualStartRequest struct {
	AppID, ViewID, RecordID, FlowID, OperationID                           string
	ExpectedWorkflowRevision, ExpectedSchemaVersion, ExpectedRecordVersion int64
}
type WorkflowManualStartResult struct {
	OperationID string `json:"operationId"`
	FlowID      string `json:"flowId"`
	InstanceID  string `json:"instanceId"`
	Status      string `json:"status"`
	Ignored     bool   `json:"ignored"`
}

// Declaration-only TDD placeholder; no admission behavior is implemented.
func (s *Service) StartManualWorkflow(context.Context, session.Principal, WorkflowManualStartRequest, applications.Metadata) (WorkflowManualStartResult, error) {
	return WorkflowManualStartResult{}, ErrUnavailable
}
