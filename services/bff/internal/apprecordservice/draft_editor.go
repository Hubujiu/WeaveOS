package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"sort"
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
	var empty DraftEditorContext
	if !appfields.ValidID(draftID) {
		return empty, appdrafts.ErrInvalid
	}
	strategy, tx, facts, err := s.draftRead(ctx, p, appID, viewID)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(context.Background())
	target, _, _, err := draftBinding(ctx, tx, facts, draftID)
	if err != nil {
		return empty, err
	}
	base, owner, err := currentDraftBase(ctx, tx, facts.TableID, target)
	if err != nil {
		return empty, err
	}
	access := draftAccess(facts, strategy.policy, strategy.definitions, base, owner)
	draft, err := draftStore().GetInTx(ctx, tx, access, draftID)
	if err != nil {
		return empty, err
	}
	fields := make([]string, 0, len(strategy.definitions))
	for _, field := range strategy.definitions {
		if access.Field(field.ID).Writable {
			fields = append(fields, field.ID)
		}
	}
	sort.Strings(fields)
	result := DraftEditorContext{Draft: draft, AppID: appID, SchemaVersion: strategy.control.Schema,
		ViewVersion: strategy.control.View, PolicyRevision: strategy.control.Policy,
		RecordVersion: base, EditableFieldIDs: fields, CanCommit: access.ResourceAllowed && !draft.HasConflicts}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}
