package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/test/bufconn"
)

type rootServeFixture struct {
	host           *bffHost
	server         *http.Server
	listener       *bufconn.Listener
	client         *http.Client
	resourceCloses atomic.Int32
	closed         chan struct{}
}

func rootServeSetup(t *testing.T, handler http.Handler, first func(context.Context) error) *rootServeFixture {
	t.Helper()
	run := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	if first == nil {
		first = run
	}
	g := rootWorkerGroup(t, context.Background(), first, run)
	f := &rootServeFixture{listener: bufconn.Listen(1 << 16), closed: make(chan struct{})}
	f.host = &bffHost{Handler: handler, processCtx: context.Background(), workers: g, closeResources: func() error { f.resourceCloses.Add(1); close(f.closed); return nil }}
	f.server = &http.Server{Handler: handler}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) { return f.listener.DialContext(ctx) }}
	f.client = &http.Client{Transport: transport, Timeout: 2 * time.Second}
	t.Cleanup(func() {
		_ = f.server.Close()
		_ = f.listener.Close()
		transport.CloseIdleConnections()
		_ = f.host.Close()
	})
	return f
}
func (f *rootServeFixture) serve(ctx context.Context, grace time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() { done <- serveBFF(ctx, f.host, f.server, f.listener, grace) }()
	return done
}
func rootServeWait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case e := <-done:
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP lifecycle failed to exit within bound")
		return nil
	}
}
func rootServeStarted(t *testing.T, entered <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-entered:
	case e := <-done:
		t.Fatalf("server exited before actual request: %v", e)
	case <-time.After(2 * time.Second):
		t.Fatal("actual in-memory HTTP request never reached handler")
	}
}
func (f *rootServeFixture) request() <-chan error {
	done := make(chan error, 1)
	go func() {
		r, e := f.client.Get("http://runtime.test/work")
		if e == nil {
			_, e = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			if r.StatusCode != 200 {
				e = errors.New("unexpected HTTP status")
			}
		}
		done <- e
	}()
	return done
}
func TestRootServeStopsWorkersBeforeDrainingAndKeepsStorageUntilRequestsFinish(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	f := rootServeSetup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			_, _ = w.Write([]byte("processed"))
		case <-r.Context().Done():
		}
	}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := f.serve(ctx, time.Second)
	request := f.request()
	rootServeStarted(t, entered, done)
	cancel()
	rootWorkerSignal(t, f.host.WorkersDone())
	select {
	case <-f.closed:
		t.Fatal("storage closed while accepted HTTP request still running")
	default:
	}
	unblock()
	if e := rootServeWait(t, request); e != nil {
		t.Fatal("graceful request lost", e)
	}
	if e := rootServeWait(t, done); e != nil {
		t.Fatal("normal graceful stop failed", e)
	}
	if f.resourceCloses.Load() != 1 {
		t.Fatal("resources were not closed exactly once")
	}
}
func TestRootServeForcesTimedOutHttpDrainAndReportsFailure(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	f := rootServeSetup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(canceled) }), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := f.serve(ctx, 40*time.Millisecond)
	_ = f.request()
	rootServeStarted(t, entered, done)
	cancel()
	if e := rootServeWait(t, done); e == nil {
		t.Fatal("drain timeout silently succeeded")
	}
	rootWorkerSignal(t, canceled)
	rootWorkerSignal(t, f.host.WorkersDone())
	if f.resourceCloses.Load() != 1 {
		t.Fatal("forced shutdown did not release resources")
	}
}
func TestRootServeUnexpectedWorkerFailureStopsHttpWithoutExposingCause(t *testing.T) {
	trigger, entered := make(chan struct{}), make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(trigger) }) }
	defer finish()
	f := rootServeSetup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); _, _ = w.Write([]byte("ready")) }), func(context.Context) error { <-trigger; return errors.New("private_worker_sql") })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := f.serve(ctx, time.Second)
	request := f.request()
	rootServeStarted(t, entered, done)
	if e := rootServeWait(t, request); e != nil {
		t.Fatal(e)
	}
	finish()
	e := rootServeWait(t, done)
	if e == nil || strings.Contains(e.Error(), "private_worker_sql") || errors.Unwrap(e) != nil {
		t.Fatal("worker failure ignored or leaked", e)
	}
	if f.resourceCloses.Load() != 1 {
		t.Fatal("worker failure left owned resources open")
	}
}

type rootFailListener struct{ accepts, closes atomic.Int32 }

func (l *rootFailListener) Accept() (net.Conn, error) {
	l.accepts.Add(1)
	return nil, errors.New("private_listener_error")
}
func (l *rootFailListener) Close() error { l.closes.Add(1); return nil }
func (l *rootFailListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}
func TestRootServeListenerFailureStillStopsWorkersAndClosesResources(t *testing.T) {
	f := rootServeSetup(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil)
	listener := &rootFailListener{}
	e := serveBFF(context.Background(), f.host, f.server, listener, time.Second)
	if e == nil || strings.Contains(e.Error(), "private_listener_error") || errors.Unwrap(e) != nil {
		t.Fatal("listener failure ignored or exposed", e)
	}
	rootWorkerSignal(t, f.host.WorkersDone())
	if f.resourceCloses.Load() != 1 || listener.closes.Load() == 0 {
		t.Fatal("listen failure bypassed cleanup")
	}
}
func TestRootServeCanceledBeforeStartNeverAcceptsButCleansHost(t *testing.T) {
	f := rootServeSetup(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil)
	listener := &rootFailListener{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = serveBFF(ctx, f.host, f.server, listener, time.Second)
	if listener.accepts.Load() != 0 || listener.closes.Load() == 0 || f.resourceCloses.Load() != 1 {
		t.Fatal("pre-canceled startup served requests or leaked ownership")
	}
	rootWorkerSignal(t, f.host.WorkersDone())
}
func TestRootServeRejectsMissingInputsAndUnboundedGrace(t *testing.T) {
	f := rootServeSetup(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil)
	for _, c := range []struct {
		ctx      context.Context
		host     *bffHost
		server   *http.Server
		listener net.Listener
		grace    time.Duration
	}{{nil, f.host, f.server, f.listener, time.Second}, {context.Background(), nil, f.server, f.listener, time.Second}, {context.Background(), f.host, nil, f.listener, time.Second}, {context.Background(), f.host, f.server, nil, time.Second}, {context.Background(), f.host, f.server, f.listener, 0}, {context.Background(), f.host, f.server, f.listener, 31 * time.Second}} {
		if e := serveBFF(c.ctx, c.host, c.server, c.listener, c.grace); e == nil {
			t.Error("invalid lifecycle accepted")
		}
	}
}
