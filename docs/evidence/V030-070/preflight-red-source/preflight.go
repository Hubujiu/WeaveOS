package apptemplates

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

type Counts struct {
	Directories      int `json:"directories"`
	Tables           int `json:"tables"`
	Fields           int `json:"fields"`
	Forms            int `json:"forms"`
	Workflows        int `json:"workflows"`
	PermissionGroups int `json:"permissionGroups"`
}
type Preflight struct {
	Valid  bool   `json:"valid"`
	Counts Counts `json:"counts"`
}

func (s *Service) Preflight(context.Context, session.Principal, Manifest, []Binding) (Preflight, error) {
	return Preflight{}, ErrInvalid
}
