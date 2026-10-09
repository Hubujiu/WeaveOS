package workflowexecution

import (
	"context"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
)

// Run dispatches accepted work serially until its lifetime context is canceled.
func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.Pool == nil || nilPort(w.Client) || nilPort(ctx) ||
		w.RPCTimeout <= 0 || w.RPCTimeout > 30*time.Second ||
		w.Limits.LockTimeout < time.Millisecond || w.Limits.StatementTimeout < time.Millisecond {
		return flowcommands.ErrInvalid
	}
	return runDispatchLoop(ctx, w.DispatchOne, func(ctx context.Context, delay time.Duration) error {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	})
}

func runDispatchLoop(ctx context.Context, dispatch func(context.Context) (bool, error), wait func(context.Context, time.Duration) error) error {
	if nilPort(ctx) || dispatch == nil || wait == nil {
		return flowcommands.ErrInvalid
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		worked, err := dispatch(ctx)
		if canceled := ctx.Err(); canceled != nil {
			return canceled
		}
		if worked && err == nil {
			continue
		}
		delay := 250 * time.Millisecond
		if err != nil {
			delay = time.Second
		}
		if err := wait(ctx, delay); err != nil {
			if canceled := ctx.Err(); canceled != nil {
				return canceled
			}
			return err
		}
	}
}
