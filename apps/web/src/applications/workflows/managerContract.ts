export type WorkflowManagerItem={id:string;name:string;revision:number;currentVersion:number;candidateVersion:number;state:'disabled'|'enabled'|'closing';triggerEvents:('manual'|'record.created'|'record.updated')[];inFlightCount:number;updatedAt:string};
export type WorkflowManagerPage={items:WorkflowManagerItem[];total:number;page:number;pageSize:number;queryVersion:string};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
function timestamp(v:unknown):boolean{
 if(typeof v!=='string')return false;
 const m=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.exec(v);
 if(!m||!Number.isFinite(Date.parse(v)))return false;
 const [year,month,day,hour,minute,second]=m.slice(1).map(Number),leap=year%4===0&&(year%100!==0||year%400===0);
 return month>=1&&month<=12&&day>=1&&day<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1]&&hour<24&&minute<60&&second<60;
}
function item(v:unknown):v is WorkflowManagerItem{
 if(!object(v)||!exact(v,['id','name','revision','currentVersion','candidateVersion','state','triggerEvents','inFlightCount','updatedAt']))return false;
 return uuid(v.id)&&typeof v.name==='string'&&v.name.trim().length>0&&[...v.name].length<=100&&integer(v.revision,1)&&integer(v.currentVersion,0)&&integer(v.candidateVersion,1)&&v.candidateVersion>=v.currentVersion&&['disabled','enabled','closing'].includes(v.state as string)&&integer(v.inFlightCount,0)&&timestamp(v.updatedAt)&&Array.isArray(v.triggerEvents)&&v.triggerEvents.length<=3&&new Set(v.triggerEvents).size===v.triggerEvents.length&&v.triggerEvents.every(e=>['manual','record.created','record.updated'].includes(e))&&(v.currentVersion!==0||v.triggerEvents.length===0);
}
export function validWorkflowManagerPage(value:unknown,page:number,pageSize:number):value is WorkflowManagerPage{
 if(!integer(page,1)||!integer(pageSize,1)||pageSize>100||BigInt(page-1)*BigInt(pageSize)>BigInt(Number.MAX_SAFE_INTEGER)||!object(value)||!exact(value,['items','total','page','pageSize','queryVersion']))return false;
 if(value.page!==page||value.pageSize!==pageSize||!integer(value.total,0)||typeof value.queryVersion!=='string'||!value.queryVersion.length||value.queryVersion.length>256||!Array.isArray(value.items)||value.items.length>pageSize)return false;
 const offset=BigInt(page-1)*BigInt(pageSize);
 if(value.items.length&&BigInt(value.items.length)>BigInt(value.total)-offset)return false;
 return value.items.every(item)&&new Set(value.items.map(v=>v.id)).size===value.items.length;
}
