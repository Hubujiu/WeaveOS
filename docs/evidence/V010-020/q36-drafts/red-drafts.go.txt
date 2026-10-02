package personnel

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

var ErrDraftConflict = errors.New("personnel draft version conflict")
var ErrDraftLimit = errors.New("personnel draft limit reached")

// RED-stage declarations only: no behavior installed.
func (a *Application) CreateDraft(context.Context, session.Principal, DraftCreateInput) (PersonnelDraft, error) {
	return PersonnelDraft{}, ErrNotImplemented
}
func (a *Application) ListDrafts(context.Context, session.Principal) (PersonnelDraftList, error) {
	return PersonnelDraftList{}, ErrNotImplemented
}
func (a *Application) GetDraft(context.Context, session.Principal, string) (PersonnelDraft, error) {
	return PersonnelDraft{}, ErrNotImplemented
}
func (a *Application) UpdateDraft(context.Context, session.Principal, string, DraftUpdateInput) (PersonnelDraft, error) {
	return PersonnelDraft{}, ErrNotImplemented
}
func (a *Application) DeleteDraft(context.Context, session.Principal, string, int64) error {
	return ErrNotImplemented
}
func CleanupDraft(context.Context, pgx.Tx, session.Principal, *DraftReference, DraftKind, *string) (bool, error) {
	return false, ErrNotImplemented
}
