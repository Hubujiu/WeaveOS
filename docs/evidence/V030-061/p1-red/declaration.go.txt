package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowInboxRequest struct {
	Page, PageSize int
	QueryVersion   string
}
type WorkflowInboxItem struct {
	ID                string `json:"id"`
	AppID             string `json:"appId"`
	ViewID            string `json:"viewId"`
	RecordID          string `json:"recordId"`
	InstanceID        string `json:"instanceId"`
	FlowID            string `json:"flowId"`
	NodeID            string `json:"nodeId"`
	FlowName          string `json:"flowName"`
	CreatedAt         string `json:"createdAt"`
	DefinitionVersion int64  `json:"definitionVersion"`
	ActivationEpoch   int64  `json:"activationEpoch"`
	Sequence          int64  `json:"sequence"`
}
type WorkflowInboxResult struct {
	Items        []WorkflowInboxItem `json:"items"`
	Total        int64               `json:"total"`
	Page         int                 `json:"page"`
	PageSize     int                 `json:"pageSize"`
	QueryVersion string              `json:"queryVersion"`
}

// Declaration-only stub for the independent RED stage.
func (s *Service) SearchWorkflowInbox(ctx context.Context, p session.Principal, q WorkflowInboxRequest) (WorkflowInboxResult, error) {
	return WorkflowInboxResult{}, ErrUnavailable
}
