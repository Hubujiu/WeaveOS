export type StructureDeletionKind='application'|'directory'|'table'|'form';
export type StructureDeletionResource={kind:'structure-deletion';appId:string;resourceKind:StructureDeletionKind;id:string};
export type StructureDeletionReceipt={operationId:string;appId:string;resourceKind:StructureDeletionKind;id:string;structureVersion:number;deleted:true};
export type StructureDeletionContext={resource:StructureDeletionResource;operationId:string;expectedStructureVersion:number};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const allowed:Record<StructureDeletionKind,readonly string[]>={application:['directories','tables','forms','permission_groups','workflows'],directory:['directories','tables','forms','grants'],table:['forms','workflows','drafts','pending_commands','records'],form:['workflows','drafts','pending_commands','grants']};
function validResource(v:unknown):v is StructureDeletionResource{
 return object(v)&&exact(v,['kind','appId','resourceKind','id'])&&v.kind==='structure-deletion'&&uuid(v.appId)&&uuid(v.id)&&typeof v.resourceKind==='string'&&Object.hasOwn(allowed,v.resourceKind)&&(v.resourceKind!=='application'||v.id===v.appId);
}
export function validStructureDeletionRequest(resource:unknown,request:unknown):boolean{
 if(!validResource(resource)||!object(request)||!object(request.body))return false;
 const base='applications/'+resource.appId;
 const path=resource.resourceKind==='application'?base+'/deletion':base+'/'+({directory:'directories',table:'tables',form:'forms'}[resource.resourceKind])+'/'+resource.id+'/deletion';
 const body=request.body;
 return request.path===path&&request.method==='POST'&&request.expectedStatus===200&&exact(body,['expectedStructureVersion','expectedResourceVersion'])&&integer(body.expectedStructureVersion,0)&&body.expectedStructureVersion<Number.MAX_SAFE_INTEGER&&integer(body.expectedResourceVersion,resource.resourceKind==='application'?1:0)&&(resource.resourceKind!=='directory'||body.expectedResourceVersion===0);
}
export function validStructureDeletionReceipt(value:unknown,context:StructureDeletionContext):value is StructureDeletionReceipt{
 if(!object(context)||!validResource(context.resource)||!uuid(context.operationId)||!integer(context.expectedStructureVersion,0)||context.expectedStructureVersion>=Number.MAX_SAFE_INTEGER)return false;
 return object(value)&&exact(value,['operationId','appId','resourceKind','id','structureVersion','deleted'])&&value.operationId===context.operationId&&value.appId===context.resource.appId&&value.resourceKind===context.resource.resourceKind&&value.id===context.resource.id&&value.structureVersion===context.expectedStructureVersion+1&&value.deleted===true;
}
export function validStructureDeletionDependencies(value:unknown,kind:StructureDeletionKind):value is {dependencies:string[]}{
 return Object.hasOwn(allowed,kind)&&object(value)&&exact(value,['dependencies'])&&Array.isArray(value.dependencies)&&value.dependencies.length>0&&value.dependencies.length<=8&&new Set(value.dependencies).size===value.dependencies.length&&value.dependencies.every(d=>typeof d==='string'&&allowed[kind].includes(d));
}
