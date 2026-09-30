package session

import (
	"context"
	"github.com/redis/go-redis/v9"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// A real blackholed TCP connection injects transport failure, not fake Redis semantics.
// Caller deadlines must bound readiness/authentication dependency checks.
func TestRedisTransportHonorsCallerDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer connection.Close(); <-done }()
		}
	}()
	store := NewStore("redis://"+listener.Addr().String()+"/15", "deadline-test")
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := store.Ping(ctx); err == nil {
		t.Fatal("blackholed dependency cannot be ready")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("caller deadline was ignored: elapsed %s exceeds1s safety margin", elapsed)
	}
}

// ADR-001/004 fail-closed dependency budget: a failed authentication transport
// must not make five independent dial attempts after command retries are disabled.
func TestSessionDialFailureDoesNotRetryAuthenticationTransport(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	store := NewStore("redis://"+address+"/15", "single-dial")
	defer store.Close()
	options := store.client.(*redis.Client).Options()
	dial := options.Dialer
	var attempts atomic.Int32
	options.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
		attempts.Add(1)
		return dial(ctx, network, addr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err == nil {
		t.Fatal("closed real TCP endpoint cannot be ready")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("an authentication operation must attempt one real transport dial, got %d", got)
	}
}
