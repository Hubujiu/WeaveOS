import {validRecordProjection} from './taskPreviewContract';
import type {Field} from '../forms/contracts';
import type {RecordItem} from '../records/contracts';
export type InstanceLocation={appId:string;viewId:string;recordId:string;instanceId:string};
export type LifecyclePreview={basisToken:string;instance:{id:string;sequence:number;state:'active'};record:RecordItem;fields:Field[];canWithdraw:boolean;returnTargets:{nodeId:string;taskId:string;activationEpoch:number}[]};
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(key=>Object.hasOwn(v,key));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
export function validLifecyclePreview(v:unknown,location:InstanceLocation):v is LifecyclePreview{
 if(!Object.values(location).every(id)||!object(v)||!exact(v,['basisToken','instance','record','fields','canWithdraw','returnTargets'])||typeof v.basisToken!=='string'||!v.basisToken.length||v.basisToken.length>256||typeof v.canWithdraw!=='boolean')return false;
 const i=v.instance;
 if(!object(i)||!exact(i,['id','sequence','state'])||i.id!==location.instanceId||i.state!=='active'||!integer(i.sequence,0)||!validRecordProjection(v.record,v.fields,{appId:location.appId,viewId:location.viewId,recordId:location.recordId},200))return false;
 return Array.isArray(v.returnTargets)&&v.returnTargets.length<=100&&v.returnTargets.every(t=>object(t)&&exact(t,['nodeId','taskId','activationEpoch'])&&id(t.nodeId)&&id(t.taskId)&&integer(t.activationEpoch,1))&&new Set(v.returnTargets.map(t=>t.nodeId)).size===v.returnTargets.length;
}
