import type {StructureCopyKind} from './copyReceipt';
export type StructureCopyPlanRequest={source:{appId:string;kind:StructureCopyKind;id:string};target:{appId:string|null;directoryId:string|null;name:string}};
export type StructureCopyPreflightRequest=StructureCopyPlanRequest&{expectedSourceFingerprint:string;expectedTargetStructureVersion:number;expectedTargetPolicyRevision:number;bindings:{kind:'user'|'department';sourceId:string;targetId:string}[];groupMappings:{sourceGroupId:string;name:string}[]};
export type StructureCopyResource={kind:'structure-copy';sourceAppId:string;sourceKind:StructureCopyKind;sourceId:string};
export function validStructureCopyPlanRequest(_value:unknown):_value is StructureCopyPlanRequest{return false;}
export function validStructureCopyPreflightRequest(_value:unknown):_value is StructureCopyPreflightRequest{return false;}
export function validStructureCopyRequest(_resource:unknown,_request:unknown):boolean{return false;}
