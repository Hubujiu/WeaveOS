package appschema

import "context"

// Save owns exactly one transaction. Dependency and metadata adapters are
// injected; no flow/application tables or permission rules are guessed here.
func (e Executor) Save(ctx context.Context, request Request) (Result, error) {
	return Result{}, ErrNotImplemented
}
