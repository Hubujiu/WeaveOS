import type {ResourceFilterGroup} from '../../QueryFilterState';
import type {RecordSort} from './contracts';
export type PresetScope={appId:string;viewId:string};
export type ApplicationPresetState={name:string;filter:ResourceFilterGroup|null;sort:RecordSort;hiddenColumnIds:string[];columnOrder:string[];columnWidths:Record<string,number>};
export type ApplicationPresetValid=ApplicationPresetState&PresetScope&{id:string;version:number;invalid:false;createdAt:string;updatedAt:string};
export type ApplicationPresetInvalid={id:string;name:string;version:number;invalid:true;reason:'FIELD_UNAVAILABLE'|'PERMISSION_CHANGED'|'DEFINITION_CHANGED'};
export type ApplicationPreset=ApplicationPresetValid|ApplicationPresetInvalid;
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const positive=(v:unknown):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=1;
const text=(v:unknown):v is string=>typeof v==='string'&&!/[\u0000\p{Surrogate}]/u.test(v);
const column=(v:unknown):v is string=>id(v)||typeof v==='string'&&['id','createdBy','createdAt','updatedAt','recordVersion'].includes(v);
const columns=(v:unknown):v is string[]=>Array.isArray(v)&&v.length<=205&&v.every(column)&&new Set(v).size===v.length;
const bytes=(v:unknown)=>new TextEncoder().encode(JSON.stringify(v)).byteLength;
const oneOf=(v:unknown,allowed:readonly string[])=>typeof v==='string'&&allowed.includes(v);
const timestamp=(v:unknown):v is string=>{
 if(typeof v!=='string'||!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(v)||!Number.isFinite(Date.parse(v)))return false;
 const date=v.slice(0,10);return Number(v.slice(11,13))<24&&new Date(date+'T00:00:00Z').toISOString().slice(0,10)===date;
};
function filter(v:unknown):boolean{
 if(v===null)return true;
 let leaves=0;
 const leaf=(x:unknown):boolean=>{
  if(!object(x)||!exact(x,['fieldId','operator','value'])||!(id(x.fieldId)||x.fieldId==='createdAt'||x.fieldId==='updatedAt')||!oneOf(x.operator,['eq','neq','gt','gte','lt','lte']))return false;
  leaves++;
  return x.value===null||typeof x.value==='boolean'||text(x.value)||Array.isArray(x.value)&&x.value.every(text);
 };
 const and=(x:unknown):boolean=>object(x)&&exact(x,['operator','children'])&&x.operator==='and'&&Array.isArray(x.children)&&x.children.length>0&&x.children.every(leaf);
 const valid=and(v)||object(v)&&exact(v,['operator','children'])&&v.operator==='or'&&Array.isArray(v.children)&&v.children.length>0&&v.children.every(and);
 return !!valid&&leaves<=20&&bytes(v)<=16384;
}
export function validApplicationPreset(value:unknown,scope:PresetScope,expectedId?:string):value is ApplicationPreset{
 if(!id(scope.appId)||!id(scope.viewId)||!object(value)||!id(value.id)||(expectedId!==undefined&&value.id!==expectedId)||!positive(value.version)||!text(value.name)||!value.name.trim()||[...value.name].length>100)return false;
 if(value.invalid===true)return exact(value,['id','name','version','invalid','reason'])&&oneOf(value.reason,['FIELD_UNAVAILABLE','PERMISSION_CHANGED','DEFINITION_CHANGED']);
 if(value.invalid!==false||!exact(value,['id','appId','viewId','name','version','invalid','createdAt','updatedAt','filter','sort','hiddenColumnIds','columnOrder','columnWidths'])||value.appId!==scope.appId||value.viewId!==scope.viewId||!timestamp(value.createdAt)||!timestamp(value.updatedAt))return false;
 if(!filter(value.filter)||!columns(value.hiddenColumnIds)||!columns(value.columnOrder)||!object(value.columnWidths)||Object.keys(value.columnWidths).length>205||!Object.entries(value.columnWidths).every(([key,width])=>column(key)&&positive(width)))return false;
 const sort=value.sort;
 if(sort!==null&&(!object(sort)||!exact(sort,['fieldId','direction'])||!(id(sort.fieldId)||sort.fieldId==='createdAt'||sort.fieldId==='updatedAt')||!oneOf(sort.direction,['asc','desc'])))return false;
 return bytes({name:value.name,filter:value.filter,sort:value.sort,hiddenColumnIds:value.hiddenColumnIds,columnOrder:value.columnOrder,columnWidths:value.columnWidths})<=32768;
}
export function validApplicationPresets(value:unknown,scope:PresetScope):value is {items:ApplicationPreset[]}{
 return id(scope.appId)&&id(scope.viewId)&&object(value)&&exact(value,['items'])&&Array.isArray(value.items)&&value.items.length<=20&&value.items.every(item=>validApplicationPreset(item,scope))&&new Set(value.items.map(item=>item.id)).size===value.items.length;
}

// Declaration-only boundary, awaiting independent mutation RED.
export function validPresetMutationRequest(_resource:unknown,_request:unknown):boolean{return false;}
export function validPresetMutationReceipt(_value:unknown,_operationId:string,_presetId?:string):boolean{return false;}
