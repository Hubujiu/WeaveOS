package apprecordservice

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type WorkflowEventRequest struct{ AppID, ViewID, RecordID, EventID string }
type WorkflowEventSummary struct {
	ID            string  `json:"id"`
	InstanceID    string  `json:"instanceId"`
	FlowID        string  `json:"flowId"`
	NodeID        *string `json:"nodeId"`
	TargetNodeID  *string `json:"targetNodeId"`
	ActorID       string  `json:"actorId"`
	Action        string  `json:"action"`
	Outcome       string  `json:"outcome"`
	Sequence      int64   `json:"sequence"`
	SchemaVersion int64   `json:"schemaVersion"`
	RecordVersion int64   `json:"recordVersion"`
	OccurredAt    string  `json:"occurredAt"`
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
	return WorkflowEventResult{}, ErrUnavailable
}
