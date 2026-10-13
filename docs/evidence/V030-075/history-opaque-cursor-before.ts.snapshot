export type WorkflowEvent={id:string;instanceId:string;flowId:string;flowName:string;flowNameSource:'captured'|'legacy_last_known';nodeId:string|null;targetNodeId:string|null;actorId:string;action:'start'|'agree'|'reject'|'withdraw'|'return';outcome:'success'|'no_effect';sequence:number;schemaVersion:number;recordVersion:number;occurredAt:string};
export type HistoricalField={fieldId:string;fieldName:string;fieldKind:string;value:string|boolean|string[]|null;valueLabels:Record<string,{label:string;deleted:boolean}>};
export type WorkflowEventDetail={event:WorkflowEvent;basis:{status:'available'|'unavailable';fields:HistoricalField[]}};
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number)=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
function date(v:unknown):boolean{
 if(typeof v!=='string')return false;const m=/^(\d{4})-(\d{2})-(\d{2})$/.exec(v);if(!m)return false;
 const y=Number(m[1]),month=Number(m[2]),d=Number(m[3]),leap=y%4===0&&(y%100!==0||y%400===0);
 return month>=1&&month<=12&&d>=1&&d<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1];
}
function timestamp(v:unknown,utc=false):boolean{
 if(typeof v!=='string'||utc&&!v.endsWith('Z'))return false;
 const m=/^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.exec(v);
 return !!m&&date(m[1])&&Number(m[2])<24&&Number(m[3])<60&&Number(m[4])<60&&Number.isFinite(Date.parse(v));
}
const eventKeys=['id','instanceId','flowId','flowName','flowNameSource','nodeId','targetNodeId','actorId','action','outcome','sequence','schemaVersion','recordVersion','occurredAt'];
function event(v:unknown):v is WorkflowEvent{
 return object(v)&&exact(v,eventKeys)&&['id','instanceId','flowId','actorId'].every(k=>id(v[k]))&&(v.nodeId===null||id(v.nodeId))&&(v.targetNodeId===null||id(v.targetNodeId))&&(v.action==='return'||v.targetNodeId===null)&&typeof v.flowName==='string'&&v.flowName.length>0&&Array.from(v.flowName).length<=100&&['captured','legacy_last_known'].includes(String(v.flowNameSource))&&['start','agree','reject','withdraw','return'].includes(String(v.action))&&['success','no_effect'].includes(String(v.outcome))&&integer(v.sequence,0)&&integer(v.schemaVersion,1)&&integer(v.recordVersion,1)&&timestamp(v.occurredAt,true);
}
export function validWorkflowHistory(data:unknown,pagination:unknown,pageSize:number):data is {items:WorkflowEvent[]}{
 if(!integer(pageSize,1)||pageSize>100||!object(data)||!exact(data,['items'])||!Array.isArray(data.items)||data.items.length>pageSize||!data.items.every(event)||new Set(data.items.map(e=>e.id)).size!==data.items.length||!object(pagination)||!exact(pagination,['hasMore','nextPageToken'])||typeof pagination.hasMore!=='boolean')return false;
 return pagination.hasMore?typeof pagination.nextPageToken==='string'&&/^[A-Za-z0-9_-]{43}$/.test(pagination.nextPageToken):pagination.nextPageToken===null;
}
function field(v:unknown):v is HistoricalField{
 if(!object(v)||!exact(v,['fieldId','fieldName','fieldKind','value','valueLabels'])||!id(v.fieldId)||typeof v.fieldName!=='string'||typeof v.fieldKind!=='string'||!object(v.valueLabels))return false;
 const kinds=['text','multiline','number','money','boolean','date','datetime','single_select','multi_select','member','department'];
 if(!kinds.includes(v.fieldKind))return false;
 const val=v.value,reference=['member','department','single_select','multi_select'].includes(v.fieldKind);
 if(val!==null){
  if(v.fieldKind==='boolean'){if(typeof val!=='boolean')return false;}
  else if(v.fieldKind==='multi_select'){if(!Array.isArray(val)||!val.every(id)||new Set(val).size!==val.length)return false;}
  else if(reference){if(!id(val))return false;}
  else if(typeof val!=='string')return false;
  else if(v.fieldKind==='money'||v.fieldKind==='number'){if(!/^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(val))return false;}
  else if(v.fieldKind==='date'&&!date(val)||v.fieldKind==='datetime'&&!timestamp(val))return false;
 }
 const ids=new Set(reference?(Array.isArray(val)?val:typeof val==='string'?[val]:[]):[]);
 return Object.entries(v.valueLabels).every(([key,label])=>id(key)&&ids.has(key)&&object(label)&&exact(label,['label','deleted'])&&typeof label.label==='string'&&typeof label.deleted==='boolean');
}
export function validWorkflowEventDetail(data:unknown,expected:WorkflowEvent):data is WorkflowEventDetail{
 if(!object(data)||!exact(data,['event','basis'])||!event(data.event)||!event(expected)||!object(data.basis)||!exact(data.basis,['status','fields']))return false;
 const actual=data.event;
 if(!eventKeys.every(k=>actual[k as keyof WorkflowEvent]===expected[k as keyof WorkflowEvent]))return false;
 const basis=data.basis;
 if(!Array.isArray(basis.fields)||basis.fields.length>1600||!basis.fields.every(field)||new Set(basis.fields.map(f=>f.fieldId)).size!==basis.fields.length)return false;
 return basis.status==='available'||basis.status==='unavailable'&&basis.fields.length===0;
}

// Display only the authorized original projection, without numeric conversion
// or an extra directory lookup that could replace historical names.
export function historyValue(field:HistoricalField):string{
 const value=field.value;
 if(value===null)return '—';
 if(typeof value==='boolean')return value?'是':'否';
 if(['member','department','single_select','multi_select'].includes(field.fieldKind)){
  const ids=Array.isArray(value)?value:[value];
  return ids.map(id=>{const label=field.valueLabels[id];return label?label.label+(label.deleted?'（已删除）':''):'引用信息不可用';}).join('、');
 }
 return typeof value==='string'?value:'';
}
export function historyComparison(before:HistoricalField,current?:HistoricalField):{before:string;after:string;comparable:boolean}{
 const comparable=!!current&&current.fieldId===before.fieldId&&current.fieldKind===before.fieldKind;
 return {before:historyValue(before),after:comparable?historyValue(current!):'当前不可比较',comparable};
}
