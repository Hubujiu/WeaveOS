export type StructureCopyKind='application'|'directory'|'table'|'form';
export type StructureCopyReceipt={operationId:string;appId:string;resourceKind:StructureCopyKind;id:string;structureVersion:number;policyRevision:number;status:'copied'};
export type CopyReceiptContext={operationId:string;source:{appId:string;kind:StructureCopyKind;id:string};targetAppId:string|null;expectedStructureVersion:number;expectedPolicyRevision:number;newGroupCount:number};
export function validStructureCopyReceipt(_value:unknown,_context:CopyReceiptContext):_value is StructureCopyReceipt{return false;}
