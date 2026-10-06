package flowgraph

import "github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"

// CompileScopedBPMN retains one application-isolated process identity across
// publications so Flowable can allocate native versions under the same key.
func CompileScopedBPMN(g Graph, fields []appquery.Field, appID, flowID string) ([]byte, error) {
	if !validUUID(appID) || !validUUID(flowID) {
		return nil, ErrInvalid
	}
	return compileBPMN(g, fields, "p_"+uuidCompact(appID)+"_"+uuidCompact(flowID))
}
