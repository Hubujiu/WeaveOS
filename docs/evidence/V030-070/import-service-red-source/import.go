package apptemplates

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Import is a no-behavior declaration for independent RED before atomic writes.
func (s *Service) Import(ctx context.Context, p session.Principal, operationID string, in Manifest, bindings []Binding, metadata applications.Metadata) (applications.Result, error) {
	return applications.Result{}, session.ErrUnavailable
}
