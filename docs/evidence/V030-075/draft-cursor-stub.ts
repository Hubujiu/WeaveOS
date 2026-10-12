import type {Draft,DraftSummary} from './contracts';
const summaryKeys=['id','viewId','tableId','targetRecordId','schemaVersion','baseRecordVersion','draftVersion','createdAt','updatedAt','hasConflicts'];
const object=(value:unknown):value is Record<string,unknown>=>value!==null&&typeof value==='object'&&!Array.isArray(value);
const exact=(value:Record<string,unknown>,keys:readonly string[])=>Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const id=(value:unknown):value is string=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
const version=(value:unknown,minimum:number)=>typeof value==='number'&&Number.isSafeInteger(value)&&value>=minimum;
const timestamp=(value:unknown)=>typeof value==='string'&&/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value)&&Number.isFinite(Date.parse(value));
function summaryFields(value:Record<string,unknown>,viewId:string,tableId:string):boolean{
 return id(viewId)&&id(tableId)&&id(value.id)&&value.viewId===viewId&&value.tableId===tableId&&
  ((value.targetRecordId===null&&value.baseRecordVersion===null)||(id(value.targetRecordId)&&version(value.baseRecordVersion,1)))&&
  version(value.schemaVersion,0)&&version(value.draftVersion,1)&&timestamp(value.createdAt)&&timestamp(value.updatedAt)&&typeof value.hasConflicts==='boolean';
}
export function validDraftSummary(value:unknown,viewId:string,tableId:string):value is DraftSummary{
 return object(value)&&exact(value,summaryKeys)&&summaryFields(value,viewId,tableId);
}
export function validDraft(value:unknown,viewId:string,tableId:string,draftId:string,fieldIds:readonly string[]):value is Draft{
 if(!id(draftId)||!object(value)||!exact(value,[...summaryKeys,'values','conflicts'])||!summaryFields(value,viewId,tableId)||value.id!==draftId||!object(value.values)||!Array.isArray(value.conflicts))return false;
 const allowed=new Set(fieldIds);
 // Draft text is intentionally incomplete: no required, decimal or reference
 // business validation here. Current runtime authorization still bounds keys.
 if(!Object.entries(value.values).every(([key,entry])=>id(key)&&allowed.has(key)&&(entry===null||typeof entry==='string'||typeof entry==='boolean'||Array.isArray(entry)&&entry.every(item=>typeof item==='string'))))return false;
 return value.hasConflicts===(value.conflicts.length>0)&&value.conflicts.every(conflict=>object(conflict)&&exact(conflict,['fieldId','reason'])&&(conflict.fieldId===null||id(conflict.fieldId))&&typeof conflict.reason==='string'&&['FIELD_REMOVED','FIELD_PERMISSION_REVOKED','SCHEMA_CHANGED','BASE_RECORD_CHANGED'].includes(conflict.reason));
}

export function validDraftPage(data:unknown,pagination:unknown,viewId:string,tableId:string,pageSize:number):data is {items:DraftSummary[]}{return false;}
