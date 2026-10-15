export type DeletedRecordItem={id:string;recordVersion:number;deletedAt:string};
export type DeletedRecordPage={items:DeletedRecordItem[];total:number;page:number;pageSize:number;queryVersion:string;schemaVersion:number};
const object=(value:unknown):value is Record<string,unknown>=>!!value&&typeof value==='object'&&!Array.isArray(value);
const exact=(value:Record<string,unknown>,keys:string[])=>Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const integer=(value:unknown,min:number):value is number=>typeof value==='number'&&Number.isSafeInteger(value)&&value>=min;
const uuid=(value:unknown):value is string=>typeof value==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
function utcTime(value:unknown):boolean{
 if(typeof value!=='string')return false;
 const m=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?Z$/.exec(value);
 if(!m||!Number.isFinite(Date.parse(value)))return false;
 const [year,month,day,hour,minute,second]=m.slice(1,7).map(Number),leap=year%4===0&&(year%100!==0||year%400===0);
 return month>=1&&month<=12&&day>=1&&day<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1]&&hour<24&&minute<60&&second<60;
}
function item(value:unknown):value is DeletedRecordItem{
 return object(value)&&exact(value,['id','recordVersion','deletedAt'])&&uuid(value.id)&&integer(value.recordVersion,1)&&utcTime(value.deletedAt);
}
export function validDeletedRecordPage(value:unknown,page:number,pageSize:number):value is DeletedRecordPage{
 if(!integer(page,1)||!integer(pageSize,1)||pageSize>100||!object(value)||!exact(value,['items','total','page','pageSize','queryVersion','schemaVersion']))return false;
 const offset=BigInt(page-1)*BigInt(pageSize);if(offset>BigInt(Number.MAX_SAFE_INTEGER))return false;
 if(value.page!==page||value.pageSize!==pageSize||!integer(value.total,0)||!integer(value.schemaVersion,1)||typeof value.queryVersion!=='string'||value.queryVersion.length===0||[...value.queryVersion].length>256||!Array.isArray(value.items))return false;
 const remaining=BigInt(value.total)-offset,expected=remaining<=0n?0:remaining>BigInt(pageSize)?pageSize:Number(remaining);
 return value.items.length===expected&&value.items.every(item)&&new Set(value.items.map(row=>row.id)).size===value.items.length;
}
