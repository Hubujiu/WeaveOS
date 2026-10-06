package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func rootWorkerSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("worker lifecycle event did not occur")
	}
}
func rootWorkerGroup(t *testing.T, ctx context.Context, a, b func(context.Context) error) *workflowWorkers {
	t.Helper()
	g, e := startWorkflowWorkers(ctx, a, b)
	if e != nil || g == nil {
		t.Fatalf("worker group unavailable: %v", e)
	}
	t.Cleanup(func() {
		g.Stop()
		wait, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = g.Wait(wait)
	})
	return g
}
func rootWorkerWait(t *testing.T, g *workflowWorkers) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return g.Wait(ctx)
}

func TestRootWorkersRejectInvalidInputsBeforeStarting(t *testing.T) {
	var calls atomic.Int32
	run := func(context.Context) error { calls.Add(1); return nil }
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, c := range []struct {
		ctx  context.Context
		a, b func(context.Context) error
	}{{nil, run, run}, {canceled, run, run}, {context.Background(), nil, run}, {context.Background(), run, nil}} {
		g, e := startWorkflowWorkers(c.ctx, c.a, c.b)
		if e == nil || g != nil {
			t.Error("invalid lifecycle accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid start launched work")
	}
}
func TestRootWorkersStartExactlyOneOfEachUntilStopped(t *testing.T) {
	var a, b atomic.Int32
	startedA, startedB := make(chan struct{}), make(chan struct{})
	g := rootWorkerGroup(t, context.Background(), func(ctx context.Context) error { a.Add(1); close(startedA); <-ctx.Done(); return ctx.Err() }, func(ctx context.Context) error { b.Add(1); close(startedB); <-ctx.Done(); return ctx.Err() })
	rootWorkerSignal(t, startedA)
	rootWorkerSignal(t, startedB)
	if !g.Ready() || g.Err() != nil {
		t.Fatal("healthy running workers marked unavailable")
	}
	select {
	case <-g.Done():
		t.Fatal("Done closed while loops running")
	default:
	}
	g.Stop()
	if e := rootWorkerWait(t, g); e != nil {
		t.Fatal(e)
	}
	if a.Load() != 1 || b.Load() != 1 || g.Ready() || g.Err() != nil {
		t.Fatal("normal shutdown duplicated work or became a failure")
	}
}
func TestRootWorkersFollowProcessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := make(chan struct{}), make(chan struct{})
	g := rootWorkerGroup(t, ctx, func(ctx context.Context) error { <-ctx.Done(); close(a); return ctx.Err() }, func(ctx context.Context) error { <-ctx.Done(); close(b); return ctx.Err() })
	cancel()
	rootWorkerSignal(t, a)
	rootWorkerSignal(t, b)
	rootWorkerSignal(t, g.Done())
	if e := rootWorkerWait(t, g); e != nil || g.Ready() {
		t.Fatalf("process cancellation did not finish normally: %v", e)
	}
}
func TestRootWorkersDoneWaitsForBothBoundedCleanupPaths(t *testing.T) {
	started, slowCanceled, fastDone, release := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseSlow := func() { once.Do(func() { close(release) }) }
	defer releaseSlow()
	g := rootWorkerGroup(t, context.Background(), func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(slowCanceled)
		<-release
		return ctx.Err()
	}, func(ctx context.Context) error { <-ctx.Done(); close(fastDone); return ctx.Err() })
	rootWorkerSignal(t, started)
	g.Stop()
	rootWorkerSignal(t, slowCanceled)
	rootWorkerSignal(t, fastDone)
	select {
	case <-g.Done():
		t.Fatal("resource owner may close storage before slow bookkeeping finishes")
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if !errors.Is(g.Wait(ctx), context.DeadlineExceeded) {
		t.Fatal("bounded wait falsely completed before both loops")
	}
	releaseSlow()
	if e := rootWorkerWait(t, g); e != nil {
		t.Fatal(e)
	}
}
func TestRootWorkersUnexpectedFailureCancelsSiblingAndRedactsCause(t *testing.T) {
	siblingStarted, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	secret := errors.New("private_sql_credentials_and_payload")
	g := rootWorkerGroup(t, context.Background(), func(context.Context) error { <-release; return secret }, func(ctx context.Context) error {
		close(siblingStarted)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})
	rootWorkerSignal(t, siblingStarted)
	close(release)
	rootWorkerSignal(t, finished)
	rootWorkerSignal(t, g.Done())
	e := g.Err()
	if e == nil || strings.Contains(e.Error(), secret.Error()) || errors.Unwrap(e) != nil || g.Ready() {
		t.Fatal("unexpected exit was healthy, silent, or leaked the cause")
	}
	if rootWorkerWait(t, g) == nil {
		t.Fatal("Wait hid unexpected worker failure")
	}
}
func TestRootWorkersUnexpectedNilReturnIsNotSuccess(t *testing.T) {
	g := rootWorkerGroup(t, context.Background(), func(context.Context) error { return nil }, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	rootWorkerSignal(t, g.Done())
	if g.Err() == nil || g.Ready() || rootWorkerWait(t, g) == nil {
		t.Fatal("unexpected nil exit silently stopped a required worker")
	}
}
func TestRootWorkersConcurrentStopAndWaitAreIdempotent(t *testing.T) {
	var calls atomic.Int32
	run := func(ctx context.Context) error { calls.Add(1); <-ctx.Done(); return ctx.Err() }
	g := rootWorkerGroup(t, context.Background(), run, run)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Stop()
			if e := rootWorkerWait(t, g); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 2 || g.Err() != nil || g.Ready() {
		t.Fatal("concurrent stop duplicated or lost a worker")
	}
}
func TestRootWorkersInvalidWaitDoesNotStopRunningWorkers(t *testing.T) {
	started := make(chan struct{})
	run := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	g := rootWorkerGroup(t, context.Background(), func(ctx context.Context) error { close(started); return run(ctx) }, run)
	rootWorkerSignal(t, started)
	if g.Wait(nil) == nil {
		t.Fatal("nil wait context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(g.Wait(ctx), context.Canceled) {
		t.Fatal("canceled wait did not return caller cancellation")
	}
	if !g.Ready() {
		t.Fatal("timed/canceled wait incorrectly stopped healthy work")
	}
}
