package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

var errBFFServeConfiguration = errors.New("BFF HTTP configuration invalid")
var errBFFServeFailure = errors.New("BFF HTTP server failed")
var errBFFShutdownFailure = errors.New("BFF HTTP shutdown failed")

// serveBFF owns the listener and host after validating the complete input.
// The server keeps its configured handler and connection timeouts unchanged.
func serveBFF(ctx context.Context, host *bffHost, server *http.Server, listener net.Listener, shutdownTimeout time.Duration) error {
	if ctx == nil || host == nil || server == nil || listener == nil || shutdownTimeout < time.Millisecond || shutdownTimeout > 30*time.Second {
		return errBFFServeConfiguration
	}

	served := make(chan error, 1)
	started := ctx.Err() == nil
	serveReturned := false
	failed := false
	cleanupFailed := false
	if started {
		go func() {
			// Serve closes the listener on every return path.
			served <- server.Serve(listener)
		}()
		select {
		case <-ctx.Done():
		case <-host.WorkersDone():
			failed = ctx.Err() == nil || host.Err() != nil
		case <-served:
			serveReturned = true
			// A return before this owner requests shutdown is unexpected,
			// including an externally closed server or listener.
			failed = true
		}
	}

	// Stop new claims before draining requests, but leave their RPC and
	// storage dependencies alive until the HTTP grace period has ended.
	host.Quiesce()
	if !started {
		// A pre-canceled process must never enter Accept.
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			cleanupFailed = true
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	if err := server.Shutdown(shutdown); err != nil {
		cleanupFailed = true
		_ = server.Close()
	}
	cancel()
	if started && !serveReturned {
		if err := <-served; !errors.Is(err, http.ErrServerClosed) {
			failed = true
		}
	}
	if host.Close() != nil {
		cleanupFailed = true
	}
	if cleanupFailed {
		return errBFFShutdownFailure
	}
	if failed || host.Err() != nil {
		return errBFFServeFailure
	}
	return nil
}
