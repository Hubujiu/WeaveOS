// V061 wire and V075 consumption contract. The server is the permission authority.
export type InboxItem={id:string;appId:string;viewId:string;recordId:string;instanceId:string;flowId:string;nodeId:string;flowName:string;createdAt:string;definitionVersion:number;activationEpoch:number;sequence:number};
export type InboxPage={items:InboxItem[];total:number;page:number;pageSize:number;queryVersion:string};
const ids=['id','appId','viewId','recordId','instanceId','flowId','nodeId'] as const;
const uuid=/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/;
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
function timestamp(v:unknown):v is string{
 if(typeof v!=='string')return false;
 const m=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(Z|[+-]\d{2}:\d{2})$/.exec(v);
 if(!m||!Number.isFinite(Date.parse(v)))return false;
 const [year,month,day,hour,minute,second]=m.slice(1,7).map(Number);
 const leap=year%4===0&&(year%100!==0||year%400===0);
 const days=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31];
 return month>=1&&month<=12&&day>=1&&day<=days[month-1]&&hour<24&&minute<60&&second<60;
}
function validItem(v:unknown):v is InboxItem{
 return object(v)&&exact(v,[...ids,'flowName','createdAt','definitionVersion','activationEpoch','sequence'])&&
  ids.every(k=>typeof v[k]==='string'&&uuid.test(v[k]))&&typeof v.flowName==='string'&&timestamp(v.createdAt)&&
  integer(v.definitionVersion,1)&&integer(v.activationEpoch,1)&&integer(v.sequence,0);
}
export function validInboxPage(v:unknown,page:number,pageSize:number):v is InboxPage {
 if(!integer(page,1)||!integer(pageSize,1)||pageSize>100||!object(v)||!exact(v,['items','total','page','pageSize','queryVersion']))return false;
 if(v.page!==page||v.pageSize!==pageSize||!integer(v.total,0)||typeof v.queryVersion!=='string'||v.queryVersion.length<1||v.queryVersion.length>256||!Array.isArray(v.items)||v.items.length>pageSize)return false;
 // Avoid unsafe page-offset multiplication. Beyond the last page may be empty.
 const offset=BigInt(page-1)*BigInt(pageSize);
 if(BigInt(v.items.length)>BigInt(v.total)-offset&&v.items.length!==0)return false;
 return v.items.every(validItem)&&new Set(v.items.map(item=>item.id)).size===v.items.length;
}
