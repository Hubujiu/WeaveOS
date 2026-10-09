package apprecordservice

import "context"

// Compile-only declarations for the test-first start admission contract.
func (s *Service) NewStartAdmitter() func(context.Context) (bool, error) {
	return func(context.Context) (bool, error) { return false, nil }
}
func (s *Service) acceptWorkflowStart(context.Context, string) (bool, error) { return false, nil }
