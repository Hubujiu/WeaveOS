package workflowrpc

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ServiceIdentity struct {
	token string
}

func NewServiceIdentity(token string) (*ServiceIdentity, error) {
	if !validServiceToken(token) {
		return nil, errors.New("invalid workflow service identity")
	}
	return &ServiceIdentity{token: token}, nil
}

func validServiceToken(token string) bool {
	if len(token) < 32 || len(token) > 256 {
		return false
	}
	for i := range token {
		c := token[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// Format redacts both value and pointer representations of the identity.
func (s ServiceIdentity) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[REDACTED service identity]")
}

func (s *ServiceIdentity) UnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply any, conn *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		if s == nil || !validServiceToken(s.token) {
			return errors.New("workflow service identity unavailable")
		}
		if executionNil(ctx) || invoker == nil {
			return errors.New("invalid workflow service RPC invocation")
		}
		outgoing := metadata.MD{}
		if current, ok := metadata.FromOutgoingContext(ctx); ok {
			outgoing = current.Copy()
		}
		outgoing.Delete("cookie")
		outgoing.Set("authorization", "Bearer "+s.token)
		return invoker(metadata.NewOutgoingContext(ctx, outgoing), method, request, reply, conn, options...)
	}
}
