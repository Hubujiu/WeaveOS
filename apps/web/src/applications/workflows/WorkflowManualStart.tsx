import {useCallback,useEffect,useLayoutEffect,useRef,useState,type MutableRefObject} from 'react';
import {Modal} from '../../Modal';
import {useResourceQuery} from '../../usePersonnelQuery';
import {TablePagination} from '../../vendor/arca/components/motion/table/table-pagination';
import {applicationReadPost,ApplicationError} from '../api';
import {useApplicationOperation,type ApplicationMutationRequest} from '../useApplicationOperation';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {findManualStartRecovery} from './manualStartRecovery';
import {validManualStartPage,type ManualStartOption,type ManualStartPage,type ManualStartReceipt} from './manualStartContract';

type Props={actorId:string;appId:string;viewId:string;recordId:string;onClose:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void};
type Parameters={page:number;pageSize:number};
type Job={key:string;request:ApplicationMutationRequest;recovered?:boolean};
type Operation=ReturnType<typeof useApplicationOperation<ManualStartReceipt>>;
export function WorkflowManualStart(props:Props){return <ManualStartScope key={JSON.stringify([props.actorId,props.appId,props.viewId,props.recordId])} {...props}/>;}
function ManualStartScope({actorId,appId,viewId,recordId,onClose,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const scope=JSON.stringify(['manual-start',actorId,appId,viewId,recordId]),path=`applications/${appId}/forms/${viewId}/records/${recordId}`;
 const [parameters,setParameters]=useState<Parameters>({page:1,pageSize:20}),[selected,setSelected]=useState<ManualStartOption|null>(null);
 const [job,setJob]=useState<Job|null>(()=>{
  const packet=findManualStartRecovery(actorId,appId,viewId,recordId);
  return packet?.resource?{key:packet.operationId!,recovered:true,request:{resource:packet.resource,path:packet.path,method:packet.method,expectedStatus:packet.expectedStatus,body:packet.body}}:null;
 }),[result,setResult]=useState<ManualStartReceipt|null>(null),[error,setError]=useState(''),[stale,setStale]=useState(false);
 const [operationBusy,setOperationBusy]=useState(false);
 const pending=useRef<Job|null>(job),operationRef=useRef<Operation|null>(null),released=useRef(false);
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,path+'/workflow-start-options/search',body,signal);
  if(!validManualStartPage(raw,body.page,body.pageSize))throw new Error('可发起流程响应格式不正确');return raw;
 },[actorId,path]);
 const query=useResourceQuery<ManualStartPage,Parameters>(scope,parameters,{items:[],total:0,...parameters,queryVersion:''},transport);
 const failure=query.error,changed=failure instanceof ApplicationError&&failure.code==='APPLICATION_QUERY_CHANGED',expired=failure instanceof ApplicationError&&failure.code==='APPLICATION_QUERY_CONTEXT_EXPIRED';
 useEffect(()=>{if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();},[failure,onUnauthorized,onIdentityMismatch]);
 useEffect(()=>{onDirtyChange(!!job);return()=>onDirtyChange(false);},[job,onDirtyChange]);
 useLayoutEffect(()=>registerLeaveGuard({kind:'manual-start',actorId,appId,viewId,recordId},createLeaveController(()=>{
  const operation=operationRef.current,status=operation?.getPendingStatus()??(pending.current?(pending.current.recovered?'unknown':'preflight'):'clean');
  return {status:released.current?'clean':status,fingerprint:JSON.stringify([status,operation?.packet?.operationId,pending.current?.key])};
 },decision=>{
  if(decision==='retain_operation'){released.current=true;return;}
  if(decision==='discard'&&operationRef.current?.getPendingStatus()==='preflight'){operationRef.current.cancelPreflight();pending.current=null;setJob(null);}
 })),[scope,registerLeaveGuard]);
 const blocked=!!job||!!result;
 function refresh(){if(blocked)return;setSelected(null);setError('');setStale(false);setParameters({page:1,pageSize:20});query.refresh();}
 function start(){
  if(!selected||pending.current||result||stale||query.blocked)return;
  const next:Job={key:crypto.randomUUID(),request:{resource:{kind:'manual-start',appId,viewId,id:recordId,flowId:selected.flowId},path:path+'/workflow-starts',method:'POST',expectedStatus:202,body:{flowId:selected.flowId,expectedWorkflowRevision:selected.workflowRevision,expectedSchemaVersion:selected.schemaVersion,expectedRecordVersion:selected.recordVersion}}};
  pending.current=next;setError('');setJob(next);
 }
 function settle(current:Job,receipt?:ManualStartReceipt,message?:string,code?:string){
  if(pending.current!==current)return;pending.current=null;setJob(null);
  if(receipt)setResult(receipt);else {setError(message??'无法发起，请刷新后重新核对。');setStale(['APPLICATION_RECORD_CONFLICT','APPLICATION_SCHEMA_NOT_READY','APPLICATION_POLICY_CONFLICT','WORKFLOW_NOT_READY','WORKFLOW_CONFLICT','APPLICATION_FORBIDDEN','APPLICATION_NOT_FOUND'].includes(code??''));}
 }
 const close=()=>requestSectionLeave(onClose);
 return <Modal title="发起流程" onClose={close} busy={!!job&&operationBusy}>
  <div className="workflow-instance-controls">
   {query.loading?<p role="status">正在读取可发起流程…</p>:failure?<p role="alert">{changed?'可发起流程已变化，请刷新后重新选择。':expired?'可发起流程查询已过期，请刷新后重新选择。':failure.message}</p>:<>
    {!query.data.items.length&&<p>暂无可发起的流程</p>}
    <fieldset disabled={blocked||stale} className="workflow-manual-options"><legend>选择流程</legend>{query.data.items.map(option=><label className="forms-checkbox" key={option.flowId}><input type="radio" name="manual-flow" checked={selected?.flowId===option.flowId} onChange={()=>setSelected(option)}/>{option.name||'未命名流程'}</label>)}</fieldset>
    {query.data.total>20&&<div className="arca-source personnel-source-scope"><TablePagination pageIndex={query.data.page-1} pageSize={20} pageCount={Math.max(1,Math.ceil(query.data.total/20))} recordCount={query.data.total} sizes={[20]} rowsPerPageLabel="每页" info="{from}–{to} / {count}" previousPageLabel="上一页" nextPageLabel="下一页" onPageIndexChange={index=>{if(!blocked){setSelected(null);setParameters({page:index+1,pageSize:20});}}} onPageSizeChange={()=>{}}/></div>}
   </>}
   {error&&<p role="alert">{stale?'流程或记录已变化，请刷新可发起流程并重新核对。':error}</p>}
   {job&&<ManualStartMutation key={job.key} actorId={actorId} scope={scope} job={job} operationRef={operationRef} onBusyChange={setOperationBusy} settle={settle} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>}
   {result&&<p role="status">{result.status==='ignored'?'该流程已在进行中，未重复发起。':'已接受发起请求，可在关联流程中查看实际进度。'}</p>}
   {!blocked&&(failure||stale||error)&&<button type="button" className="admin-button" onClick={refresh}>刷新可发起流程</button>}
   <div className="workflow-instance-confirm"><button type="button" className="admin-button" disabled={!!job} onClick={close}>{result?'关闭':'取消'}</button>{!result&&<button type="button" className="admin-button primary" disabled={!selected||blocked||stale||query.blocked} onClick={start}>确认发起</button>}</div>
  </div>
 </Modal>;
}
function ManualStartMutation({actorId,scope,job,operationRef,onBusyChange,settle,onUnauthorized,onIdentityMismatch}:{actorId:string;scope:string;job:Job;operationRef:MutableRefObject<Operation|null>;onBusyChange:(busy:boolean)=>void;settle:(job:Job,result?:ManualStartReceipt,message?:string,code?:string)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const operation=useApplicationOperation<ManualStartReceipt>({actorId,scope,resource:job.request.resource,confirmed:value=>settle(job,value),unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const latest=useRef(operation);latest.current=operation;
 useLayoutEffect(()=>{operationRef.current=operation;onBusyChange(operation.phase==='preflight'||operation.phase==='pending');},[operation,onBusyChange]);
 useLayoutEffect(()=>()=>{operationRef.current=null;},[]);
 useEffect(()=>{let current=true;void Promise.resolve().then(()=>{if(current&&!job.recovered)latest.current.start(job.request);});return()=>{current=false;};},[job]);
 useEffect(()=>{if(operation.phase==='error')settle(job,undefined,operation.message,operation.errorCode);},[operation.phase,operation.message,operation.errorCode]);
 if(operation.phase==='unconfirmed')return <div><p role="alert">发起结果尚未确认。{operation.message}</p><div className="workflow-instance-buttons"><button type="button" className="admin-button" onClick={operation.query}>核查发起结果</button><button type="button" className="admin-button" onClick={operation.retry}>重试原发起</button></div></div>;
 return <p role="status">正在核对发起请求…</p>;
}
