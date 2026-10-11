export type RecordWorkflow={id:string;flowId:string;name:string;initiatorId:string;state:'starting'|'active'|'completed'|'rejected'|'withdrawn'|'no_effect';createdAt:string;updatedAt:string;definitionVersion:number;sequence:number};
export type RecordWorkflowPage={items:RecordWorkflow[];total:number;page:number;pageSize:number;queryVersion:string};
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
function timestamp(v:unknown):boolean{
 if(typeof v!=='string'||!Number.isFinite(Date.parse(v)))return false;
 const m=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.exec(v);
 if(!m)return false;const [year,month,day,hour,minute,second]=m.slice(1,7).map(Number),leap=year%4===0&&(year%100!==0||year%400===0);
 return month>=1&&month<=12&&day>=1&&day<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1]&&hour<24&&minute<60&&second<60;
}
function row(v:unknown):v is RecordWorkflow{
 return object(v)&&exact(v,['id','flowId','name','initiatorId','state','createdAt','updatedAt','definitionVersion','sequence'])&&['id','flowId','initiatorId'].every(k=>id(v[k]))&&typeof v.name==='string'&&['starting','active','completed','rejected','withdrawn','no_effect'].includes(String(v.state))&&timestamp(v.createdAt)&&timestamp(v.updatedAt)&&integer(v.definitionVersion,1)&&integer(v.sequence,0);
}
export function validRecordWorkflows(value:unknown,page:number,size:number):value is RecordWorkflowPage{
 if(!integer(page,1)||!integer(size,1)||size>100||!object(value)||!exact(value,['items','total','page','pageSize','queryVersion'])||value.page!==page||value.pageSize!==size||!integer(value.total,0)||typeof value.queryVersion!=='string'||value.queryVersion.length<1||value.queryVersion.length>256||!Array.isArray(value.items)||value.items.length>size)return false;
 const offset=BigInt(page-1)*BigInt(size);
 return !(value.items.length&&BigInt(value.items.length)>BigInt(value.total)-offset)&&value.items.every(row)&&new Set(value.items.map(v=>v.id)).size===value.items.length;
}
