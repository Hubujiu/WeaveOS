import type {HistoryChange,HistoryEvent} from './contracts';
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const version=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const value=(v:unknown)=>v===null||typeof v==='string'||typeof v==='boolean'||Array.isArray(v)&&v.every(x=>typeof x==='string');
const kinds=['text','multiline','number','money','date','datetime','single_select','multi_select','boolean','member','department'];
const references=['single_select','multi_select','member','department'];
function timestamp(v:unknown):boolean{
 if(typeof v!=='string'||!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(v)||!Number.isFinite(Date.parse(v)))return false;
 const day=new Date(v.slice(0,10)+'T00:00:00Z');return Number.isFinite(day.valueOf())&&day.toISOString().slice(0,10)===v.slice(0,10);
}
function change(v:unknown):v is HistoryChange{
 if(!object(v)||!exact(v,['fieldId','fieldKind','before','after','fieldLabel','fieldDeleted','valueLabels'])||!id(v.fieldId)||typeof v.fieldKind!=='string'||!kinds.includes(v.fieldKind)||!value(v.before)||!value(v.after)||typeof v.fieldLabel!=='string'||typeof v.fieldDeleted!=='boolean'||!object(v.valueLabels))return false;
 const permitted=new Set(references.includes(v.fieldKind)?[v.before,v.after].flatMap(x=>Array.isArray(x)?x:typeof x==='string'?[x]:[]):[]);
 return Object.entries(v.valueLabels).every(([key,label])=>id(key)&&permitted.has(key)&&object(label)&&exact(label,['label','deleted','labelUnavailable'])&&(typeof label.label==='string'||label.label===null&&label.labelUnavailable===true)&&typeof label.deleted==='boolean'&&typeof label.labelUnavailable==='boolean');
}
function event(v:unknown):v is HistoryEvent{
 return object(v)&&exact(v,['id','recordVersionBefore','recordVersionAfter','actorId','occurredAt','origin','changes'])&&id(v.id)&&id(v.actorId)&&version(v.recordVersionBefore,0)&&version(v.recordVersionAfter,1)&&v.recordVersionAfter>v.recordVersionBefore&&timestamp(v.occurredAt)&&(v.origin==='ordinary'||v.origin==='task_save')&&Array.isArray(v.changes)&&v.changes.length>0&&v.changes.every(change)&&new Set(v.changes.map(c=>c.fieldId)).size===v.changes.length;
}
export function validRecordHistory(data:unknown,pagination:unknown,pageSize:number):data is {items:HistoryEvent[]}{
 if(!Number.isInteger(pageSize)||pageSize<1||pageSize>100||!object(data)||!exact(data,['items'])||!Array.isArray(data.items)||data.items.length>pageSize||!data.items.every(event)||new Set(data.items.map(e=>e.id)).size!==data.items.length||!object(pagination)||!exact(pagination,['hasMore','nextPageToken'])||typeof pagination.hasMore!=='boolean')return false;
 return pagination.hasMore?typeof pagination.nextPageToken==='string'&&pagination.nextPageToken.length>0:pagination.nextPageToken===null;
}
export function recordHistoryValue(change:HistoryChange,side:'before'|'after'):string{
 const v=change[side];if(v===null)return '未填写';if(v==='')return '空字符串';if(typeof v==='boolean')return v?'是':'否';
 const label=(key:string)=>{if(!references.includes(change.fieldKind))return key;const entry=change.valueLabels[key];return (entry&&!entry.labelUnavailable&&entry.label!==null?entry.label:'名称不可用（'+key+'）')+(entry?.deleted?' · 已删除':'');};
 return Array.isArray(v)?v.length?v.map(label).join('、'):'未选择':label(v);
}
