package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"testing"
)

func identityConfig(token string) map[string]string {
	return map[string]string{
		"WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_TARGET": "flowable:50051", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": token,
	}
}
func TestRootWorkflowIdentityRequiredWhenEnabled(t *testing.T) {
	if _, err := readWorkflowRuntimeConfig(wfGet(identityConfig(""))); err == nil {
		t.Fatal("enabled runtime accepted without service identity")
	}
	if _, err := readConfig(wfGet(identityConfig(""))); err == nil {
		t.Fatal("main configuration accepted missing service identity")
	}
}
func TestRootWorkflowIdentityRejectedWhenDisabled(t *testing.T) {
	for _, enabled := range []string{"", "false"} {
		if _, err := readWorkflowRuntimeConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": enabled, "WEAVEOS_WORKFLOW_SERVICE_TOKEN": "V041_SYNTHETIC_TEST_ONLY_1234567890"})); err == nil {
			t.Fatal("disabled mode ignored a remaining service token")
		}
	}
}
func TestRootWorkflowIdentityValidatesWithoutEchoingInvalidValues(t *testing.T) {
	for _, token := range []string{"short", strings.Repeat("A", 31), strings.Repeat("A", 257), strings.Repeat("A", 31) + " ", strings.Repeat("A", 31) + "汉", strings.Repeat("A", 31) + "\n"} {
		_, err := readWorkflowRuntimeConfig(wfGet(identityConfig(token)))
		if err == nil {
			t.Error("malformed service identity accepted")
			continue
		}
		if strings.Contains(err.Error(), token) {
			t.Fatal("configuration error echoed service identity")
		}
	}
}
func TestRootWorkflowIdentityActuallyFeedsExistingRpcInterceptor(t *testing.T) {
	token := "V041_SYNTHETIC_TEST_ONLY_1234567890"
	env := identityConfig(token)
	c, err := readWorkflowRuntimeConfig(wfGet(env))
	if err != nil {
		t.Fatal(err)
	}
	if c.Identity == nil {
		t.Fatal("parsed enabled configuration lacks actual identity")
	}
	env["WEAVEOS_WORKFLOW_SERVICE_TOKEN"] = "changed-after-read"
	calls := 0
	err = c.Identity.UnaryInterceptor()(context.Background(), "/synthetic/check", nil, nil, nil, func(ctx context.Context, _ string, _ any, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		calls++
		md, _ := metadata.FromOutgoingContext(ctx)
		got := md.Get("authorization")
		if len(got) != 1 || got[0] != "Bearer "+token {
			t.Fatal("parsed identity did not feed fixed RPC authorization")
		}
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("identity call failed: calls=%d err=%v", calls, err)
	}
}
func TestRootWorkflowIdentityConfigurationRemainsRedacted(t *testing.T) {
	for _, token := range []string{strings.Repeat("A", 32), strings.Repeat("B", 256)} {
		c, err := readWorkflowRuntimeConfig(wfGet(identityConfig(token)))
		if err != nil {
			t.Fatal(err)
		}
		if c.Identity == nil {
			t.Fatal("identity not retained")
		}
		for _, out := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprintf("%+v", c.Identity)} {
			if strings.Contains(out, token) {
				t.Fatal("workflow configuration printed service secret")
			}
		}
	}
}
