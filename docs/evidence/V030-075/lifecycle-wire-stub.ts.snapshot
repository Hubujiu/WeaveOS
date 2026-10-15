import type {Field} from '../forms/contracts';
import type {RecordItem} from '../records/contracts';
export type InstanceLocation={appId:string;viewId:string;recordId:string;instanceId:string};
export type LifecyclePreview={basisToken:string;instance:{id:string;sequence:number;state:'active'};record:RecordItem;fields:Field[];canWithdraw:boolean;returnTargets:{nodeId:string;taskId:string;activationEpoch:number}[]};
export function validLifecyclePreview(_value:unknown,_location:InstanceLocation):_value is LifecyclePreview{return false;}
