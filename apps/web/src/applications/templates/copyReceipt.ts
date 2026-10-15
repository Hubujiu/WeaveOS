export type StructureCopyKind='application'|'directory'|'table'|'form';
export type StructureCopyReceipt={operationId:string;appId:string;resourceKind:StructureCopyKind;id:string;structureVersion:number;policyRevision:number;status:'copied'};
export type CopyReceiptContext={operationId:string;source:{appId:string;kind:StructureCopyKind;id:string};targetAppId:string|null;expectedStructureVersion:number;expectedPolicyRevision:number;newGroupCount:number};
const object=(value:unknown):value is Record<string,unknown>=>!!value&&typeof value==='object'&&!Array.isArray(value);
const uuid=(value:unknown):value is string=>typeof value==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
const integer=(value:unknown,min:number):value is number=>typeof value==='number'&&Number.isSafeInteger(value)&&value>=min;
const kinds:readonly string[]=['application','directory','table','form'];
const keys=['operationId','appId','resourceKind','id','structureVersion','policyRevision','status'];
export function validStructureCopyReceipt(value:unknown,context:CopyReceiptContext):value is StructureCopyReceipt{
 if(!object(context)||!object(context.source)||!uuid(context.operationId)||!uuid(context.source.appId)||!uuid(context.source.id)||!kinds.includes(context.source.kind)||!integer(context.newGroupCount,0)||context.newGroupCount>128)return false;
 if(!object(value)||Object.keys(value).length!==keys.length||!keys.every(key=>Object.hasOwn(value,key))||value.operationId!==context.operationId||value.resourceKind!==context.source.kind||value.status!=='copied'||!uuid(value.appId)||!uuid(value.id)||value.id===context.source.id||!integer(value.structureVersion,1)||!integer(value.policyRevision,1))return false;
 if(context.source.kind==='application')return context.source.id===context.source.appId&&context.targetAppId===null&&context.expectedStructureVersion===0&&context.expectedPolicyRevision===0&&value.appId!==context.source.appId&&value.id===value.appId&&value.structureVersion===1&&value.policyRevision===1;
 if(!uuid(context.targetAppId)||value.appId!==context.targetAppId||!integer(context.expectedStructureVersion,0)||!integer(context.expectedPolicyRevision,1))return false;
 const structure=context.expectedStructureVersion+1,policy=context.expectedPolicyRevision+(context.newGroupCount>0?1:0);
 return Number.isSafeInteger(structure)&&Number.isSafeInteger(policy)&&value.structureVersion===structure&&value.policyRevision===policy;
}
