export type WorkflowManagementAction='enable'|'close';
export type WorkflowManagementReceipt={operationId:string;id:string;revision:number;currentVersion:number;candidateVersion:number;state:'enabled'|'disabled'|'closing'};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
export function validWorkflowManagementRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id','action'])||resource.kind!=='workflow-management'||!['enable','close'].includes(resource.action as string)||!['appId','viewId','id'].every(k=>id(resource[k]))||!object(request))return false;
 return request.path===`applications/${resource.appId}/forms/${resource.viewId}/workflows/${resource.id}/${resource.action}`&&request.method==='POST'&&request.expectedStatus===200&&object(request.body)&&exact(request.body,['expectedRevision'])&&integer(request.body.expectedRevision,1);
}
export function validWorkflowManagementReceipt(value:unknown,operationId:string,flowId:string,expectedRevision:number,action:WorkflowManagementAction):value is WorkflowManagementReceipt{
 if(!id(operationId)||!id(flowId)||!integer(expectedRevision,1)||!['enable','close'].includes(action)||!object(value)||!exact(value,['operationId','id','revision','currentVersion','candidateVersion','state'])||value.operationId!==operationId||value.id!==flowId||!integer(value.revision,expectedRevision)||!integer(value.currentVersion,action==='enable'?1:0)||!integer(value.candidateVersion,1)||value.candidateVersion<value.currentVersion)return false;
 return action==='enable'?value.state==='enabled':value.state==='disabled'||value.state==='closing';
}
