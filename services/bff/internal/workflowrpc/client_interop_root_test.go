//go:build workflowrpc_integration

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

// Explicit tag and exact isolated hostname: missing fixture is a failure, not a skip.
func TestRootGoJavaPostgresDeploymentInterop(t *testing.T) {
	target := os.Getenv("WEAVEOS_RPC_TEST_TARGET")
	host, _, e := net.SplitHostPort(target)
	if e != nil || host != "b3-workflow" {
		t.Fatal("only the dedicated unexposed b3-workflow fixture is allowed")
	}
	cc, e := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if e != nil {
		t.Fatal(e)
	}
	defer cc.Close()
	c, e := NewClient(cc, 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	q := request()
	q.BpmnXml = []byte("<definitions xmlns=\"http://www.omg.org/spec/BPMN/20100524/MODEL\" targetNamespace=\"urn:weaveos:workflow\"><process id=\"p_" + strings.ReplaceAll(versionID, "-", "") + "\" isExecutable=\"true\"><startEvent id=\"n_40000000000040008000000000000001\"></startEvent><endEvent id=\"n_40000000000040008000000000000002\"></endEvent><sequenceFlow id=\"e_0\" sourceRef=\"n_40000000000040008000000000000001\" targetRef=\"n_40000000000040008000000000000002\"></sequenceFlow></process></definitions>")
	ctx := context.Background()
	before, e := c.Lookup(ctx, lookupRequest(q))
	if e != nil || before.GetNotObserved() == nil {
		t.Fatalf("initial result %v %v", before, e)
	}
	r, e := c.Deploy(ctx, q)
	if e != nil {
		t.Fatal(e)
	}
	if r.GetAppId() != appID || r.GetFlowId() != flowID || r.GetVersionId() != versionID || r.GetVersion() != 1 || r.GetBpmnSha256() != digest(q.BpmnXml) || r.GetEngineDeploymentId() == "" || r.GetProcessDefinitionId() == "" {
		t.Fatalf("unbound receipt %v", r)
	}
	again, e := c.Deploy(ctx, q)
	if e != nil || !proto.Equal(r, again) {
		t.Fatalf("replay %v %v", again, e)
	}
	after, e := c.Lookup(ctx, lookupRequest(q))
	if e != nil || !proto.Equal(r, after.GetConfirmed()) {
		t.Fatalf("lookup %v %v", after, e)
	}
	bad := proto.Clone(q).(*pb.DeployRequest)
	bad.BpmnXml = append(bad.BpmnXml, '\n')
	if _, e = c.Deploy(ctx, bad); status.Code(e) != codes.AlreadyExists {
		t.Fatalf("content conflict %v", e)
	}
	mismatch := lookupRequest(q)
	mismatch.FlowId = appID
	if _, e = c.Lookup(ctx, mismatch); status.Code(e) != codes.FailedPrecondition {
		t.Fatalf("lookup conflict %v", e)
	}
}
