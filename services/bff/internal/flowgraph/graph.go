package flowgraph
import (
 "encoding/json"
 "errors"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
)
var ErrInvalid=errors.New("invalid workflow graph")
type Graph struct {Version int;Nodes []Node;Edges []Edge}
type Node struct {ID string;Kind string;Approval *Approval;Condition json.RawMessage}
type Approval struct {Mode string;AssigneeIDs []string;EditableFieldIDs []string}
type Edge struct {From,To,Branch string}
type Validated struct {Order,ReferencedFields []string}
func Validate(Graph,[]appquery.Field)(Validated,error){return Validated{},errors.New("root RED-only graph declaration")}
