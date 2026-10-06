package main

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
)

var errBFFHostUnavailable = errors.New("BFF host unavailable")
var errBFFHostCleanup = errors.New("BFF host cleanup failed")

// bffHost owns the existing HTTP composition and its process-scoped resources.
// Dependencies are fixed before publication; only stopping and close state vary.
type bffHost struct {
	Handler http.Handler

	processCtx     context.Context
	authReady      httpserver.ReadyCheck
	rpc            *workflowRPC
	workers        *workflowWorkers
	closeResources func() error

	mu        sync.Mutex
	stopping  bool
	closeOnce sync.Once
	closeErr  error
}

func (host *bffHost) running() bool {
	if host == nil {
		return false
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	return !host.stopping && host.processCtx != nil && host.processCtx.Err() == nil
}

func (host *bffHost) ready(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil || !host.running() || host.authReady == nil {
		return errBFFHostUnavailable
	}
	if host.workers != nil && !host.workers.Ready() {
		return errBFFHostUnavailable
	}
	if err := host.authReady(ctx); err != nil {
		return errBFFHostUnavailable
	}
	if host.rpc != nil {
		if err := host.rpc.Ready(ctx); err != nil {
			return errBFFHostUnavailable
		}
	}
	// Closing or worker failure during dependency I/O must not yield a stale
	// healthy result. No lifecycle lock is held across the network checks.
	if ctx.Err() != nil || !host.running() || host.workers != nil && !host.workers.Ready() {
		return errBFFHostUnavailable
	}
	return nil
}

func (host *bffHost) Quiesce() {
	if host == nil {
		return
	}
	host.mu.Lock()
	host.stopping = true
	workers := host.workers
	host.mu.Unlock()
	if workers != nil {
		workers.Stop()
	}
}

func (host *bffHost) Close() error {
	if host == nil {
		return nil
	}
	host.closeOnce.Do(func() {
		host.Quiesce()
		if host.workers != nil {
			wait, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := host.workers.Wait(wait); err != nil {
				host.closeErr = errBFFHostCleanup
			}
			cancel()
		}
		if host.rpc != nil {
			if err := host.rpc.Close(); err != nil {
				host.closeErr = errBFFHostCleanup
			}
		}
		if host.closeResources != nil {
			if err := host.closeResources(); err != nil {
				host.closeErr = errBFFHostCleanup
			}
		}
	})
	return host.closeErr
}

func (host *bffHost) WorkersDone() <-chan struct{} {
	if host == nil || host.workers == nil {
		return nil
	}
	return host.workers.Done()
}

func (host *bffHost) Err() error {
	if host == nil || host.workers == nil {
		return nil
	}
	return host.workers.Err()
}

func (host *bffHost) failStartup(err error) (*bffHost, error) {
	if host.Close() != nil {
		return nil, errors.Join(err, errBFFHostCleanup)
	}
	return nil, err
}
