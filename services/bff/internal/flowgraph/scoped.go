package flowgraph

import "github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"

// CompileScopedBPMN is the V038 declaration-only boundary for native versioning.
// The first test stage deliberately has no implementation.
func CompileScopedBPMN(g Graph, fields []appquery.Field, appID, flowID string) ([]byte, error) {
	return nil, ErrInvalid
}
