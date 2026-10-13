import {validRecordWorkflows,type RecordWorkflow} from './recordWorkflowsContract';
export type WorkflowStartedItem={appId:string;viewId:string;recordId:string;instance:RecordWorkflow};
export type WorkflowStartedPage={items:WorkflowStartedItem[];total:number;page:number;pageSize:number;queryVersion:string};
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
export function validWorkflowStartedPage(value:unknown,page:number,pageSize:number,actorId:string):value is WorkflowStartedPage{
 if(!id(actorId)||!integer(page,1)||!integer(pageSize,1)||pageSize>100||!object(value)||!exact(value,['items','total','page','pageSize','queryVersion'])||value.page!==page||value.pageSize!==pageSize||!integer(value.total,0)||typeof value.queryVersion!=='string'||value.queryVersion.length<1||value.queryVersion.length>256||!Array.isArray(value.items))return false;
 const offset=BigInt(page-1)*BigInt(pageSize);if(offset>BigInt(Number.MAX_SAFE_INTEGER))return false;
 const remaining=BigInt(value.total)-offset,expected=remaining<0n?0n:remaining<BigInt(pageSize)?remaining:BigInt(pageSize);
 if(BigInt(value.items.length)!==expected)return false;
 if(!value.items.every(item=>object(item)&&exact(item,['appId','viewId','recordId','instance'])&&id(item.appId)&&id(item.viewId)&&id(item.recordId)))return false;
 const instances=value.items.map(item=>item.instance);
 if(!validRecordWorkflows({...value,items:instances},page,pageSize))return false;
 return instances.every((instance:RecordWorkflow)=>instance.initiatorId===actorId);
}
