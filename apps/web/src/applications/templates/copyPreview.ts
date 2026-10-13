import {validStructureCopyPlanRequest} from './copyRequest';
import {decodeTemplateFile} from './fileInput';
import {templateReferences} from './references';
import {validTemplatePreflight} from './receipts';
import type {StructureCopyPlanRequest} from './copyRequest';
import type {TemplateReferences} from './references';
import type {TemplateCounts} from './receipts';
export type StructureCopyPlan={sourceFingerprint:string;targetStructureVersion:number;targetPolicyRevision:number;manifest:Record<string,unknown>;externalReferences:TemplateReferences};
export type StructureCopyPreflight={valid:true;counts:TemplateCounts;mappedManifest:Record<string,unknown>};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
function readManifest(value:unknown,input:StructureCopyPlanRequest):Record<string,unknown>|null{
 try{
  if(!validStructureCopyPlanRequest(input))return null;
  const m=decodeTemplateFile(new TextEncoder().encode(JSON.stringify(value)));
  if(!m||!object(m.application)||m.application.id!==input.source.appId||!templateReferences(m))return null;
  const kind=input.source.kind;
  if(kind!=='application'){
   const key={directory:'directories',table:'tables',form:'forms'}[kind];
   if(!(m[key] as unknown[]).some(item=>object(item)&&item.id===input.source.id))return null;
  }
  return m;
 }catch{return null;}
}
export function validStructureCopyPlan(value:unknown,input:StructureCopyPlanRequest):value is StructureCopyPlan{
 if(!object(value)||!exact(value,['sourceFingerprint','targetStructureVersion','targetPolicyRevision','manifest','externalReferences'])||typeof value.sourceFingerprint!=='string'||!/^[0-9a-f]{64}$/.test(value.sourceFingerprint)||!integer(value.targetStructureVersion,0)||!integer(value.targetPolicyRevision,0)||!object(value.externalReferences)||!exact(value.externalReferences,['userIds','departmentIds']))return false;
 const m=readManifest(value.manifest,input);if(!m)return false;
 if(input.source.kind==='application'){
  if(value.targetStructureVersion!==0||value.targetPolicyRevision!==0)return false;
 }else if(value.targetPolicyRevision<1)return false;
 const refs=templateReferences(m)!;
 return (['userIds','departmentIds'] as const).every(key=>{const ids=value.externalReferences as Record<string,unknown>,list=ids[key];return Array.isArray(list)&&list.length===refs[key].length&&list.every((id,i)=>id===refs[key][i]);});
}
export function validStructureCopyPreflight(value:unknown,input:StructureCopyPlanRequest):value is StructureCopyPreflight{
 if(!object(value)||!exact(value,['valid','counts','mappedManifest'])||!validTemplatePreflight({valid:value.valid,counts:value.counts}))return false;
 const m=readManifest(value.mappedManifest,input);if(!m)return false;
 const tables=m.tables as {fields:unknown[]}[],counts=value.counts as TemplateCounts;
 return counts.directories===(m.directories as unknown[]).length&&counts.tables===tables.length&&counts.fields===tables.reduce((n,t)=>n+t.fields.length,0)&&counts.forms===(m.forms as unknown[]).length&&counts.workflows===(m.workflows as unknown[]).length&&counts.permissionGroups===(m.permissionGroups as unknown[]).length;
}
