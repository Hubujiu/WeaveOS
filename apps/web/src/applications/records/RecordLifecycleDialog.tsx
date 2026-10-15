import {useEffect,useRef} from 'react';
import {FormsDialog} from '../forms/FormsDialog';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {useApplicationOperation} from '../useApplicationOperation';
import type {RecordLifecycleResult} from './lifecycleContract';
import '../forms/forms.css';
export type RecordLifecycleSelection={id:string;recordVersion:number;schemaVersion:number;action:'deletion'|'restoration'};
type Props={actorId:string;appId:string;viewId:string;selection:RecordLifecycleSelection;onClose:()=>void;onConfirmed:(value:RecordLifecycleResult)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(value:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard};
export function RecordLifecycleDialog({actorId,appId,viewId,selection,onClose,onConfirmed,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard}:Props){
 const resource={kind:'record-lifecycle' as const,appId,viewId,id:selection.id,action:selection.action};
 const scope=JSON.stringify(['record-lifecycle',actorId,appId,viewId,selection.id]);
 const operation=useApplicationOperation<RecordLifecycleResult>({actorId,scope,resource,confirmed:result=>{if(result)onConfirmed(result);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const busy=operation.phase==='preflight'||operation.phase==='pending',unknown=operation.phase==='unconfirmed';
 const active=useRef(operation);active.current=operation;
 useEffect(()=>registerLeaveGuard({kind:'record-lifecycle',actorId,appId,viewId,recordId:selection.id},createLeaveController(()=>{const status=active.current.getPendingStatus()??'clean';return{status,fingerprint:JSON.stringify([status,active.current.packet?.operationId])};},decision=>{if(decision==='discard')active.current.cancelPreflight();})),[actorId,appId,viewId,selection.id,registerLeaveGuard]);
 useEffect(()=>{onDirtyChange(busy||unknown);return()=>onDirtyChange(false);},[busy,unknown,onDirtyChange]);
 const action=operation.packet?.resource?.kind==='record-lifecycle'?operation.packet.resource.action:selection.action;
 const deleting=action==='deletion';
 const submit=()=>{if(busy||unknown||operation.phase==='error'||operation.getPendingStatus())return;operation.start({resource,path:`applications/${appId}/forms/${viewId}/records/${selection.id}/${selection.action}`,method:'POST',expectedStatus:200,body:{expectedSchemaVersion:selection.schemaVersion,expectedRecordVersion:selection.recordVersion}});};
 return <FormsDialog title={deleting?'删除这条记录？':'恢复这条记录？'} onClose={onClose} busy={busy||unknown}>
  <div className="record-lifecycle-dialog">
   <p>{deleting?'记录将从正常列表隐藏，已有字段和审批历史保留。':'恢复后记录重新出现在正常列表，原字段和历史保留。'}</p>
   <p>{deleting?'可从「已删除记录」恢复；进行中或结果未确认的审批会阻止删除。':'恢复不会自动发起审批，记录版本将继续递增。'}</p>
   {operation.message&&<p role="alert">{operation.message}</p>}
   {busy&&<p role="status">正在核对记录操作…</p>}
   {unknown&&<div className="workflow-instance-buttons"><button type="button" className="admin-button" onClick={operation.query}>核查原操作</button><button type="button" className="admin-button" onClick={operation.retry}>重试原操作</button></div>}
   <div className="forms-dialog-actions"><button type="button" data-forms-close disabled={busy||unknown}>取消</button><button type="button" className="forms-primary" disabled={busy||unknown||operation.phase==='error'} onClick={submit}>{deleting?'确认删除':'确认恢复'}</button></div>
  </div>
 </FormsDialog>;
}
