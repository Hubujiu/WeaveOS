package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowManualOptionsRequest struct {
	AppID, ViewID, RecordID, QueryVersion string
	Page, PageSize                        int
}
type WorkflowManualOption struct {
	FlowID            string `json:"flowId"`
	Name              string `json:"name"`
	WorkflowRevision  int64  `json:"workflowRevision"`
	DefinitionVersion int64  `json:"definitionVersion"`
	SchemaVersion     int64  `json:"schemaVersion"`
	RecordVersion     int64  `json:"recordVersion"`
}
type WorkflowManualOptionsResult struct {
	Items        []WorkflowManualOption `json:"items"`
	Total        int64                  `json:"total"`
	Page         int                    `json:"page"`
	PageSize     int                    `json:"pageSize"`
	QueryVersion string                 `json:"queryVersion"`
}

// Declaration-only placeholder for the independently specified read acceptance.
func (s *Service) SearchManualWorkflowOptions(context.Context, session.Principal, WorkflowManualOptionsRequest) (WorkflowManualOptionsResult, error) {
	return WorkflowManualOptionsResult{}, ErrUnavailable
}
