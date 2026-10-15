import type {StructureCopyKind} from './copyReceipt';
export type StructureCopyPlanRequest={source:{appId:string;kind:StructureCopyKind;id:string};target:{appId:string|null;directoryId:string|null;name:string}};
export type StructureCopyPreflightRequest=StructureCopyPlanRequest&{expectedSourceFingerprint:string;expectedTargetStructureVersion:number;expectedTargetPolicyRevision:number;bindings:{kind:'user'|'department';sourceId:string;targetId:string}[];groupMappings:{sourceGroupId:string;name:string}[]};
export type StructureCopyResource={kind:'structure-copy';sourceAppId:string;sourceKind:StructureCopyKind;sourceId:string};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const kinds:readonly string[]=['application','directory','table','form'];
const planKeys=['source','target'],preflightKeys=[...planKeys,'expectedSourceFingerprint','expectedTargetStructureVersion','expectedTargetPolicyRevision','bindings','groupMappings'];
function name(v:unknown):v is string{
 if(typeof v!=='string'||v.includes('\0')||Array.from(v).some(c=>{const n=c.codePointAt(0)!;return n>=0xd800&&n<=0xdfff;}))return false;
 // Go TrimSpace uses Unicode White_Space; JS trim differs for NEL and BOM.
 const trimmed=v.replace(/^\p{White_Space}+|\p{White_Space}+$/gu,'');
 return Array.from(trimmed).length>=1&&Array.from(trimmed).length<=100;
}
function withinBudget(v:unknown):boolean{try{return new TextEncoder().encode(JSON.stringify(v)).byteLength<=4194304;}catch{return false;}}
function planShape(v:Record<string,unknown>):boolean{
 if(!object(v.source)||!exact(v.source,['appId','kind','id'])||!uuid(v.source.appId)||!uuid(v.source.id)||typeof v.source.kind!=='string'||!kinds.includes(v.source.kind)||!object(v.target)||!exact(v.target,['appId','directoryId','name'])||!name(v.target.name))return false;
 if(v.source.kind==='application')return v.source.id===v.source.appId&&v.target.appId===null&&v.target.directoryId===null;
 return uuid(v.target.appId)&&(v.target.directoryId===null||uuid(v.target.directoryId));
}
export function validStructureCopyPlanRequest(value:unknown):value is StructureCopyPlanRequest{
 return object(value)&&exact(value,planKeys)&&planShape(value)&&withinBudget(value);
}
export function validStructureCopyPreflightRequest(value:unknown):value is StructureCopyPreflightRequest{
 if(!object(value)||!exact(value,preflightKeys)||!planShape(value)||typeof value.expectedSourceFingerprint!=='string'||!/^[0-9a-f]{64}$/.test(value.expectedSourceFingerprint)||!integer(value.expectedTargetStructureVersion,0)||!integer(value.expectedTargetPolicyRevision,0)||!Array.isArray(value.bindings)||!Array.isArray(value.groupMappings)||value.groupMappings.length>128||!withinBudget(value))return false;
 if((value.source as StructureCopyPlanRequest['source']).kind==='application'){
  if(value.expectedTargetStructureVersion!==0||value.expectedTargetPolicyRevision!==0)return false;
 }else if(value.expectedTargetPolicyRevision<1)return false;
 const bindings=new Set<string>(),groups=new Set<string>();
 for(const b of value.bindings){
  if(!object(b)||!exact(b,['kind','sourceId','targetId'])||!['user','department'].includes(b.kind as string)||!uuid(b.sourceId)||!uuid(b.targetId))return false;
  const key=b.kind+':'+b.sourceId;if(bindings.has(key))return false;bindings.add(key);
 }
 for(const g of value.groupMappings){
  if(!object(g)||!exact(g,['sourceGroupId','name'])||!uuid(g.sourceGroupId)||!name(g.name)||groups.has(g.sourceGroupId))return false;groups.add(g.sourceGroupId);
 }
 return true;
}
export function validStructureCopyRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','sourceAppId','sourceKind','sourceId'])||resource.kind!=='structure-copy'||!object(request)||request.path!=='structure-copies/execute'||request.method!=='POST'||request.expectedStatus!==201||!validStructureCopyPreflightRequest(request.body))return false;
 return resource.sourceAppId===request.body.source.appId&&resource.sourceKind===request.body.source.kind&&resource.sourceId===request.body.source.id;
}
