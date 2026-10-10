package apptemplates

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotExportable = errors.New("application configuration is not exportable")

type Service struct{ Pool *pgxpool.Pool }

func (s *Service) ExportManifest(context.Context, session.Principal, string) (Manifest, error) {
	return Manifest{}, session.ErrUnavailable
}
