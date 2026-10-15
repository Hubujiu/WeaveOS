export type ManualStartOption={flowId:string;name:string;workflowRevision:number;definitionVersion:number;schemaVersion:number;recordVersion:number};
export type ManualStartPage={items:ManualStartOption[];total:number;page:number;pageSize:number;queryVersion:string};
export type ManualStartReceipt={operationId:string;flowId:string;instanceId:string;status:'accepted'|'ignored';ignored:boolean};
const object=(value:unknown):value is Record<string,unknown>=>!!value&&typeof value==='object'&&!Array.isArray(value);
const exact=(value:Record<string,unknown>,keys:string[])=>Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const id=(value:unknown):value is string=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
const positive=(value:unknown):value is number=>typeof value==='number'&&Number.isSafeInteger(value)&&value>0;
export function validManualStartPage(value:unknown,page:number,pageSize:number):value is ManualStartPage{
 if(!positive(page)||!positive(pageSize)||pageSize>100||!object(value)||!exact(value,['items','total','page','pageSize','queryVersion'])||value.page!==page||value.pageSize!==pageSize||typeof value.total!=='number'||!Number.isSafeInteger(value.total)||value.total<0||typeof value.queryVersion!=='string'||!value.queryVersion.length||value.queryVersion.length>256||!Array.isArray(value.items)||value.items.length>pageSize||value.items.length>value.total)return false;
 const seen=new Set<string>();
 return value.items.every(item=>{
  if(!object(item)||!exact(item,['flowId','name','workflowRevision','definitionVersion','schemaVersion','recordVersion'])||!id(item.flowId)||seen.has(item.flowId)||typeof item.name!=='string'||!positive(item.workflowRevision)||!positive(item.definitionVersion)||!positive(item.schemaVersion)||!positive(item.recordVersion))return false;
  seen.add(item.flowId);return true;
 });
}
export function validManualStartReceipt(value:unknown,operationId:string,flowId:string):value is ManualStartReceipt{
 return id(operationId)&&id(flowId)&&object(value)&&exact(value,['operationId','flowId','instanceId','status','ignored'])&&value.operationId===operationId&&value.flowId===flowId&&id(value.instanceId)&&((value.status==='accepted'&&value.ignored===false)||(value.status==='ignored'&&value.ignored===true));
}
// V044 pre-operation packet declaration; behavior follows independent RED.
export function validManualStartRequest(_resource:unknown,_request:unknown):boolean{return false;}
