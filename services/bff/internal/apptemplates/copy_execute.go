package apptemplates

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func (s *Service) ExecuteCopy(ctx context.Context, p session.Principal, operationID string, in CopyPreflightInput, metadata applications.Metadata) (applications.Result, error) {
	return applications.Result{}, ErrInvalid
}
