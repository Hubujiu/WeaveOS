package appstructure

import (
	"context"
	"testing"
	"time"
)

func TestRootDeletionProcessLoopRecoversAndWaitsForCancellation(t *testing.T) {
	p := deletionFixture(t)
	acceptDeletion(t, p, deletionInput(t, p))
	peer := &deletionPeer{}
	a := deletionWorker(p, peer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.RunDeletions(ctx) }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		state, _ := deletionState(t, p)
		if state == "deleted" {
			break
		}
		select {
		case err := <-done:
			t.Fatal("required recovery loop exited before cleanup", err)
		case <-deadline.C:
			t.Fatal("required recovery loop did not complete durable intent")
		case <-time.After(10 * time.Millisecond):
		}
	}
	select {
	case err := <-done:
		t.Fatal("idle process worker exited", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal("worker lost cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if len(peer.deletes) != 1 {
		t.Fatal("process worker duplicated original deletion")
	}
}
