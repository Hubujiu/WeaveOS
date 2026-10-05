package workflowrpc
import("context";"errors";"time";pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb";"google.golang.org/grpc")
type Client struct{}
func NewClient(grpc.ClientConnInterface,time.Duration)(*Client,error){return &Client{},nil}
func(*Client)Deploy(context.Context,*pb.DeployRequest)(*pb.DeploymentReceipt,error){return nil,errors.New("not implemented")}
func(*Client)Lookup(context.Context,*pb.LookupRequest)(*pb.LookupResponse,error){return nil,errors.New("not implemented")}
