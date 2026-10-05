package appworkflows
import("context";"errors";pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb")
type DeploymentClient interface {
 Deploy(context.Context,*pb.DeployRequest)(*pb.DeploymentReceipt,error)
 Lookup(context.Context,*pb.LookupRequest)(*pb.LookupResponse,error)
}
// Root compilation-only declarations; no publication behavior exists before RED.
func(a *Application)DispatchPublication(context.Context)(bool,error){return false,errors.New("publication dispatch not implemented")}
func(a *Application)RunPublications(context.Context)error{return errors.New("publication worker not implemented")}
