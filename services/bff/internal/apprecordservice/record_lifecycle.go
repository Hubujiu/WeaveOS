package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type RecordLifecycleRequest struct {
	AppID, ViewID, RecordID, OperationID         string
	ExpectedSchemaVersion, ExpectedRecordVersion int64
	Deleted                                      bool
}
type RecordLifecycleResult struct {
	OperationID   string `json:"operationId"`
	ID            string `json:"id"`
	RecordVersion int64  `json:"recordVersion"`
	SchemaVersion int64  `json:"schemaVersion"`
	Deleted       bool   `json:"deleted"`
}

// Declaration-only placeholder for the first real behavior RED.
func (s *Service) ChangeRecordLifecycle(context.Context, session.Principal, RecordLifecycleRequest) (RecordLifecycleResult, error) {
	return RecordLifecycleResult{}, ErrUnavailable
}
