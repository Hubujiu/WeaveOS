import type {Field} from '../forms/contracts';
import type {RecordItem} from '../records/contracts';
export type TaskLocation={appId:string;viewId:string;recordId:string;instanceId:string;taskId:string};
export type TaskPreview={basisToken:string;task:{id:string;instanceId:string;nodeId:string;activationEpoch:number;sequence:number};record:RecordItem;fields:Field[];editableFieldIds:string[]};
const uuid=/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/;
const id=(v:unknown):v is string=>typeof v==='string'&&uuid.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const integer=(v:unknown,min:number,max=Number.MAX_SAFE_INTEGER):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min&&v<=max;
const nullableText=(v:unknown)=>v===null||typeof v==='string';
function date(v:unknown):v is string{
 if(typeof v!=='string')return false;const m=/^(\d{4})-(\d{2})-(\d{2})$/.exec(v);if(!m)return false;
 const year=Number(m[1]),month=Number(m[2]),day=Number(m[3]);const leap=year%4===0&&(year%100!==0||year%400===0);
 return month>=1&&month<=12&&day>=1&&day<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1];
}
function timestamp(v:unknown):v is string{
 if(typeof v!=='string')return false;
 const m=/^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.exec(v);
 return !!m&&date(m[1])&&Number(m[2])<24&&Number(m[3])<60&&Number(m[4])<60&&Number.isFinite(Date.parse(v));
}
function value(kind:unknown,v:unknown):boolean{
 if(v===null)return true;
 if(kind==='boolean')return typeof v==='boolean';
 if(kind==='multi_select')return Array.isArray(v)&&v.every(id)&&new Set(v).size===v.length;
 if(kind==='member'||kind==='department'||kind==='single_select')return id(v);
 if(typeof v!=='string')return false;
 if(kind==='number'||kind==='money')return /^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(v);
 if(kind==='date')return date(v);
 if(kind==='datetime')return timestamp(v);
 return kind==='text'||kind==='multiline';
}
function field(v:unknown):v is Field{
 if(!object(v)||!exact(v,['id','name','kind','required','default','config','presentation'])||!id(v.id)||typeof v.name!=='string'||!v.name.trim()||Array.from(v.name).length>100||typeof v.required!=='boolean'||!object(v.config)||!object(v.presentation))return false;
 const c=v.config,p=v.presentation;
 if(!exact(p,['helpText','displayTimeZone'])||!nullableText(p.helpText)||!nullableText(p.displayTimeZone)||!value(v.kind,v.default))return false;
 switch(v.kind){
  case 'text':case 'multiline':return exact(c,['maxLength'])&&(c.maxLength===null||integer(c.maxLength,1));
  case 'number':case 'money':return exact(c,['precision','scale','roundingPlaces','roundingMode'])&&integer(c.precision,1,38)&&integer(c.scale,0,18)&&c.scale<=c.precision&&integer(c.roundingPlaces,-18,18)&&c.roundingPlaces<=c.scale&&typeof c.roundingMode==='string'&&['HALF_UP','HALF_EVEN','TOWARD_ZERO','FLOOR','CEILING'].includes(c.roundingMode);
  case 'datetime':return exact(c,['precision'])&&typeof c.precision==='string'&&['minute','second','millisecond'].includes(c.precision);
  case 'single_select':case 'multi_select':return exact(c,['options'])&&Array.isArray(c.options)&&c.options.every(o=>object(o)&&exact(o,['id','label'])&&id(o.id)&&typeof o.label==='string')&&new Set(c.options.map(o=>o.id)).size===c.options.length;
  case 'date':case 'boolean':case 'member':case 'department':return exact(c,[]);
  default:return false;
 }
}
export function validTaskPreview(v:unknown,location:TaskLocation):v is TaskPreview{
 if(!Object.values(location).every(id)||!object(v)||!exact(v,['basisToken','task','record','fields','editableFieldIds'])||typeof v.basisToken!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(v.basisToken)||!object(v.task)||!object(v.record))return false;
 const t=v.task,r=v.record;
 if(!exact(t,['id','instanceId','nodeId','activationEpoch','sequence'])||t.id!==location.taskId||t.instanceId!==location.instanceId||!id(t.nodeId)||!integer(t.activationEpoch,1)||!integer(t.sequence,0))return false;
 if(!exact(r,['id','appId','viewId','tableId','createdBy','createdAt','updatedAt','recordVersion','schemaVersion','values','referenceDisplays'])||r.id!==location.recordId||r.appId!==location.appId||r.viewId!==location.viewId||!id(r.tableId)||!id(r.createdBy)||!timestamp(r.createdAt)||!timestamp(r.updatedAt)||!integer(r.recordVersion,1)||!integer(r.schemaVersion,0)||!object(r.values)||!object(r.referenceDisplays))return false;
 if(!Array.isArray(v.fields)||v.fields.length>1600||!v.fields.every(field))return false;
 const fields=new Map(v.fields.map(f=>[f.id,f]));
 if(fields.size!==v.fields.length||!exact(r.values,[...fields.keys()]))return false;
 for(const [key,val] of Object.entries(r.values))if(!value(fields.get(key)!.kind,val))return false;
 if(!Array.isArray(v.editableFieldIds)||v.editableFieldIds.length>200||!v.editableFieldIds.every(key=>id(key)&&fields.has(key))||new Set(v.editableFieldIds).size!==v.editableFieldIds.length)return false;
 for(const [key,displays] of Object.entries(r.referenceDisplays)){
  const f=fields.get(key);if(!f||!['member','department','single_select','multi_select'].includes(f.kind)||!object(displays))return false;
  const selected=r.values[key],selectedIds=Array.isArray(selected)?selected:[selected];
  for(const [key,d] of Object.entries(displays))if(!id(key)||!selectedIds.includes(key)||!object(d)||!exact(d,['id','label','deleted'])||d.id!==key||typeof d.label!=='string'||typeof d.deleted!=='boolean')return false;
 }
 return true;
}
