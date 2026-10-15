import {validWorkflowGraph,type WorkflowGraph,type WorkflowNode} from './definitionContract';
// Display only: edges remain authoritative and are never synthesized from rows.
export function workflowLayers(graph:WorkflowGraph):WorkflowNode[][]|null{
 if(!validWorkflowGraph(graph))return null;
 const byId=new Map(graph.nodes.map(n=>[n.id,n])),incoming=new Map(graph.nodes.map(n=>[n.id,0])),outgoing=new Map(graph.nodes.map(n=>[n.id,[] as string[]]));
 for(const edge of graph.edges){incoming.set(edge.to,incoming.get(edge.to)!+1);outgoing.get(edge.from)!.push(edge.to);}
 let ready=graph.nodes.filter(n=>incoming.get(n.id)===0);const layers:WorkflowNode[][]=[];
 while(ready.length){
  layers.push(ready);const next:WorkflowNode[]=[];
  for(const node of ready)for(const id of outgoing.get(node.id)!){const left=incoming.get(id)!-1;incoming.set(id,left);if(left===0)next.push(byId.get(id)!);}
  ready=next;
 }
 return layers;
}
