export type WorkflowApprover={id:string;label:string;status:'active'};
export type WorkflowApproverPage={items:WorkflowApprover[];total:number;page:number;pageSize:number;queryVersion:string;schemaVersion:number;policyRevision:number};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
export function validWorkflowApproverPage(value:unknown,page:number,pageSize:number,schemaVersion:number,policyRevision:number):value is WorkflowApproverPage{
 if(!integer(page,1)||!integer(pageSize,1)||pageSize>100||!integer(schemaVersion,1)||!integer(policyRevision,1)||BigInt(page-1)*BigInt(pageSize)>BigInt(Number.MAX_SAFE_INTEGER))return false;
 if(!object(value)||!exact(value,['items','total','page','pageSize','queryVersion','schemaVersion','policyRevision'])||value.page!==page||value.pageSize!==pageSize||value.schemaVersion!==schemaVersion||value.policyRevision!==policyRevision||!integer(value.total,0)||typeof value.queryVersion!=='string'||!value.queryVersion.length||value.queryVersion.length>256)return false;
 if(!Array.isArray(value.items)||value.items.length>pageSize||value.items.length&&BigInt(value.items.length)>BigInt(value.total)-BigInt(page-1)*BigInt(pageSize))return false;
 return value.items.every(v=>object(v)&&exact(v,['id','label','status'])&&uuid(v.id)&&v.status==='active'&&typeof v.label==='string'&&v.label.trim().length>0&&[...v.label].length<=254)&&new Set(value.items.map(v=>v.id)).size===value.items.length;
}
