package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health"
	hp "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const rootRPCToken = "V041_RPC_TRANSPORT_SYNTHETIC_1234567890"
const rootDeploymentHealth = "weaveos.workflow.v1.DeploymentService"
const rootExecutionHealth = "weaveos.workflow.v1.ExecutionService"

type rootRPCFixture struct {
	health      *health.Server
	listener    *bufconn.Listener
	server      *grpc.Server
	conn        *grpc.ClientConn
	mu          sync.Mutex
	services    []string
	methods     []string
	blockHealth bool
}
type rootRPCDeployment struct {
	pb.UnimplementedDeploymentServiceServer
}

func (*rootRPCDeployment) Lookup(context.Context, *pb.LookupRequest) (*pb.LookupResponse, error) {
	return &pb.LookupResponse{Result: &pb.LookupResponse_NotObserved{NotObserved: &pb.NotObserved{}}}, nil
}

type rootRPCExecution struct {
	pb.UnimplementedExecutionServiceServer
}

func (*rootRPCExecution) Lookup(context.Context, *pb.ExecutionLookupRequest) (*pb.ExecutionLookupResponse, error) {
	return &pb.ExecutionLookupResponse{Result: &pb.ExecutionLookupResponse_NotObserved{NotObserved: &pb.NotObserved{}}}, nil
}

