export type TemplateCounts={directories:number;tables:number;fields:number;forms:number;workflows:number;permissionGroups:number};
export type TemplatePreflight={valid:true;counts:TemplateCounts};
export type TemplateImportReceipt={operationId:string;appId:string;policyRevision:1;structureVersion:1;status:'imported'};
const object=(value:unknown):value is Record<string,unknown>=>!!value&&typeof value==='object'&&!Array.isArray(value);
const exact=(value:Record<string,unknown>,keys:string[])=>Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const uuid=(value:unknown):value is string=>typeof value==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
const limits={directories:1000,tables:128,fields:25600,forms:256,workflows:128,permissionGroups:128};
export function validTemplatePreflight(value:unknown):value is TemplatePreflight{
 if(!object(value)||!exact(value,['valid','counts'])||value.valid!==true||!object(value.counts)||!exact(value.counts,Object.keys(limits)))return false;
 const counts=value.counts;
 return Object.entries(limits).every(([key,max])=>typeof counts[key]==='number'&&Number.isSafeInteger(counts[key])&&(counts[key] as number)>=0&&(counts[key] as number)<=max);
}
export function validTemplateImportReceipt(value:unknown,operationId:string):value is TemplateImportReceipt{
 return uuid(operationId)&&object(value)&&exact(value,['operationId','appId','policyRevision','structureVersion','status'])&&value.operationId===operationId&&uuid(value.appId)&&value.policyRevision===1&&value.structureVersion===1&&value.status==='imported';
}
