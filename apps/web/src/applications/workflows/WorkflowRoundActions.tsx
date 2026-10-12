import {useEffect,useLayoutEffect,useRef,useState,type MutableRefObject} from 'react';
import {findRoundRecovery} from './roundRecovery';
import {Modal} from '../../Modal';
import {applicationApi,ApplicationError} from '../api';
import {useApplicationOperation,type ApplicationMutationRequest} from '../useApplicationOperation';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {validRoundPreview,type RoundPreview,type RoundStartReceipt} from './roundContract';
import type {InstanceLocation} from './lifecycleContract';

type Props={actorId:string;location:InstanceLocation;flowId:string;name:string;onClose:()=>void;onRework?:(instanceId:string)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void};
type Kind='resubmit'|'review';
type Job={key:string;request:ApplicationMutationRequest;recovered?:boolean};
type Operation=ReturnType<typeof useApplicationOperation<RoundStartReceipt>>;
export function WorkflowRoundActions(props:Props){return <RoundScope key={JSON.stringify([props.actorId,props.location])} {...props}/>;}
function RoundScope({actorId,location,flowId,name,onClose,onRework,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const scope=JSON.stringify(['workflow-round',actorId,location.appId,location.viewId,location.recordId,location.instanceId]);
 const path=`applications/${location.appId}/forms/${location.viewId}/records/${location.recordId}/workflows/${location.instanceId}/round-actions`;
 const [preview,setPreview]=useState<RoundPreview|null>(null),[loading,setLoading]=useState(true),[epoch,setEpoch]=useState(0),[error,setError]=useState(''),[stale,setStale]=useState(false);
 const [kind,setKind]=useState<Kind|null>(null),[job,setJob]=useState<Job|null>(()=>{const packet=findRoundRecovery(actorId,location.appId,location.viewId,location.recordId,location.instanceId,'round-start');return packet?.resource?{key:packet.operationId!,recovered:true,request:{resource:packet.resource,path:packet.path,method:packet.method,expectedStatus:packet.expectedStatus,body:packet.body}}:null;}),[busy,setBusy]=useState(false),[result,setResult]=useState<RoundStartReceipt|null>(null);
 const pending=useRef<Job|null>(job),operationRef=useRef<Operation|null>(null),released=useRef(false),handlers=useRef({onUnauthorized,onIdentityMismatch});handlers.current={onUnauthorized,onIdentityMismatch};
 useEffect(()=>{
  const abort=new AbortController();let current=true;setPreview(null);setLoading(true);setError('');
  void(async()=>{await Promise.resolve();if(abort.signal.aborted)return;try{
   const raw=await applicationApi<unknown>(actorId,path,'GET',undefined,abort.signal,200);
   if(!current||abort.signal.aborted)return;
   if(!validRoundPreview(raw,location.instanceId)||raw.flowId!==flowId)throw new Error('轮次操作资格响应格式不正确');setPreview(raw);
  }catch(cause){if(!current||abort.signal.aborted)return;setError(cause instanceof Error?cause.message:'无法读取轮次资格');if(cause instanceof ApplicationError&&cause.status===401)handlers.current.onUnauthorized();else if(cause instanceof ApplicationError&&cause.code==='AUTH_SESSION_CHANGED')handlers.current.onIdentityMismatch();}finally{if(current&&!abort.signal.aborted)setLoading(false);}})();
  return()=>{current=false;abort.abort();};
 },[actorId,path,flowId,epoch]);
 useEffect(()=>{onDirtyChange(!!job);return()=>onDirtyChange(false);},[job,onDirtyChange]);
 useLayoutEffect(()=>registerLeaveGuard({kind:'workflow-round',actorId,...location},createLeaveController(()=>{
  const status=operationRef.current?.getPendingStatus()??(pending.current?(pending.current.recovered?'unknown':'preflight'):'clean');return {status:released.current?'clean':status,fingerprint:JSON.stringify([status,pending.current?.key,operationRef.current?.packet?.operationId])};
 },decision=>{if(decision==='retain_operation'){released.current=true;return;}if(decision==='discard'&&operationRef.current?.getPendingStatus()==='preflight'){operationRef.current.cancelPreflight();pending.current=null;setJob(null);}})),[scope,registerLeaveGuard]);
 const blocked=!!job||!!result;
 function submit(){
  if(!preview||!kind||pending.current||result||stale||(kind==='resubmit'?!preview.canResubmit:!preview.canReview))return;
  const next:Job={key:crypto.randomUUID(),request:{path,method:'POST',expectedStatus:202,resource:{kind:'round-start',appId:location.appId,viewId:location.viewId,id:location.recordId,instanceId:location.instanceId,flowId:preview.flowId,roundKind:kind},body:{kind,expectedWorkflowRevision:preview.workflowRevision,expectedSchemaVersion:preview.schemaVersion,expectedRecordVersion:preview.recordVersion}}};pending.current=next;setJob(next);setError('');
 }
 function settle(current:Job,receipt?:RoundStartReceipt,message?:string,code?:string){
  if(current!==pending.current)return;pending.current=null;setJob(null);setBusy(false);
  if(receipt)setResult(receipt);else{setError(message??'无法受理新轮次，请刷新后核对。');setStale(['APPLICATION_RECORD_CONFLICT','APPLICATION_SCHEMA_NOT_READY','APPLICATION_POLICY_CONFLICT','WORKFLOW_CONFLICT','WORKFLOW_NOT_READY','APPLICATION_FORBIDDEN','APPLICATION_NOT_FOUND'].includes(code??''));if(code==='APPLICATION_FORBIDDEN'||code==='APPLICATION_NOT_FOUND')setPreview(null);}
 }
 function refresh(){if(blocked)return;setStale(false);setKind(null);setPreview(null);setEpoch(x=>x+1);}
 const title=kind==='resubmit'?'重新提交审批':kind==='review'?'发起重审':'轮次操作：'+name;
 return <Modal title={title} onClose={()=>requestSectionLeave(onClose)} busy={busy}>
  <div className="workflow-round-controls">
   {loading&&<p role="status">正在读取轮次操作资格…</p>}
   {preview&&!kind&&<><p>第 {preview.roundNumber} 轮 · 定义 v{preview.definitionVersion}</p>{preview.latestInstanceId!==location.instanceId?<p>历史轮次仅供查看，请选择最新轮次。</p>:<div className="workflow-instance-buttons">{preview.canRework&&onRework&&<button className="admin-button" type="button" disabled={blocked||stale} onClick={()=>onRework(location.instanceId)}>返工编辑</button>}{preview.canResubmit&&<button className="admin-button" type="button" disabled={blocked||stale} onClick={()=>setKind('resubmit')}>重新提交审批</button>}{preview.canReview&&<button className="admin-button" type="button" disabled={blocked||stale} onClick={()=>setKind('review')}>发起重审</button>}{!preview.canResubmit&&!preview.canReview&&<p>当前不可发起新一轮</p>}</div>}</>}
   {kind&&<><p>{name}</p><p>仅发起此流程的新一轮；不会自动保存记录，也不会重新发起其它流程。</p></>}
   {error&&<p role="alert">{stale?'轮次、记录或权限已变化，请刷新轮次资格并重新核对。':error}</p>}
   {job&&<RoundMutation actorId={actorId} scope={scope} job={job} operationRef={operationRef} onBusyChange={setBusy} settle={settle} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>}
   {result&&<p role="status">已接受新一轮请求，第 {result.roundNumber} 轮；可在关联流程中查看实际进度。</p>}
   {!blocked&&error&&<button className="admin-button" type="button" onClick={refresh}>刷新轮次资格</button>}
   {kind&&!result&&<div className="workflow-instance-confirm"><button className="admin-button" type="button" disabled={blocked} onClick={()=>setKind(null)}>取消</button><button className="admin-button primary" type="button" disabled={blocked||stale||!preview||(kind==='resubmit'?!preview.canResubmit:!preview.canReview)} onClick={submit}>{kind==='resubmit'?'重新提交':'确认重审'}</button></div>}
  </div>
 </Modal>;
}
function RoundMutation({actorId,scope,job,operationRef,onBusyChange,settle,onUnauthorized,onIdentityMismatch}:{actorId:string;scope:string;job:Job;operationRef:MutableRefObject<Operation|null>;onBusyChange:(busy:boolean)=>void;settle:(job:Job,receipt?:RoundStartReceipt,message?:string,code?:string)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const operation=useApplicationOperation<RoundStartReceipt>({actorId,scope,resource:job.request.resource,confirmed:value=>settle(job,value),unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch}),latest=useRef(operation);latest.current=operation;
 useLayoutEffect(()=>{operationRef.current=operation;onBusyChange(operation.phase==='preflight'||operation.phase==='pending');},[operation,onBusyChange]);useLayoutEffect(()=>()=>{operationRef.current=null;},[]);
 useEffect(()=>{let active=true;void Promise.resolve().then(()=>{if(active&&!job.recovered)latest.current.start(job.request);});return()=>{active=false;};},[job]);
 useEffect(()=>{if(operation.phase==='error')settle(job,undefined,operation.message,operation.errorCode);},[operation.phase,operation.message,operation.errorCode]);
 return operation.phase==='unconfirmed'?<div><p role="alert">新轮次结果尚未确认。{operation.message}</p><div className="workflow-instance-buttons"><button className="admin-button" type="button" onClick={operation.query}>核查新轮次结果</button><button className="admin-button" type="button" onClick={operation.retry}>重试原轮次请求</button></div></div>:<p role="status">正在核对新轮次请求…</p>;
}
