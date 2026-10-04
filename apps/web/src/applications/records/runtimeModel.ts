import type {FieldValue,HistoryChange,HistoryValueLabel,RecordItem,RuntimeField,RuntimeView,Scope,UUID} from './contracts';

export type RuntimeDisplayField={field:RuntimeField;readable:boolean;editable:boolean;value?:FieldValue};
export function scopeAllows(scope:Scope,actorId:UUID,createdBy:UUID):boolean{return scope==='all'||scope==='own'&&actorId===createdBy;}
const sameResource=(view:RuntimeView,record:RecordItem)=>record.appId===view.appId&&record.tableId===view.tableId&&record.viewId===view.viewId;
export function projectRuntimeFields(view:RuntimeView,mode:'create'|'read'|'edit',actorId:UUID,record?:RecordItem):RuntimeDisplayField[]{
 if(mode==='create')return view.capabilities.create?view.fields.filter(field=>field.access.create).map(field=>({
  field,readable:false,editable:true,...(Object.hasOwn(field,'default')?{value:field.default}:{})
 })):[];
 if(!record||!sameResource(view,record)||!scopeAllows(view.capabilities.read,actorId,record.createdBy))return [];
 return view.fields.flatMap(field=>{
  const readable=scopeAllows(field.access.read,actorId,record.createdBy);
  const editable=mode==='edit'&&scopeAllows(view.capabilities.edit,actorId,record.createdBy)&&scopeAllows(field.access.edit,actorId,record.createdBy);
  if(!readable&&!editable)return [];
  return [{field,readable,editable,...(readable&&Object.hasOwn(record.values,field.id)?{value:record.values[field.id]}:{})}];
 });
}
export function canOfferHistory(view:RuntimeView,field:RuntimeField,actorId:UUID,record:RecordItem):boolean{
 if(!sameResource(view,record))return false;
 const current=view.fields.find(candidate=>candidate.id===field.id);
 return !!current&&scopeAllows(view.capabilities.read,actorId,record.createdBy)&&scopeAllows(view.capabilities.history,actorId,record.createdBy)&&
  scopeAllows(current.access.read,actorId,record.createdBy)&&scopeAllows(current.access.history,actorId,record.createdBy);
}
export function requiresRuntimeReview(before:RuntimeView,after:RuntimeView,dirty:boolean):boolean{
 return dirty&&(before.appId!==after.appId||before.viewId!==after.viewId||before.schemaVersion!==after.schemaVersion||before.viewVersion!==after.viewVersion||before.policyRevision!==after.policyRevision);
}
export function allowedHistoryValueLabels(change:HistoryChange):Record<UUID,HistoryValueLabel>{
 if(!['single_select','multi_select','member','department'].includes(change.fieldKind))return {};
 const ids=new Set([change.before,change.after].flatMap(value=>Array.isArray(value)?value:typeof value==='string'?[value]:[]));
 return Object.fromEntries(Object.entries(change.valueLabels).filter(([id])=>ids.has(id)));
}
