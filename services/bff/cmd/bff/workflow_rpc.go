package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const workflowRPCMessageLimit = 1048576 + 16384

var errWorkflowRPCConfiguration = errors.New("invalid workflow RPC configuration")
var errWorkflowRPCUnavailable = errors.New("workflow RPC unavailable")

// workflowRPC owns one channel shared by both business clients and health checks.
// Its lifetime is independent of the context used to establish readiness.
type workflowRPC struct {
	Deployment *workflowrpc.Client
	Execution  *workflowrpc.ExecutionClient

	conn     *grpc.ClientConn
	health   healthpb.HealthClient
	timeout  time.Duration
	mu       sync.Mutex
	closed   bool
	closeErr error
}

func connectWorkflowRPC(ctx context.Context, cfg workflowRuntimeConfig, newClient func(string, ...grpc.DialOption) (*grpc.ClientConn, error)) (*workflowRPC, error) {
	if ctx == nil || ctx.Err() != nil || !cfg.Enabled || !validWorkflowTarget(cfg.Target) || cfg.RPCTimeout < time.Millisecond || cfg.RPCTimeout > 30*time.Second || cfg.Identity == nil || newClient == nil {
		return nil, errWorkflowRPCConfiguration
	}

	// One deadline covers construction and both service checks, with any earlier
	// caller deadline or cancellation retained. NewClient itself owns a channel
	// context rather than this temporary startup context.
	startup, cancel := context.WithTimeout(ctx, cfg.RPCTimeout)
	defer cancel()
	if startup.Err() != nil {
		return nil, errWorkflowRPCUnavailable
	}
	conn, err := newClient(cfg.Target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(cfg.Identity.UnaryInterceptor()),
		grpc.WithNoProxy(),
		// Policy retries are disabled; gRPC's transparent retries may remain.
		grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(workflowRPCMessageLimit),
			grpc.MaxCallRecvMsgSize(workflowRPCMessageLimit),
		),
	)
	if err != nil || conn == nil {
		if conn != nil {
			_ = conn.Close()
		}
		return nil, errWorkflowRPCUnavailable
	}
	rpc := &workflowRPC{
		conn: conn, health: healthpb.NewHealthClient(conn), timeout: cfg.RPCTimeout,
	}
	rpc.Deployment, err = workflowrpc.NewClient(conn, cfg.RPCTimeout)
	if err != nil {
		_ = rpc.Close()
		return nil, errWorkflowRPCUnavailable
	}
	rpc.Execution, err = workflowrpc.NewExecutionClient(conn, cfg.RPCTimeout)
	if err != nil {
		_ = rpc.Close()
		return nil, errWorkflowRPCUnavailable
	}
	if err := rpc.checkReady(startup); err != nil {
		_ = rpc.Close()
		return nil, errWorkflowRPCUnavailable
	}
	return rpc, nil
}

func (rpc *workflowRPC) Ready(ctx context.Context) error {
	if rpc == nil || ctx == nil || ctx.Err() != nil || rpc.timeout < time.Millisecond || rpc.timeout > 30*time.Second {
		return errWorkflowRPCUnavailable
	}
	check, cancel := context.WithTimeout(ctx, rpc.timeout)
	defer cancel()
	return rpc.checkReady(check)
}

func (rpc *workflowRPC) checkReady(ctx context.Context) error {
	if rpc == nil || ctx == nil || ctx.Err() != nil {
		return errWorkflowRPCUnavailable
	}
	rpc.mu.Lock()
	if rpc.closed || rpc.conn == nil || rpc.health == nil {
		rpc.mu.Unlock()
		return errWorkflowRPCUnavailable
	}
	health := rpc.health
	rpc.mu.Unlock()

	// Do not hold the lifecycle lock during network I/O: Close must be able to
	// cancel an in-flight check by closing the shared channel.
	for _, service := range [...]string{
		"weaveos.workflow.v1.DeploymentService",
		"weaveos.workflow.v1.ExecutionService",
	} {
		response, err := health.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		if err != nil || response == nil || response.Status != healthpb.HealthCheckResponse_SERVING {
			return errWorkflowRPCUnavailable
		}
	}
	if ctx.Err() != nil {
		return errWorkflowRPCUnavailable
	}
	rpc.mu.Lock()
	defer rpc.mu.Unlock()
	if rpc.closed {
		return errWorkflowRPCUnavailable
	}
	return nil
}

func (rpc *workflowRPC) Close() error {
	if rpc == nil {
		return nil
	}
	rpc.mu.Lock()
	defer rpc.mu.Unlock()
	if rpc.closed {
		return rpc.closeErr
	}
	rpc.closed = true
	if rpc.conn != nil {
		if err := rpc.conn.Close(); err != nil && !errors.Is(err, grpc.ErrClientConnClosing) {
			rpc.closeErr = errWorkflowRPCUnavailable
		}
	}
	return rpc.closeErr
}
