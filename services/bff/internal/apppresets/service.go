package apppresets

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("application preset version conflict")
var ErrNameConflict = errors.New("application preset name conflict")
var ErrLimit = errors.New("application preset limit reached")

type Service struct {
	Pool   *pgxpool.Pool
	Limits appschema.Limits
}
type Request struct {
	AppID, ViewID, ID, OperationID         string
	ExpectedSchemaVersion, ExpectedVersion int64
	State                                  State
}

// RED-only declarations; the service has no behavior yet.
func (s *Service) Create(ctx context.Context, p session.Principal, r Request) (applications.Result, error) {
	return applications.Result{}, session.ErrUnavailable
}
func (s *Service) Update(ctx context.Context, p session.Principal, r Request) (applications.Result, error) {
	return applications.Result{}, session.ErrUnavailable
}
func (s *Service) Delete(ctx context.Context, p session.Principal, r Request) (applications.Result, error) {
	return applications.Result{}, session.ErrUnavailable
}
func (s *Service) Get(ctx context.Context, p session.Principal, app, view, id string) (json.RawMessage, error) {
	return nil, session.ErrUnavailable
}
func (s *Service) List(ctx context.Context, p session.Principal, app, view string) ([]json.RawMessage, error) {
	return nil, session.ErrUnavailable
}
