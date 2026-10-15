export type WorkflowDeletionResult={operationId:string;flowId:string;status:'pending'|'unknown'|'deleted';reason:string|null;deletedVersions:number|null;deletedAt:string|null;completedAt:string|null};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const utc=(v:unknown):v is string=>typeof v==='string'&&/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(v)&&Number.isFinite(Date.parse(v))&&new Date(v).toISOString().slice(0,19)===v.slice(0,19);
export function validWorkflowDeletionRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id'])||resource.kind!=='workflow-deletion'||!['appId','viewId','id'].every(k=>id(resource[k]))||!object(request))return false;
 return request.path===`applications/${resource.appId}/forms/${resource.viewId}/workflows/${resource.id}/delete`&&request.method==='POST'&&request.expectedStatus===202&&object(request.body)&&exact(request.body,['expectedRevision'])&&integer(request.body.expectedRevision,1);
}
export function validWorkflowDeletionResult(value:unknown,operationId:string,flowId:string,status:number,method:'GET'|'POST'):value is WorkflowDeletionResult{
 if(!id(operationId)||!id(flowId)||!object(value)||!exact(value,['operationId','flowId','status','reason','deletedVersions','deletedAt','completedAt'])||value.operationId!==operationId||value.flowId!==flowId||!['pending','unknown','deleted'].includes(value.status as string))return false;
 if(value.reason!==null&&!(typeof value.reason==='string'&&['waiting_work','engine_unavailable','engine_rejected','receipt_invalid','cleanup_retry'].includes(value.reason)))return false;
 const engineConfirmed=integer(value.deletedVersions,0)&&utc(value.deletedAt);
 if(!engineConfirmed&&(value.deletedVersions!==null||value.deletedAt!==null))return false;
 if(value.status==='deleted'?(!engineConfirmed||!utc(value.completedAt)||value.reason!==null):value.completedAt!==null)return false;
 return method==='GET'?status===200:method==='POST'&&status===(value.status==='deleted'?200:202);
}
