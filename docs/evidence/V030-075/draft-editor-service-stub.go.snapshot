package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// DraftEditorContext is a current, server-authorized edit mask, not record read authority.
type DraftEditorContext struct {
	Draft            appdrafts.Draft `json:"draft"`
	AppID            string          `json:"appId"`
	SchemaVersion    int64           `json:"schemaVersion"`
	ViewVersion      int64           `json:"viewVersion"`
	PolicyRevision   int64           `json:"policyRevision"`
	RecordVersion    *int64          `json:"recordVersion"`
	EditableFieldIDs []string        `json:"editableFieldIds"`
	CanCommit        bool            `json:"canCommit"`
}

func (s *Service) GetDraftEditorContext(ctx context.Context, p session.Principal, appID, viewID, draftID string) (DraftEditorContext, error) {
	return DraftEditorContext{}, ErrUnavailable
}
