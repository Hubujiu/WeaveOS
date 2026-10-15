import {useEffect,useRef,useState} from 'react';
import {FormsDialog} from '../forms/FormsDialog';
import '../forms/forms.css';
import {Modal} from '../../Modal';
import {applicationApi,ApplicationError} from '../api';
import {useApplicationOperation} from '../useApplicationOperation';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {validLifecyclePreview,type InstanceLocation,type LifecyclePreview} from './lifecycleContract';
import type {WorkflowOperation} from './taskActionContract';

type Props={actorId:string;location:InstanceLocation;name:string;onClose:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void};
export function WorkflowInstanceActions({actorId,location,name,onClose,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const scope=JSON.stringify(['workflow-instance',actorId,location.appId,location.viewId,location.recordId,location.instanceId]);
 const path=`applications/${location.appId}/forms/${location.viewId}/records/${location.recordId}/workflow-instances/${location.instanceId}`;
 const resource={kind:'instance-action' as const,appId:location.appId,viewId:location.viewId,id:location.recordId,instanceId:location.instanceId};
 const [epoch,setEpoch]=useState(0),[preview,setPreview]=useState<LifecyclePreview|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState(''),[confirming,setConfirming]=useState(false),[result,setResult]=useState<WorkflowOperation|null>(null);
 const [returning,setReturning]=useState(false),[target,setTarget]=useState(''),[reason,setReason]=useState('');
 const handlers=useRef({onUnauthorized,onIdentityMismatch});handlers.current={onUnauthorized,onIdentityMismatch};
 const operation=useApplicationOperation<WorkflowOperation>({actorId,scope,resource,confirmed:value=>{if(value)setResult(value);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const busy=operation.phase==='preflight'||operation.phase==='pending',unknown=operation.phase==='unconfirmed',accepted=operation.phase==='accepted';
 const blocked=busy||unknown||accepted||!!result;
 const stale=['WORKFLOW_BASIS_EXPIRED','WORKFLOW_BASIS_CHANGED','WORKFLOW_TASK_CHANGED','APPLICATION_POLICY_CONFLICT','APPLICATION_SCHEMA_NOT_READY'].includes(operation.errorCode??'');
 const revoked=['APPLICATION_FORBIDDEN','APPLICATION_NOT_FOUND'].includes(operation.errorCode??'');
 useEffect(()=>{if(revoked)setPreview(null);},[revoked]);
 const latest=useRef(operation);latest.current=operation;
 useEffect(()=>registerLeaveGuard({kind:'workflow-instance',actorId,...location},createLeaveController(()=>{
  const pending=latest.current.getPendingStatus();return {status:pending??'clean',fingerprint:JSON.stringify([pending,latest.current.packet?.operationId])};
 },decision=>{if(decision==='discard')latest.current.cancelPreflight();})),[scope,registerLeaveGuard]);
 useEffect(()=>{onDirtyChange(busy||unknown||accepted);return()=>onDirtyChange(false);},[busy,unknown,accepted,onDirtyChange]);
 useEffect(()=>{
  const controller=new AbortController();let current=true;setLoading(true);setPreview(null);setError('');
  void (async()=>{
   await Promise.resolve();if(controller.signal.aborted)return;
   try{
    const raw=await applicationApi<unknown>(actorId,path+'/lifecycle','GET',undefined,controller.signal,200);
    if(!current||controller.signal.aborted)return;
    if(!validLifecyclePreview(raw,location))throw new Error('流程操作资格响应格式不正确');
    setPreview(raw);
   }catch(cause){
    if(!current||controller.signal.aborted)return;
    setError(cause instanceof ApplicationError&&cause.code==='WORKFLOW_TASK_CHANGED'?'流程已变化或结束，请重新查看当前状态。':cause instanceof Error?cause.message:'无法读取流程操作资格');
    if(cause instanceof ApplicationError&&cause.status===401)handlers.current.onUnauthorized();else if(cause instanceof ApplicationError&&cause.code==='AUTH_SESSION_CHANGED')handlers.current.onIdentityMismatch();
   }finally{if(current&&!controller.signal.aborted)setLoading(false);}
  })();return()=>{current=false;controller.abort();};
 },[actorId,path,epoch]);
 const refresh=()=>{if(blocked)return;operation.dismissError();setConfirming(false);setPreview(null);setError('');setLoading(true);setEpoch(value=>value+1);};
 const withdraw=()=>{if(!preview?.canWithdraw||blocked||stale||revoked)return;operation.start({resource,path:path+'/actions',method:'POST',expectedStatus:202,body:{action:'withdraw',basisToken:preview.basisToken}});};
 const validReason=Array.from(reason).length>=1&&Array.from(reason).length<=500&&!/^[\p{White_Space}]*$/u.test(reason)&&!/[\u0000\uD800-\uDFFF]/u.test(reason);
 const returnToNode=()=>{if(!preview||!preview.returnTargets.some(t=>t.nodeId===target)||!validReason||blocked||stale||revoked||operation.getPendingStatus())return;operation.start({resource,path:path+'/actions',method:'POST',expectedStatus:202,body:{action:'return',basisToken:preview.basisToken,targetNodeId:target,reason}});};
 if(returning)return <FormsDialog title="退回到节点" onClose={()=>{if(blocked)requestSectionLeave(onClose);else {setReturning(false);setTarget('');setReason('');}}} busy={busy}>
  <div className="workflow-return-dialog">
   {preview&&!revoked&&!result&&<><label>目标节点<select value={target} disabled={blocked||stale} onChange={event=>setTarget(event.target.value)}><option value="">请选择目标节点</option>{preview.returnTargets.map(t=><option key={t.nodeId} value={t.nodeId}>节点 {t.nodeId}</option>)}</select></label>
    <label>退回原因（必填，最多500字）<input required value={reason} disabled={blocked||stale} onChange={event=>setReason(event.target.value)}/></label><p className="forms-muted">仅显示本轮已走过的节点</p></>}
   {stale&&<p role="alert">操作依据已变化或过期，请刷新操作资格并重新核对。</p>}
   {operation.phase==='error'&&!stale&&<p role="alert">{operation.message}</p>}
   {busy&&<p role="status">正在核对退回操作…</p>}
   {accepted&&<p role="status">退回处理中</p>}
   {unknown&&<p role="alert">退回结果尚未确认，请核查原操作或重试原退回。{operation.message}</p>}
   {(accepted||unknown)&&<div className="workflow-instance-buttons"><button type="button" className="admin-button" disabled={busy} onClick={operation.query}>核查退回结果</button><button type="button" className="admin-button" disabled={busy} onClick={operation.retry}>重试原退回</button></div>}
   {result&&<p role="status">{result.status==='success'?'已退回':'退回未执行'}</p>}
   {!blocked&&(stale||revoked)&&<button type="button" className="admin-button" onClick={()=>{setReturning(false);setTarget('');setReason('');refresh();}}>刷新操作资格</button>}
   {!result&&<div className="forms-dialog-actions"><button type="button" data-forms-close disabled={blocked}>取消</button><button type="button" className="forms-primary" disabled={!preview||!preview.returnTargets.some(t=>t.nodeId===target)||!validReason||blocked||stale||revoked} onClick={returnToNode}>确认退回</button></div>}
  </div>
 </FormsDialog>;
 return <Modal title={confirming?'撤回'+name:'流程操作：'+name} onClose={()=>requestSectionLeave(onClose)} busy={busy}>
  <div className="workflow-instance-controls">
   {loading&&<p role="status">正在读取当前操作资格…</p>}
   {error&&<p role="alert">{error}</p>}
   {preview&&!confirming&&!result&&<><p>当前记录版本 v{preview.record.recordVersion}</p>{preview.canWithdraw?<button type="button" className="admin-button" disabled={blocked||stale} onClick={()=>setConfirming(true)}>撤回审批</button>:<p>当前不可撤回</p>}{preview.returnTargets.length>0&&<button type="button" className="admin-button" disabled={blocked||stale} onClick={()=>{setTarget('');setReason('');setReturning(true);}}>退回节点</button>}</>}
   {confirming&&!result&&<><h3>撤回当前{name}？</h3><p>记录及已保存修改将保留</p></>}
   {stale&&<p role="alert">操作依据已变化或过期，请刷新操作资格并重新核对。</p>}
   {operation.phase==='error'&&!stale&&<p role="alert">{operation.message}</p>}
   {busy&&<p role="status">正在核对撤回操作…</p>}
   {accepted&&<p role="status">撤回处理中</p>}
   {unknown&&<p role="alert">撤回结果尚未确认，请核查原操作或重试原撤回。{operation.message}</p>}
   {(accepted||unknown)&&<div className="workflow-instance-buttons"><button type="button" className="admin-button" disabled={busy} onClick={operation.query}>核查撤回结果</button><button type="button" className="admin-button" disabled={busy} onClick={operation.retry}>重试原撤回</button></div>}
   {result&&<p role="status">{result.status==='success'?'已撤回':'撤回未执行'}</p>}
   {!blocked&&(error||stale||revoked)&&<button type="button" className="admin-button" onClick={refresh}>刷新操作资格</button>}
   {confirming&&!result&&<div className="workflow-instance-confirm"><button type="button" className="admin-button" disabled={busy||unknown||accepted} onClick={()=>setConfirming(false)}>取消</button><button type="button" className="admin-button primary" disabled={!preview?.canWithdraw||blocked||stale||revoked} onClick={withdraw}>确认撤回</button></div>}
  </div>
 </Modal>;
}
