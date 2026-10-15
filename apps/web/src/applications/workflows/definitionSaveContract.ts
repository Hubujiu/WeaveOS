import {validWorkflowGraph,validWorkflowTriggers} from './definitionContract';
export type WorkflowDefinitionReceipt={operationId:string;id:string;revision:number;currentVersion:number;candidateVersion:number;state:'disabled'|'enabled'};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
export function validWorkflowDefinitionRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id'])||resource.kind!=='workflow-definition'||!['appId','viewId','id'].every(k=>id(resource[k]))||!object(request))return false;
 const path=`applications/${resource.appId}/forms/${resource.viewId}/workflows/${resource.id}/definition`,b=request.body;
 if(request.path!==path||request.method!=='PUT'||!object(b)||!exact(b,['name','expectedRevision','expectedSchemaVersion','graph','allowWithdraw','triggers']))return false;
 return typeof b.name==='string'&&!!b.name.trim()&&[...b.name].length<=100&&integer(b.expectedRevision,0)&&integer(b.expectedSchemaVersion,1)&&request.expectedStatus===(b.expectedRevision===0?201:200)&&typeof b.allowWithdraw==='boolean'&&validWorkflowGraph(b.graph)&&validWorkflowTriggers(b.triggers);
}
export function validWorkflowDefinitionReceipt(value:unknown,operationId:string,flowId:string,expectedRevision:number):value is WorkflowDefinitionReceipt{
 return id(operationId)&&id(flowId)&&integer(expectedRevision,0)&&object(value)&&exact(value,['operationId','id','revision','currentVersion','candidateVersion','state'])&&value.operationId===operationId&&value.id===flowId&&integer(value.revision,1)&&value.revision>expectedRevision&&integer(value.currentVersion,0)&&integer(value.candidateVersion,1)&&value.candidateVersion>=value.currentVersion&&(value.state==='disabled'||value.state==='enabled');
}
