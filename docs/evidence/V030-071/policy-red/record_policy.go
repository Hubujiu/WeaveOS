package applications

import "github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"

// RecordPolicy consumes only server-loaded RecordContext facts.
func RecordPolicy(facts RecordContext) (appaccess.Policy, bool) { return appaccess.Policy{}, false }
