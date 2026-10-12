export type WorkflowPublicationResult={operationId:string;flowId:string;version:number;status:'pending'|'unknown'|'confirmed'|'superseded'|'blocked';reason:string|null};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const positive=(v:unknown):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>0;
export function validPublicationRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id','version'])||resource.kind!=='workflow-publication'||!['appId','viewId','id'].every(k=>id(resource[k]))||!positive(resource.version)||!object(request))return false;
 const path=`applications/${resource.appId}/forms/${resource.viewId}/workflows/${resource.id}/publish`;
 return request.path===path&&request.method==='POST'&&request.expectedStatus===202&&object(request.body)&&exact(request.body,['expectedRevision','expectedSchemaVersion'])&&positive(request.body.expectedRevision)&&positive(request.body.expectedSchemaVersion);
}
export function validPublicationResult(value:unknown,operationId:string,flowId:string,version:number,status:number,method:'GET'|'POST'):value is WorkflowPublicationResult{
 if(!id(operationId)||!id(flowId)||!positive(version)||!object(value)||!exact(value,['operationId','flowId','version','status','reason'])||value.operationId!==operationId||value.flowId!==flowId||value.version!==version)return false;
 const reason=value.reason;let pending=false,valid=false;
 switch(value.status){
  case 'pending':pending=true;valid=reason===null;break;
  case 'unknown':pending=true;valid=typeof reason==='string'&&['DEPENDENCY_UNAVAILABLE','ENGINE_REPLY_INVALID','ENGINE_CONFLICT'].includes(reason);break;
  case 'confirmed':valid=reason===null;break;
  case 'superseded':valid=reason==='CANDIDATE_SUPERSEDED';break;
  case 'blocked':valid=typeof reason==='string'&&['AUTHORITY_REVOKED','APPROVER_INELIGIBLE','SCHEMA_INCOMPATIBLE','FLOW_CLOSED','ENGINE_CONFLICT'].includes(reason);break;
 }
 return valid&&(method==='GET'?status===200:method==='POST'&&status===(pending?202:200));
}
