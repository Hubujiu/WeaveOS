export type RecordLifecycleState={id:string;recordVersion:number;schemaVersion:number;deleted:boolean};
export type RecordLifecycleResult=RecordLifecycleState&{operationId:string};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const action=(v:unknown)=>v==='deletion'||v==='restoration';
export function validRecordLifecycleRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id','action'])||resource.kind!=='record-lifecycle'||!['appId','viewId','id'].every(k=>uuid(resource[k]))||!action(resource.action)||!object(request))return false;
 const base=`applications/${resource.appId}/forms/${resource.viewId}/records/${resource.id}/${resource.action}`,body=request.body;
 return request.path===base&&request.method==='POST'&&request.expectedStatus===200&&object(body)&&exact(body,['expectedSchemaVersion','expectedRecordVersion'])&&integer(body.expectedSchemaVersion,0)&&integer(body.expectedRecordVersion,1);
}
function validStateFields(value:Record<string,unknown>,recordId:string):boolean{
 return uuid(recordId)&&value.id===recordId&&integer(value.recordVersion,1)&&integer(value.schemaVersion,1)&&typeof value.deleted==='boolean';
}
export function validRecordLifecycleState(value:unknown,recordId:string):value is RecordLifecycleState{
 return object(value)&&exact(value,['id','recordVersion','schemaVersion','deleted'])&&validStateFields(value,recordId);
}
export function validRecordLifecycleReceipt(value:unknown,operationId:string,recordId:string,recordVersion:number,schemaVersion:number,requestedAction:'deletion'|'restoration'):value is RecordLifecycleResult{
 return uuid(operationId)&&integer(recordVersion,1)&&integer(schemaVersion,0)&&action(requestedAction)&&object(value)&&exact(value,['operationId','id','recordVersion','schemaVersion','deleted'])&&validStateFields(value,recordId)&&value.operationId===operationId&&integer(value.recordVersion,2)&&value.recordVersion>recordVersion&&value.schemaVersion===schemaVersion&&value.deleted===(requestedAction==='deletion');
}
