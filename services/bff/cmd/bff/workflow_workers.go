package main

import (
	"context"
	"errors"
	"sync"
)

// workflowWorkers owns the shared lifetime of the two existing worker loops.
type workflowWorkers struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu        sync.Mutex
	remaining int
	err       error
}

func startWorkflowWorkers(processCtx context.Context, runPublications, runExecution func(context.Context) error) (*workflowWorkers, error) {
	if processCtx == nil || processCtx.Err() != nil {
		return nil, errors.New("workflow workers require an active process context")
	}
	if runPublications == nil || runExecution == nil {
		return nil, errors.New("workflow worker functions are required")
	}

	ctx, cancel := context.WithCancel(processCtx)
	w := &workflowWorkers{
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
		remaining: 2,
	}
	go w.run(runPublications)
	go w.run(runExecution)
	return w, nil
}

func (w *workflowWorkers) run(run func(context.Context) error) {
	// The returned cause may contain private data and is deliberately discarded.
	_ = run(w.ctx)
	unexpected := w.ctx.Err() == nil

	w.mu.Lock()
	defer w.mu.Unlock()
	if unexpected && w.err == nil {
		w.err = errors.New("workflow worker exited unexpectedly")
		w.cancel()
	}
	w.remaining--
	if w.remaining == 0 {
		close(w.done)
	}
}

func (w *workflowWorkers) Stop() {
	w.cancel()
}

func (w *workflowWorkers) Done() <-chan struct{} {
	return w.done
}

func (w *workflowWorkers) Ready() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ctx.Err() == nil && w.remaining == 2 && w.err == nil
}

func (w *workflowWorkers) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// Wait observes completion without changing the workers' lifetime.
func (w *workflowWorkers) Wait(ctx context.Context) error {
	if ctx == nil {
		return errors.New("workflow worker wait context is required")
	}
	select {
	case <-w.done:
		return w.Err()
	default:
	}
	select {
	case <-w.done:
		return w.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}
