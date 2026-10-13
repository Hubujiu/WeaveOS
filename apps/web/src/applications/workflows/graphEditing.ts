import {validWorkflowGraph,type WorkflowGraph,type WorkflowNode} from './definitionContract';

/** Edits one exact edge. Approval eligibility and typed conditions remain server checked. */
export function insertWorkflowNode(graph:WorkflowGraph,edge:WorkflowGraph['edges'][number],node:WorkflowNode):WorkflowGraph|null{
 if(!validWorkflowGraph(graph)||!edge||typeof edge!=='object'||!node||!['approval','condition'].includes(node.kind)||graph.nodes.some(n=>n.id===node.id))return null;
 const keys=Object.keys(edge);
 if(!keys.includes('from')||!keys.includes('to')||keys.some(k=>!['from','to','branch'].includes(k)))return null;
 const index=graph.edges.findIndex(e=>e.from===edge.from&&e.to===edge.to&&e.branch===edge.branch&&Object.hasOwn(e,'branch')===Object.hasOwn(edge,'branch'));
 if(index<0)return null;
 const outgoing:WorkflowGraph['edges']=node.kind==='condition'
  ? [{from:node.id,to:edge.to,branch:'true'},{from:node.id,to:edge.to,branch:'false'}]
  : [{from:node.id,to:edge.to}];
 const result:WorkflowGraph={version:1,nodes:[...graph.nodes,node],edges:[...graph.edges.slice(0,index),{...edge,to:node.id},...outgoing,...graph.edges.slice(index+1)]};
 return validWorkflowGraph(result)?structuredClone(result):null;
}
