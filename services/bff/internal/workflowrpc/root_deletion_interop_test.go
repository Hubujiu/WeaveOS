//go:build workflow_deletion_integration

package workflowrpc

import (
	"context"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRootGoJavaActualDeletionRuntimeInterop(t *testing.T) {
	target := os.Getenv("WEAVEOS_V067_RPC_TARGET")
	host, _, err := net.SplitHostPort(target)
	if err != nil || (host != "127.0.0.1" && host != "b3-workflow") {
		t.Fatal("only isolated runtime fixture permitted")
	}
	identity, err := NewServiceIdentity("v067_synthetic_internal_test_identity_only")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry(), grpc.WithUnaryInterceptor(identity.UnaryInterceptor()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c, err := NewClient(conn, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	q := request()
	q.BpmnXml = []byte("<definitions xmlns=\"http://www.omg.org/spec/BPMN/20100524/MODEL\" targetNamespace=\"urn:weaveos:workflow\"><process id=\"p_" + strings.ReplaceAll(versionID, "-", "") + "\" isExecutable=\"true\"><startEvent id=\"n_40000000000040008000000000000001\"/><endEvent id=\"n_40000000000040008000000000000002\"/><sequenceFlow id=\"e_0\" sourceRef=\"n_40000000000040008000000000000001\" targetRef=\"n_40000000000040008000000000000002\"/></process></definitions>")
	original, err := c.Deploy(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	before, err := c.LookupFlowDeletion(ctx, deletionRequest())
	if err != nil || before.GetNotObserved() == nil {
		t.Fatalf("initial lookup %v %v", before, err)
	}
	receipt, err := c.DeleteFlow(ctx, deletionRequest())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.DeletedVersions != 1 || receipt.DeletedAtSeconds < 1 || receipt.DeletedAtNanos%1000 != 0 {
		t.Fatal("invalid committed result")
	}
	replay, err := c.DeleteFlow(ctx, deletionRequest())
	if err != nil || !proto.Equal(receipt, replay) {
		t.Fatalf("changed replay %v %v", replay, err)
	}
	found, err := c.LookupFlowDeletion(ctx, deletionRequest())
	if err != nil || !proto.Equal(receipt, found.GetConfirmed()) {
		t.Fatalf("changed lookup %v %v", found, err)
	}
	old, err := c.Deploy(ctx, q)
	if err != nil || !proto.Equal(original, old) {
		t.Fatalf("old publication replay %v %v", old, err)
	}
	next := proto.Clone(q).(*pb.DeployRequest)
	next.Version = 2
	next.VersionId = "30000000-0000-4000-8000-000000000002"
	next.BpmnXml = []byte(strings.ReplaceAll(string(q.BpmnXml), strings.ReplaceAll(versionID, "-", ""), strings.ReplaceAll(next.VersionId, "-", "")))
	if _, err = c.Deploy(ctx, next); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("retired identity resurrected %v", err)
	}
	anonymous, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if err != nil {
		t.Fatal(err)
	}
	defer anonymous.Close()
	unauth, _ := NewClient(anonymous, time.Second)
	if _, err = unauth.DeleteFlow(ctx, deletionRequest()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated deletion %v", err)
	}
	if _, err = unauth.LookupFlowDeletion(ctx, deletionRequest()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated lookup %v", err)
	}
}
