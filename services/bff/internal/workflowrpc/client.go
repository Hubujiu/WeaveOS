// Package workflowrpc validates deployment transport without owning publishing or credentials.
package workflowrpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const maxXML = 1024 * 1024
const maxMessage = maxXML + 16384
const maxVersion = 9007199254740991

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var canonicalHash = regexp.MustCompile(`^[0-9a-f]{64}$`)
var errInvalid = errors.New("invalid deployment RPC input")
var errReceipt = errors.New("unbound deployment RPC result")

type Client struct {
	service pb.DeploymentServiceClient
	timeout time.Duration
}

// The injected connection owns credentials and must disable transport retries.
// Client issues one RPC; transport failure never becomes a rollback verdict.
func NewClient(conn grpc.ClientConnInterface, timeout time.Duration) (*Client, error) {
	if conn == nil || timeout <= 0 || timeout > 30*time.Second {
		return nil, errInvalid
	}
	v := reflect.ValueOf(conn)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
		return nil, errInvalid
	}
	return &Client{service: pb.NewDeploymentServiceClient(conn), timeout: timeout}, nil
}
func (c *Client) Deploy(ctx context.Context, in *pb.DeployRequest) (*pb.DeploymentReceipt, error) {
	if c == nil || c.service == nil || ctx == nil || in == nil || !validIdentity(in.AppId, in.FlowId, in.VersionId, in.Version) || len(in.BpmnXml) == 0 || len(in.BpmnXml) > maxXML || !utf8.Valid(in.BpmnXml) || proto.Size(in) > maxMessage {
		return nil, errInvalid
	}
	q := proto.Clone(in).(*pb.DeployRequest)
	h := sha256.Sum256(q.BpmnXml)
	hash := hex.EncodeToString(h[:])
	call, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	r, err := c.service.Deploy(call, q)
	if err != nil {
		return nil, err
	}
	if !bound(r, q.AppId, q.FlowId, q.VersionId, q.Version, hash) {
		return nil, errReceipt
	}
	return r, nil
}
func (c *Client) Lookup(ctx context.Context, in *pb.LookupRequest) (*pb.LookupResponse, error) {
	if c == nil || c.service == nil || ctx == nil || in == nil || !validIdentity(in.AppId, in.FlowId, in.VersionId, in.Version) || !canonicalHash.MatchString(in.BpmnSha256) || proto.Size(in) > maxMessage {
		return nil, errInvalid
	}
	q := proto.Clone(in).(*pb.LookupRequest)
	call, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	r, err := c.service.Lookup(call, q)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errReceipt
	}
	switch result := r.Result.(type) {
	case *pb.LookupResponse_Confirmed:
		if result == nil || !bound(result.Confirmed, q.AppId, q.FlowId, q.VersionId, q.Version, q.BpmnSha256) {
			return nil, errReceipt
		}
	case *pb.LookupResponse_NotObserved:
		if result == nil || result.NotObserved == nil {
			return nil, errReceipt
		}
	default:
		return nil, errReceipt
	}
	return r, nil
}
func validIdentity(app, flow, versionID string, version uint64) bool {
	if version < 1 || version > maxVersion {
		return false
	}
	for _, id := range []string{app, flow, versionID} {
		if !canonicalUUID.MatchString(id) || id == "00000000-0000-0000-0000-000000000000" {
			return false
		}
	}
	return true
}
func bound(r *pb.DeploymentReceipt, app, flow, versionID string, version uint64, hash string) bool {
	return r != nil && r.AppId == app && r.FlowId == flow && r.VersionId == versionID && r.Version == version && r.BpmnSha256 == hash && strings.TrimSpace(r.EngineDeploymentId) != "" && strings.TrimSpace(r.ProcessDefinitionId) != ""
}