func rootRPCConfig(t *testing.T) workflowRuntimeConfig {
	t.Helper()
	i, err := workflowrpc.NewServiceIdentity(rootRPCToken)
	if err != nil {
		t.Fatal(err)
	}
	return workflowRuntimeConfig{Enabled: true, Target: "127.0.0.1:50051", RPCTimeout: time.Second, Identity: i}
}
func rootRPCServer(t *testing.T, block bool) *rootRPCFixture {
	t.Helper()
	f := &rootRPCFixture{health: health.NewServer(), listener: bufconn.Listen(2 << 20), blockHealth: block}
	f.health.SetServingStatus(rootDeploymentHealth, hp.HealthCheckResponse_SERVING)
	f.health.SetServingStatus(rootExecutionHealth, hp.HealthCheckResponse_SERVING)
	f.server = grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if !reflect.DeepEqual(md.Get("authorization"), []string{"Bearer " + rootRPCToken}) || len(md.Get("cookie")) != 0 {
			return nil, status.Error(codes.Unauthenticated, "synthetic secret must never escape")
		}
		f.mu.Lock()
		f.methods = append(f.methods, info.FullMethod)
		if h, ok := req.(*hp.HealthCheckRequest); ok {
			f.services = append(f.services, h.Service)
		}
		f.mu.Unlock()
		if _, ok := req.(*hp.HealthCheckRequest); ok && f.blockHealth {
			<-ctx.Done()
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return next(ctx, req)
	}))
	hp.RegisterHealthServer(f.server, f.health)
	pb.RegisterDeploymentServiceServer(f.server, &rootRPCDeployment{})
	pb.RegisterExecutionServiceServer(f.server, &rootRPCExecution{})
	go func() { _ = f.server.Serve(f.listener) }()
	t.Cleanup(func() {
		if f.conn != nil {
			_ = f.conn.Close()
		}
		f.server.Stop()
		_ = f.listener.Close()
	})
	return f
}
func (f *rootRPCFixture) dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return f.listener.DialContext(ctx) }))
	c, e := grpc.NewClient(target, opts...)
	f.conn = c
	return c, e
}
func rootRPCOpen(t *testing.T, f *rootRPCFixture, ctx context.Context) *workflowRPC {
	t.Helper()
	r, e := connectWorkflowRPC(ctx, rootRPCConfig(t), f.dial)
	if e != nil || r == nil {
		t.Fatalf("real RPC composition unavailable: %v", e)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}
func rootRPCClosed(t *testing.T, f *rootRPCFixture) {
	t.Helper()
	if f.conn == nil || f.conn.GetState() != connectivity.Shutdown {
		t.Fatal("partial/closed connection was not released")
	}
}

func TestRootRPCConnectAuthenticatedBothServices(t *testing.T) {
	f := rootRPCServer(t, false)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer browser", "authorization", "extra", "cookie", "browser-session"))
	r := rootRPCOpen(t, f, ctx)
	if r.Deployment == nil || r.Execution == nil {
		t.Fatal("existing typed business clients missing")
	}
	f.mu.Lock()
	services := append([]string(nil), f.services...)
	f.mu.Unlock()
	if !reflect.DeepEqual(services, []string{rootDeploymentHealth, rootExecutionHealth}) {
		t.Fatalf("both named services must be checked: %v", services)
	}
	original, _ := metadata.FromOutgoingContext(ctx)
	if got := original.Get("cookie"); !reflect.DeepEqual(got, []string{"browser-session"}) {
		t.Fatal("caller metadata mutated")
	}
}
func TestRootRPCRejectWrongIdentityAndRelease(t *testing.T) {
	f := rootRPCServer(t, false)
	cfg := rootRPCConfig(t)
	cfg.Identity, _ = workflowrpc.NewServiceIdentity(strings.Repeat("x", 32))
	r, e := connectWorkflowRPC(context.Background(), cfg, f.dial)
	if e == nil || r != nil {
		t.Fatal("wrong identity accepted")
	}
	if strings.Contains(e.Error(), "synthetic secret") || errors.Unwrap(e) != nil {
		t.Fatal("underlying remote error escaped")
	}
	rootRPCClosed(t, f)
}
func TestRootRPCRejectOneUnhealthyService(t *testing.T) {
	f := rootRPCServer(t, false)
	f.health.SetServingStatus(rootExecutionHealth, hp.HealthCheckResponse_NOT_SERVING)
	r, e := connectWorkflowRPC(context.Background(), rootRPCConfig(t), f.dial)
	if e == nil || r != nil {
		t.Fatal("half-ready runtime accepted")
	}
	rootRPCClosed(t, f)
}
func TestRootRPCReadyTracksRecoveryAndStartupContext(t *testing.T) {
	f := rootRPCServer(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	r := rootRPCOpen(t, f, ctx)
	cancel()
	if e := r.Ready(context.Background()); e != nil {
		t.Fatal("successful connection incorrectly owned by startup context", e)
	}
	f.health.SetServingStatus(rootDeploymentHealth, hp.HealthCheckResponse_NOT_SERVING)
	if r.Ready(context.Background()) == nil {
		t.Fatal("stale readiness success")
	}
	f.health.SetServingStatus(rootDeploymentHealth, hp.HealthCheckResponse_SERVING)
	if e := r.Ready(context.Background()); e != nil {
		t.Fatal("recovery not observed", e)
	}
	if r.Ready(nil) == nil {
		t.Fatal("nil readiness context accepted")
	}
}
func TestRootRPCStartupHealthHasDeadline(t *testing.T) {
	f := rootRPCServer(t, true)
	cfg := rootRPCConfig(t)
	cfg.RPCTimeout = 75 * time.Millisecond
	start := time.Now()
	r, e := connectWorkflowRPC(context.Background(), cfg, f.dial)
	if e == nil || r != nil {
		t.Fatal("blocked health accepted")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("health exceeded configured bounded call")
	}
	rootRPCClosed(t, f)
}
func TestRootRPCInvalidConfigurationNeverDials(t *testing.T) {
	cfg := rootRPCConfig(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name string
		cfg  workflowRuntimeConfig
		ctx  context.Context
	}{{"disabled", workflowRuntimeConfig{}, context.Background()}, {"nil-context", cfg, nil}, {"canceled", cfg, canceled}}
	for _, name := range []string{"target", "identity", "timeout-zero", "timeout-high"} {
		c := cfg
		switch name {
		case "target":
			c.Target = "https://public.invalid"
		case "identity":
			c.Identity = nil
		case "timeout-zero":
			c.RPCTimeout = 0
		case "timeout-high":
			c.RPCTimeout = 31 * time.Second
		}
		cases = append(cases, struct {
			name string
			cfg  workflowRuntimeConfig
			ctx  context.Context
		}{name, c, context.Background()})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls := 0
			r, e := connectWorkflowRPC(c.ctx, c.cfg, func(string, ...grpc.DialOption) (*grpc.ClientConn, error) {
				calls++
				return nil, errors.New("must not dial")
			})
			if e == nil || r != nil || calls != 0 {
				t.Fatalf("invalid input did not fail before dialing: calls=%d err=%v", calls, e)
			}
		})
	}
	if r, e := connectWorkflowRPC(context.Background(), cfg, nil); e == nil || r != nil {
		t.Fatal("nil constructor accepted")
	}
}
func TestRootRPCDialFailureIsSanitized(t *testing.T) {
	secret := errors.New("synthetic-private-password")
	r, e := connectWorkflowRPC(context.Background(), rootRPCConfig(t), func(string, ...grpc.DialOption) (*grpc.ClientConn, error) { return nil, secret })
	if e == nil || r != nil {
		t.Fatal("dial failure accepted")
	}
	if strings.Contains(e.Error(), secret.Error()) || errors.Unwrap(e) != nil {
		t.Fatal("dial error details escaped")
	}
}
func TestRootRPCCloseIsConcurrentAndIdempotent(t *testing.T) {
	f := rootRPCServer(t, false)
	r := rootRPCOpen(t, f, context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = r.Close(); _ = r.Ready(context.Background()) }()
	}
	wg.Wait()
	if e := r.Close(); e != nil {
		t.Fatal("repeat close failed", e)
	}
	if r.Ready(context.Background()) == nil {
		t.Fatal("closed connection appears ready")
	}
	rootRPCClosed(t, f)
}
func TestRootRPCBothBusinessClientsUseSameAuthenticatedConnection(t *testing.T) {
	f := rootRPCServer(t, false)
	r := rootRPCOpen(t, f, context.Background())
	if r.Deployment == nil || r.Execution == nil {
		t.Fatal("business clients not wired")
	}
	id := func(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("cookie", "private-browser", "authorization", "wrong"))
	_, e := r.Deployment.Lookup(ctx, &pb.LookupRequest{AppId: id(1), FlowId: id(2), VersionId: id(3), Version: 1, BpmnSha256: strings.Repeat("a", 64)})
	if e != nil {
		t.Fatal("deployment lookup failed", e)
	}
	c := fc.Command{ProtocolVersion: 2, CommandID: id(10), AppID: id(1), TableID: id(4), ViewID: id(5), RecordID: id(6), FlowID: id(2), VersionID: id(3), InstanceID: id(7), ActorID: id(8), Action: "start", DefinitionVersion: 1, SchemaVersion: 1, RecordVersion: 1, FenceEpoch: 1, PayloadHash: sha256.Sum256([]byte("independent-start-fixture"))}
	_, found, e := r.Execution.Lookup(ctx, c)
	if e != nil || found {
		t.Fatalf("execution lookup failed: found=%v err=%v", found, e)
	}
	_ = r.Close()
	if _, e = r.Deployment.Lookup(context.Background(), &pb.LookupRequest{AppId: id(1), FlowId: id(2), VersionId: id(3), Version: 1, BpmnSha256: strings.Repeat("a", 64)}); e == nil {
		t.Fatal("business transport remains open after close")
	}
}
