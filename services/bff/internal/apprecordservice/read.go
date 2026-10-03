package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func (s *Service) GetDraft(context.Context, session.Principal, string, string, string) (appdrafts.Draft, error) {
	return appdrafts.Draft{}, ErrUnavailable
}
func (s *Service) ListDrafts(context.Context, session.Principal, DraftListRequest) (appdrafts.Page, error) {
	return appdrafts.Page{}, ErrUnavailable
}
func (s *Service) GetRecord(context.Context, session.Principal, string, string, string) (Record, error) {
	return Record{}, ErrUnavailable
}
