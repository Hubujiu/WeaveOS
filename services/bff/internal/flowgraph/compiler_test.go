package flowgraph
import (
 "bytes"
 "encoding/xml"
 "errors"
 "reflect"
 "strings"
 "testing"
)
type rootXML struct {
 Name xml.Name `xml:"-"`
 XMLName xml.Name
 Attrs []xml.Attr `xml:",any,attr"`
 Text string `xml:",chardata"`
 Children []rootXML `xml:",any"`
}
func (n rootXML) attr(name string)string{for _,a:=range n.Attrs{if a.Name.Local==name{return a.Value}};return ""}
func (n rootXML) names(local string)[]rootXML{var out []rootXML;if n.XMLName.Local==local{out=append(out,n)};for _,c:=range n.Children{out=append(out,c.names(local)...)};return out}
func (n rootXML) id(t *testing.T,id string)rootXML{t.Helper();var found []rootXML;var walk func(rootXML);walk=func(x rootXML){if x.attr("id")==id{found=append(found,x)};for _,c:=range x.Children{walk(c)}};walk(n);if len(found)!=1{t.Fatalf("id %q occurs %d times",id,len(found))};return found[0]}
func nodeID(n int)string{return "n_"+strings.ReplaceAll(rid(n),"-","")}
func innerID(prefix string,n int)string{return prefix+strings.ReplaceAll(rid(n),"-","")}
func compiled(t *testing.T,g Graph)rootXML{t.Helper();raw,e:=CompileBPMN(g,fields(),rid(99));if e!=nil{t.Fatal(e)};var out rootXML;if e=xml.Unmarshal(raw,&out);e!=nil{t.Fatalf("invalid XML: %v",e)};return out}
func TestRootBPMNNamespacesAndExactDefinition(t *testing.T){
 doc:=compiled(t,graph());if doc.XMLName.Local!="definitions"||doc.XMLName.Space!="http://www.omg.org/spec/BPMN/20100524/MODEL"{t.Fatalf("definitions QName %+v",doc.XMLName)}
 if doc.attr("targetNamespace")!="urn:weaveos:workflow"{t.Fatal("target namespace")}
 p:=doc.names("process");if len(p)!=1||p[0].attr("id")!=innerID("p_",99)||p[0].attr("isExecutable")!="true"{t.Fatalf("process %+v",p)}
}
func TestRootBPMNMapsGraphAndClosedEdges(t *testing.T){
 g:=graph();doc:=compiled(t,g)
 for _,n:=range []struct{id int;kind string}{{1,"startEvent"},{2,"userTask"},{3,"exclusiveGateway"},{4,"endEvent"},{5,"endEvent"}}{
 if got:=doc.id(t,nodeID(n.id)).XMLName.Local;got!=n.kind{t.Fatalf("%d %s want %s",n.id,got,n.kind)}
 }
 ids:=map[string]bool{};var walk func(rootXML);walk=func(x rootXML){if id:=x.attr("id");id!=""{if ids[id]{t.Fatal("duplicate XML id",id)};ids[id]=true};for _,c:=range x.Children{walk(c)}};walk(doc)
 flows:=doc.names("sequenceFlow");if len(flows)!=6{t.Fatalf("edge count %d",len(flows))}
 for _,f:=range flows{if !ids[f.attr("sourceRef")]||!ids[f.attr("targetRef")]{t.Fatalf("dangling XML edge %+v",f)}}
}
func TestRootBPMNConditionUsesBooleanAndDefault(t *testing.T){
 doc:=compiled(t,graph());n:=doc.id(t,nodeID(3))
 if n.attr("default")!="e_3"{t.Fatalf("false edge not default: %s",n.attr("default"))}
 e:=doc.id(t,"e_2");expr:=e.names("conditionExpression")
 if len(expr)!=1||strings.TrimSpace(expr[0].Text)!="${route_"+strings.ReplaceAll(rid(3),"-","")+" == true}"{t.Fatalf("unsafe condition expression %+v",expr)}
 if expr[0].attr("type")!="tFormalExpression"&&expr[0].attr("type")!="bpmn:tFormalExpression"{t.Fatalf("formal expression type %q",expr[0].attr("type"))}
 if len(doc.id(t,"e_3").names("conditionExpression"))!=0{t.Fatal("false branch should be default")}
}
func TestRootBPMNParallelAllAndAnyModes(t *testing.T){
 for _,mode:=range []string{"all","any"}{
 g:=graph();g.Nodes[1].Approval.Mode=mode;doc:=compiled(t,g);task:=doc.id(t,nodeID(2))
 var assigneeOK bool;for _,a:=range task.Attrs{if a.Name.Space=="http://flowable.org/bpmn"&&a.Name.Local=="assignee"&&a.Value=="${approver}"{assigneeOK=true}}
 if !assigneeOK{t.Fatal("assignee expression missing or wrong namespace")}
 mi:=task.names("multiInstanceLoopCharacteristics");if len(mi)!=1||mi[0].attr("isSequential")!="false"||mi[0].attr("collection")!=innerID("a_",2)||mi[0].attr("elementVariable")!="approver"{t.Fatalf("%s multi-instance %+v",mode,mi)}
 completion:=mi[0].names("completionCondition");want:="${wf_rejected || nrOfCompletedInstances == nrOfInstances}";if mode=="any"{want="${nrOfCompletedInstances > 0}"}
 if len(completion)!=1||strings.TrimSpace(completion[0].Text)!=want{t.Fatalf("%s completion %+v",mode,completion)}
 }
}
func TestRootBPMNRejectionIsLocalTerminal(t *testing.T){
 doc:=compiled(t,graph());gateway:=doc.id(t,innerID("g_",2))
 if gateway.XMLName.Local!="exclusiveGateway"||gateway.attr("default")!="e_1"{t.Fatal("approval result gateway")}
 if doc.id(t,"e_1").attr("sourceRef")!=innerID("g_",2){t.Fatal("normal path bypasses result gateway")}
 u:=doc.id(t,innerID("u_",2));if u.attr("sourceRef")!=nodeID(2)||u.attr("targetRef")!=innerID("g_",2){t.Fatal("user task missing gateway edge")}
 r:=doc.id(t,innerID("r_",2));if r.attr("sourceRef")!=innerID("g_",2)||r.attr("targetRef")!="reject_end"{t.Fatal("reject edge")}
 expr:=r.names("conditionExpression");if len(expr)!=1||strings.TrimSpace(expr[0].Text)!="${wf_rejected == true}"{t.Fatal("reject condition")}
 if doc.id(t,"reject_end").XMLName.Local!="endEvent"{t.Fatal("reject terminal")}
}
func TestRootBPMNNeverEmbedsUserCodeOrBusinessPayload(t *testing.T){
 g:=graph();g.Nodes[2].Condition=predicate(rid(7),"eq","<script>alert('secret')</script>")
 raw,e:=CompileBPMN(g,fields(),rid(99));if e!=nil{t.Fatal(e)}
 for _,bad:=range []string{"scriptTask","serviceTask","callActivity","executionListener","taskListener","timerEventDefinition","DOCTYPE","ENTITY","alert(","secret",rid(8),rid(6)}{if bytes.Contains(raw,[]byte(bad)){t.Fatalf("forbidden execution/payload %q in output",bad)}}
 doc:=compiled(t,g);var scan func(rootXML);scan=func(x rootXML){for _,a:=range x.Attrs{if strings.Contains(strings.ToLower(a.Name.Local),"async"){t.Fatal("async attribute not permitted")}};for _,c:=range x.Children{scan(c)}};scan(doc)
}
func TestRootBPMNRejectsUnvalidatedGraphOrDefinition(t *testing.T){
 for _,id:=range []string{"","<script>","00000000-0000-0000-0000-000000000000"}{if raw,e:=CompileBPMN(graph(),fields(),id);!errors.Is(e,ErrInvalid)||len(raw)!=0{t.Fatalf("invalid process ID accepted %q %v",id,e)}}
 g:=graph();g.Nodes[1].Kind="script";if raw,e:=CompileBPMN(g,fields(),rid(99));!errors.Is(e,ErrInvalid)||len(raw)!=0{t.Fatalf("invalid graph compiled %v",e)}
}
func TestRootBPMNStableAndDoesNotMutateGraph(t *testing.T){
 g:=graph();before:=clone(g);a,e:=CompileBPMN(g,fields(),rid(99));if e!=nil{t.Fatal(e)};b,e:=CompileBPMN(g,fields(),rid(99));if e!=nil||!bytes.Equal(a,b){t.Fatal("nondeterministic output",e)}
 if !reflect.DeepEqual(g,before){t.Fatal("compiler mutated graph")}
}
func TestRootBPMNNoApprovalNeedsNoSyntheticRejection(t *testing.T){
 g:=Graph{Version:1,Nodes:[]Node{{ID:rid(1),Kind:"start"},{ID:rid(2),Kind:"end"}},Edges:[]Edge{{rid(1),rid(2),""}}};doc:=compiled(t,g)
 if len(doc.names("userTask"))!=0||len(doc.names("endEvent"))!=1||len(doc.names("sequenceFlow"))!=1{t.Fatal("extra approval machinery in minimal graph")}
}
