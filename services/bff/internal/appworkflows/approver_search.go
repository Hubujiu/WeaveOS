package appworkflows

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/redis/go-redis/v9"
)

// RED-only declaration: neither a discovery query nor a route exists yet.
func NewApproverQueryStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return nil
}
