// V075 P3: the new resource cannot broaden the ordinary record/draft whitelist.
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
export function validTaskRecordRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id','instanceId','taskId'])||resource.kind!=='task-record'||!['appId','viewId','id','instanceId','taskId'].every(k=>id(resource[k]))||!object(request))return false;
 const path=`applications/${resource.appId}/forms/${resource.viewId}/records/${resource.id}/workflow-instances/${resource.instanceId}/tasks/${resource.taskId}/record`;
 if(request.path!==path||request.method!=='PATCH'||request.expectedStatus!==200||!object(request.body)||!exact(request.body,['basisToken','changes']))return false;
 const {basisToken,changes}=request.body;
 if(typeof basisToken!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(basisToken)||!object(changes))return false;
 const entries=Object.entries(changes);
 return entries.length>=1&&entries.length<=200&&entries.every(([key,value])=>id(key)&&(value===null||typeof value==='string'||typeof value==='boolean'||Array.isArray(value)&&value.every(id)));
}
