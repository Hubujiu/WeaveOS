package workflowrpc

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Synthetic test material, never accepted by a deployed service.
const syntheticServiceToken = "V041_SYNTHETIC_TEST_ONLY_1234567890"

func TestRootServiceIdentityRejectsInvalidToken(t *testing.T) {
	for _, token := range []string{"", strings.Repeat("a", 31), strings.Repeat("a", 257), " " + syntheticServiceToken, syntheticServiceToken + "\n", syntheticServiceToken + "=", syntheticServiceToken + "é", syntheticServiceToken + "/", syntheticServiceToken + "\x00"} {
		identity, err := NewServiceIdentity(token)
		if err == nil || identity != nil {
			t.Errorf("invalid token accepted (length %d)", len(token))
		}
		if err != nil && token != "" && strings.Contains(err.Error(), token) {
			t.Error("error leaks input")
		}
	}
}
func TestRootServiceIdentityAcceptsBoundaries(t *testing.T) {
	for _, token := range []string{strings.Repeat("a", 32), strings.Repeat("Z", 256), syntheticServiceToken, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz-"} {
		s, err := NewServiceIdentity(token)
		if err != nil || s == nil {
			t.Fatal("valid synthetic token rejected")
		}
		if s.UnaryInterceptor() == nil {
			t.Fatal("no authentication interceptor")
		}
	}
}
func TestRootServiceIdentityRedactsFormatting(t *testing.T) {
	s, err := NewServiceIdentity(syntheticServiceToken)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		text := fmt.Sprintf(format, s)
		if text != "[REDACTED service identity]" {
			t.Errorf("unsafe or unspecified credential representation for %s: %s", format, text)
		}
	}
}
func rootIdentityInterceptor(t *testing.T) grpc.UnaryClientInterceptor {
	t.Helper()
	s, err := NewServiceIdentity(syntheticServiceToken)
	if err != nil {
		t.Fatal(err)
	}
	i := s.UnaryInterceptor()
	if i == nil {
		t.Fatal("no interceptor")
	}
	return i
}
func TestRootServiceIdentityReplacesCallerAuthorizationAndCookies(t *testing.T) {
	intercept := rootIdentityInterceptor(t)
	original := metadata.Pairs("authorization", "Bearer browser-value", "authorization", "extra-value", "cookie", "private-browser-cookie", "x-request-id", "request-41")
	ctx := metadata.NewOutgoingContext(context.Background(), original)
	var got metadata.MD
	err := intercept(ctx, "/weaveos.workflow.v1.ExecutionService/Execute", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		got, _ = metadata.FromOutgoingContext(ctx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Get("authorization"), []string{"Bearer " + syntheticServiceToken}) {
		t.Error("service identity not installed exactly once")
	}
	if len(got.Get("cookie")) != 0 {
		t.Error("browser cookie forwarded")
	}
	if !reflect.DeepEqual(got.Get("x-request-id"), []string{"request-41"}) {
		t.Error("request metadata lost")
	}
	after, _ := metadata.FromOutgoingContext(ctx)
	if !reflect.DeepEqual(after, original) {
		t.Error("mutated caller context metadata")
	}
}
func TestRootServiceIdentityPreservesCallAndDoesNotRetry(t *testing.T) {
	intercept := rootIdentityInterceptor(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	defer cancel()
	request, reply := new(int), new(int)
	conn := new(grpc.ClientConn)
	opt := grpc.WaitForReady(false)
	calls := 0
	expected := status.Error(codes.Unavailable, "synthetic transport failure")
	err := intercept(ctx, "/service/Method", request, reply, conn, func(call context.Context, method string, req, res any, c *grpc.ClientConn, opts ...grpc.CallOption) error {
		calls++
		if method != "/service/Method" || req != request || res != reply || c != conn || len(opts) != 1 {
			t.Error("call changed")
		}
		d, _ := ctx.Deadline()
		actual, _ := call.Deadline()
		if !d.Equal(actual) {
			t.Error("deadline changed")
		}
		if call.Done() != ctx.Done() {
			t.Error("cancellation lost")
		}
		return expected
	}, opt)
	if err != expected || calls != 1 {
		t.Error("error changed or invocation retried")
	}
}
func TestRootServiceIdentityRejectsUninitializedUse(t *testing.T) {
	for _, s := range []*ServiceIdentity{nil, {}} {
		i := s.UnaryInterceptor()
		if i == nil {
			t.Error("must return fail-closed interceptor")
			continue
		}
		called := false
		err := i(context.Background(), "/service/Method", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			called = true
			return nil
		})
		if err == nil || called {
			t.Error("uninitialized identity reached transport")
		}
	}
}
func TestRootServiceIdentityConcurrentCallsDoNotShareMetadata(t *testing.T) {
	intercept := rootIdentityInterceptor(t)
	var wg sync.WaitGroup
	for n := 0; n < 32; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			want := fmt.Sprint(n)
			ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-request-id", want))
			err := intercept(ctx, "/service/Method", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				md, _ := metadata.FromOutgoingContext(ctx)
				if !reflect.DeepEqual(md.Get("authorization"), []string{"Bearer " + syntheticServiceToken}) || !reflect.DeepEqual(md.Get("x-request-id"), []string{want}) {
					t.Error("cross-call metadata corruption")
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}(n)
	}
	wg.Wait()
}
