package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/redis/go-redis/v9"
)

type WorkflowLifecycleRequest struct{ AppID, ViewID, RecordID, InstanceID string }
type WorkflowLifecycleInfo struct {
	ID       string `json:"id"`
	Sequence int64  `json:"sequence"`
	State    string `json:"state"`
}
type WorkflowReturnTarget struct {
	NodeID          string `json:"nodeId"`
	TaskID          string `json:"taskId"`
	ActivationEpoch int64  `json:"activationEpoch"`
}
type WorkflowLifecyclePreview struct {
	BasisToken    string                 `json:"basisToken"`
	Instance      WorkflowLifecycleInfo  `json:"instance"`
	Record        Record                 `json:"record"`
	Fields        []appfields.Field      `json:"fields"`
	CanWithdraw   bool                   `json:"canWithdraw"`
	ReturnTargets []WorkflowReturnTarget `json:"returnTargets"`
}

func newWorkflowLifecycleStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return nil
}
func (s *Service) PreviewWorkflowLifecycle(ctx context.Context, p session.Principal, req WorkflowLifecycleRequest) (WorkflowLifecyclePreview, error) {
	return WorkflowLifecyclePreview{}, ErrUnavailable
}
