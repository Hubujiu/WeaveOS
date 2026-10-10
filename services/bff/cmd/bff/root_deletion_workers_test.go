package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRootDeletionWorkersOwnThirdLifetime(t *testing.T) {
	var calls atomic.Int32
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	run := func(ctx context.Context) error { calls.Add(1); <-ctx.Done(); return ctx.Err() }
	g, e := startWorkflowWorkers(context.Background(), run, run, func(ctx context.Context) error {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	})
	if e != nil {
		t.Fatal(e)
	}
	defer func() { close(release); g.Stop(); _ = rootWorkerWait(t, g) }()
	rootWorkerSignal(t, started)
	if !g.Ready() {
		t.Fatal("third loop lost readiness")
	}
	g.Stop()
	rootWorkerSignal(t, canceled)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if !errors.Is(g.Wait(ctx), context.DeadlineExceeded) {
		t.Fatal("storage may close before deletion cleanup")
	}
	if calls.Load() != 3 {
		t.Fatal("required loop duplicated or absent", calls.Load())
	}
}
func TestRootDeletionWorkersUnexpectedExitCancelsBoth(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	run := func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() }
	g, e := startWorkflowWorkers(context.Background(), run, run, func(context.Context) error { <-release; return errors.New("private deletion receipt") })
	if e != nil {
		t.Fatal(e)
	}
	defer g.Stop()
	rootWorkerSignal(t, started)
	rootWorkerSignal(t, started)
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := g.Wait(ctx)
	if err == nil || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "private") || g.Ready() {
		t.Fatal("deletion failure not owned/redacted", err)
	}
}
func TestRootDeletionWorkersRejectNilThirdBeforeLaunching(t *testing.T) {
	var calls atomic.Int32
	run := func(ctx context.Context) error { calls.Add(1); <-ctx.Done(); return ctx.Err() }
	g, e := startWorkflowWorkers(context.Background(), run, run, nil)
	if g != nil {
		g.Stop()
		_ = rootWorkerWait(t, g)
	}
	if e == nil || g != nil || calls.Load() != 0 {
		t.Fatal("nil required deletion worker launched partial host")
	}
}
