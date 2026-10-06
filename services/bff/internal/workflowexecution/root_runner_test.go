package workflowexecution

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"testing"
	"time"
)

func TestRootRunRejectsInvalidWorker(t *testing.T) {
	for _, w := range []*Worker{nil, {}} {
		if err := w.Run(context.Background()); !errors.Is(err, flowcommands.ErrInvalid) {
			t.Fatalf("invalid worker must fail, got %v", err)
		}
	}
}
func TestRootRunRejectsNilContext(t *testing.T) {
	if err := (&Worker{}).Run(nil); !errors.Is(err, flowcommands.ErrInvalid) {
		t.Fatalf("nil context must fail, got %v", err)
	}
}
func TestRootLoopCancelledBeforeFirstClaim(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := runDispatchLoop(ctx, func(context.Context) (bool, error) { calls++; return false, nil }, func(context.Context, time.Duration) error { return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestRootLoopDrainsWorkWithoutIdleSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls, waits := 0, 0
	err := runDispatchLoop(ctx, func(got context.Context) (bool, error) {
		if got != ctx {
			t.Fatal("wrong context")
		}
		calls++
		if calls == 3 {
			cancel()
		}
		return true, nil
	}, func(context.Context, time.Duration) error { waits++; cancel(); return context.Canceled })
	if !errors.Is(err, context.Canceled) || calls != 3 || waits != 0 {
		t.Fatalf("err=%v calls=%d waits=%d", err, calls, waits)
	}
}
func TestRootLoopIdleUsesBoundedCancellableWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	err := runDispatchLoop(ctx, func(context.Context) (bool, error) { calls++; return false, nil }, func(got context.Context, d time.Duration) error {
		if got != ctx || d != 250*time.Millisecond {
			t.Fatalf("wait %v", d)
		}
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestRootLoopErrorBacksOffEvenWhenWorkClaimed(t *testing.T) {
	for _, worked := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		calls, waits := 0, 0
		err := runDispatchLoop(ctx, func(context.Context) (bool, error) {
			calls++
			if calls == 1 {
				return worked, errors.New("dependency unavailable")
			}
			cancel()
			return true, nil
		}, func(got context.Context, d time.Duration) error {
			waits++
			if got != ctx || d != time.Second {
				t.Fatalf("wrong error wait %v", d)
			}
			return nil
		})
		cancel()
		if !errors.Is(err, context.Canceled) || calls != 2 || waits != 1 {
			t.Fatalf("err=%v calls=%d waits=%d", err, calls, waits)
		}
	}
}
func TestRootLoopCancellationAfterDispatchSkipsWaitAndNewClaim(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls, waits := 0, 0
	err := runDispatchLoop(ctx, func(context.Context) (bool, error) {
		calls++
		cancel()
		return false, errors.New("cancelled dependency")
	}, func(context.Context, time.Duration) error { waits++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 1 || waits != 0 {
		t.Fatalf("err=%v calls=%d waits=%d", err, calls, waits)
	}
}
func TestRootLoopRejectsIncompleteSeams(t *testing.T) {
	noop := func(context.Context) (bool, error) { return false, nil }
	wait := func(context.Context, time.Duration) error { return nil }
	for _, tc := range []struct {
		ctx      context.Context
		dispatch func(context.Context) (bool, error)
		wait     func(context.Context, time.Duration) error
	}{{nil, noop, wait}, {context.Background(), nil, wait}, {context.Background(), noop, nil}} {
		if err := runDispatchLoop(tc.ctx, tc.dispatch, tc.wait); !errors.Is(err, flowcommands.ErrInvalid) {
			t.Fatalf("invalid seam accepted: %v", err)
		}
	}
}
