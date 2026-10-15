import {FormsDialog} from '../forms/FormsDialog';
import {useCallback,useEffect,useRef,useState} from 'react';
import {applicationApi,applicationApiEnvelope,ApplicationError} from '../api';
import {FieldRenderer,type FieldValue,type LoadReferenceCandidates,type ReferenceDisplay} from '../forms/FieldRenderer';
import type {Field} from '../forms/contracts';
import {validTaskPreview,type TaskLocation,type TaskPreview} from './taskPreviewContract';
import '../forms/forms.css';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {useApplicationOperation} from '../useApplicationOperation';
import {isMutationResult,type MutationResult,type Values} from '../records/contracts';
import type {WorkflowOperation} from './taskActionContract';
import {validTaskCandidates} from './taskCandidateContract';
import {WorkflowHistory,WorkflowComparison} from './WorkflowHistory';
import type {WorkflowEventDetail} from './historyContract';

type ReadState={scope:string;loading:boolean;data:TaskPreview|null;error:string};
function readError(error:unknown):string{
 if(error instanceof ApplicationError){
  if(error.code==='WORKFLOW_TASK_CHANGED')return '任务已变化或已结束，请返回待办重新核对。';
  if(error.code==='WORKFLOW_EVIDENCE_TOO_LARGE')return '审批依据超过读取限制，无法安全展示。';
  return error.message;
 }
 return error instanceof Error&&error.message==='审批预览响应格式不正确'?error.message:'无法读取审批预览，请重试。';
}
function ReadonlyField({field,preview}:{field:Field;preview:TaskPreview}){
 const value=preview.record.values[field.id];
 // A readonly reference is its authorized label, not an editable selector or
 // a private directory lookup. Options may have been deleted since creation.
 if(['member','department','single_select','multi_select'].includes(field.kind)){
  const values=Array.isArray(value)?value:typeof value==='string'?[value]:[];
  const labels=values.map(id=>{
   const display=preview.record.referenceDisplays[field.id]?.[id];
   if(display)return display.label+(display.deleted?'（已删除）':'');
   if(field.kind==='single_select'||field.kind==='multi_select')return field.config.options.find(option=>option.id===id)?.label??'选项信息不可用';
   return '引用信息不可用';
  });
  return <div className="forms-rendered-field" role="group" aria-label={field.name}><span className="forms-field-label">{field.name}</span><div className="workflow-readonly-value">{labels.join('、')||'—'}</div>{field.presentation.helpText&&<small>{field.presentation.helpText}</small>}</div>;
 }
 return <FieldRenderer field={field} value={value} readOnly idPrefix="task-preview"/>;
}
function TaskEditableField({field,value,preview,actorId,path,scope,readOnly,onChange,onUnauthorized,onIdentityMismatch}:{field:Field;value:FieldValue;preview:TaskPreview;actorId:string;path:string;scope:string;readOnly:boolean;onChange:(value:FieldValue)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [selected,setSelected]=useState<ReferenceDisplay|null>(null);
 const kind=field.kind;
 const load=useCallback<LoadReferenceCandidates>(async(request,signal)=>{
  if(kind!=='member'&&kind!=='department')throw new Error('该字段不是引用字段');
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const params=new URLSearchParams({fieldId:field.id,pageSize:String(request.pageSize)});
  if(request.q)params.set('q',request.q);
  if(request.pageToken)params.set('pageToken',request.pageToken);
  try{
   const reply=await applicationApiEnvelope<unknown>(actorId,path+'/reference-candidates?'+params.toString(),'GET',undefined,signal,200);
   if(!validTaskCandidates(reply.data,reply.meta?.pagination,kind,request.pageSize))throw new Error('候选项响应格式不正确');
   return {items:reply.data.items,nextPageToken:reply.meta!.pagination!.nextPageToken};
  }catch(error){
   if(!signal.aborted){if(error instanceof ApplicationError&&error.status===401)onUnauthorized();else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();}
   throw error;
  }
 },[actorId,path,field.id,kind,onUnauthorized,onIdentityMismatch]);
 const selectedId=typeof value==='string'?value:null;
 const original=selectedId?preview.record.referenceDisplays[field.id]?.[selectedId]:undefined;
 const display=original??(selected?.id===selectedId?selected:null);
 return <FieldRenderer field={field} value={value} readOnly={readOnly} idPrefix="task-edit" referenceDisplay={display} referenceScopeKey={scope+':'+field.id} loadReferenceCandidates={kind==='member'||kind==='department'?load:undefined} onChange={(next,candidate)=>{setSelected(candidate?.status==='active'&&candidate.id===next?{id:candidate.id,label:candidate.label,deleted:false}:null);onChange(next);}}/>;
}
export function WorkflowTaskPage({actorId,location,onBack,onUnauthorized,onIdentityMismatch,registerLeaveGuard,requestSectionLeave,onDirtyChange}:{actorId:string;location:TaskLocation;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;onDirtyChange:(dirty:boolean)=>void}){
 const scope=JSON.stringify([actorId,location.appId,location.viewId,location.recordId,location.instanceId,location.taskId]);
 const [epoch,setEpoch]=useState(0);
 const [comparison,setComparison]=useState<{scope:string;detail:WorkflowEventDetail}|null>(null);
 const comparisonOrigin=useRef<HTMLElement|null>(null);
 const [changes,setChanges]=useState<Values>({});
 const [saved,setSaved]=useState(false);
 const [actionResult,setActionResult]=useState<WorkflowOperation|null>(null);
 const [actionWarning,setActionWarning]=useState('');
 const [rejection,setRejection]=useState<{scope:string;reason:string}|null>(null);
 const [state,setState]=useState<ReadState>({scope,loading:true,data:null,error:''});
 const path=`applications/${location.appId}/forms/${location.viewId}/records/${location.recordId}/workflow-instances/${location.instanceId}/tasks/${location.taskId}`;
 const resource={kind:'task-record' as const,appId:location.appId,viewId:location.viewId,id:location.recordId,instanceId:location.instanceId,taskId:location.taskId};
 const operation=useApplicationOperation<MutationResult>({actorId,scope,resource,
  confirmed:()=>{setChanges({});setSaved(true);setEpoch(v=>v+1);},
  unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch,
  valid:(result,packet)=>isMutationResult(result,packet.operationId??'',location.recordId)});
 const actionResource={...resource,kind:'task-action' as const};
 const actionOperation=useApplicationOperation<WorkflowOperation>({actorId,scope,resource:actionResource,
  confirmed:result=>{if(result)setActionResult(result);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const dirty=Object.keys(changes).length>0;
 const saveBusy=operation.phase==='preflight'||operation.phase==='pending',saveUnknown=operation.phase==='unconfirmed';
 const actionBusy=actionOperation.phase==='preflight'||actionOperation.phase==='pending',actionUnknown=actionOperation.phase==='unconfirmed';
 const busy=saveBusy||actionBusy,unknown=saveUnknown||actionUnknown,accepted=actionOperation.phase==='accepted';
 const blocked=busy||unknown||accepted||!!actionResult;
 useEffect(()=>{onDirtyChange(dirty||busy||unknown||accepted);return()=>onDirtyChange(false);},[dirty,busy,unknown,accepted,onDirtyChange]);
 const errorCode=actionOperation.errorCode??operation.errorCode;
 const needsRefresh=['WORKFLOW_BASIS_EXPIRED','WORKFLOW_BASIS_CHANGED','WORKFLOW_TASK_CHANGED','APPLICATION_POLICY_CONFLICT','APPLICATION_SCHEMA_NOT_READY'].includes(errorCode??'');
 const guard=useRef({changes,operation,actionOperation});guard.current={changes,operation,actionOperation};
 useEffect(()=>registerLeaveGuard({kind:'record',actorId,appId:location.appId,viewId:location.viewId,recordId:location.recordId},createLeaveController(()=>{
  const latest=guard.current,pending=latest.operation.getPendingStatus()??latest.actionOperation.getPendingStatus();
  const status=pending??(Object.keys(latest.changes).length?'draft':'clean');
  return {status,fingerprint:JSON.stringify([latest.changes,latest.operation.packet?.operationId,latest.actionOperation.packet?.operationId,status])};
 },decision=>{if(decision==='discard'){guard.current.operation.cancelPreflight();guard.current.actionOperation.cancelPreflight();setChanges({});}})),[scope,registerLeaveGuard]);
 const refresh=()=>requestSectionLeave(()=>{setChanges({});setState({scope,loading:true,data:null,error:''});operation.dismissError();actionOperation.dismissError();setActionResult(null);setActionWarning('');setEpoch(v=>v+1);});
 useEffect(()=>{
  const controller=new AbortController();let current=true;
  setState({scope,loading:true,data:null,error:''});
  void (async()=>{
   await Promise.resolve();if(controller.signal.aborted)return;
   try{
    const raw=await applicationApi<unknown>(actorId,path,'GET',undefined,controller.signal,200);
    if(!current||controller.signal.aborted)return;
    if(!validTaskPreview(raw,location))throw new Error('审批预览响应格式不正确');
    setState({scope,loading:false,data:raw,error:''});
   }catch(error){
    if(!current||controller.signal.aborted)return;
    setState({scope,loading:false,data:null,error:readError(error)});
    if(error instanceof ApplicationError&&error.status===401)onUnauthorized();
    else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
   }
  })();
  return()=>{current=false;controller.abort();};
 },[scope,epoch,actorId,path,onUnauthorized,onIdentityMismatch]);
 const current=state.scope===scope?state:null;
 const accessLost=errorCode==='APPLICATION_FORBIDDEN'||errorCode==='APPLICATION_NOT_FOUND';
 const preview=accessLost?null:current?.data;
 const editable=new Set(preview?.editableFieldIds??[]);
 const save=()=>{
  if(!preview||!dirty||blocked||needsRefresh||operation.getPendingStatus()||actionOperation.getPendingStatus())return;
  const permitted=Object.fromEntries(Object.entries(changes).filter(([id])=>editable.has(id)));
  if(!Object.keys(permitted).length)return;
  setSaved(false);
  operation.start({resource,path:path+'/record',method:'PATCH',expectedStatus:200,body:{basisToken:preview.basisToken,changes:permitted}});
 };
 const agree=()=>{
  if(!preview||blocked||needsRefresh||operation.getPendingStatus()||actionOperation.getPendingStatus())return;
  if(dirty){setActionWarning('有未保存的修改，请先保存或明确放弃后再审批。');return;}
  setActionWarning('');
  actionOperation.start({resource:actionResource,path:path+'/actions',method:'POST',expectedStatus:202,body:{action:'agree',basisToken:preview.basisToken}});
 };
 const validReason=(value:string)=>Array.from(value).length>=1&&Array.from(value).length<=500&&!/^[\p{White_Space}]*$/u.test(value)&&!/[\u0000\uD800-\uDFFF]/u.test(value);
 const openReject=()=>{
  if(!preview||blocked||needsRefresh||operation.getPendingStatus()||actionOperation.getPendingStatus())return;
  if(dirty){setActionWarning('有未保存的修改，请先保存或明确放弃后再审批。');return;}
  setActionWarning('');setRejection({scope,reason:''});
 };
 const reject=()=>{
  if(!preview||!rejection||rejection.scope!==scope||!validReason(rejection.reason)||blocked||needsRefresh||dirty||operation.getPendingStatus()||actionOperation.getPendingStatus())return;
  actionOperation.start({resource:actionResource,path:path+'/actions',method:'POST',expectedStatus:202,body:{action:'reject',basisToken:preview.basisToken,reason:rejection.reason}});
  setRejection(null);
 };
 const message=actionWarning||(errorCode==='WORKFLOW_BASIS_EXPIRED'?'审批依据已过期，请刷新预览后重新核对。':errorCode==='WORKFLOW_BASIS_CHANGED'?'审批数据已变化，请刷新预览后重新核对。':errorCode==='WORKFLOW_TASK_CHANGED'?'任务已变化或已结束，请返回待办重新核对。':operation.message||(accepted?'':actionOperation.message));
 const showComparison=!!preview&&comparison?.scope===scope;
 return <section className="workflow-task-page" aria-label="审批任务详情">
  <div className="workflow-task-toolbar"><button type="button" className="admin-button" onClick={onBack}>返回待办</button><button type="button" className="admin-button" disabled={!current||current.loading||busy||unknown||accepted} onClick={refresh}>刷新预览</button></div>
  <div hidden={showComparison} className="workflow-task-content">
  <div className="app-page-heading"><h1>审批处理</h1></div><p className="workflow-task-reference">记录 {location.recordId}</p>
  {saved&&<p role="status">修改已保存</p>}
  {message&&<p role="alert">{message}</p>}
  {(saveUnknown||saveBusy&&operation.packet)&&<div className="workflow-task-toolbar"><span role="status">{saveBusy?'正在核对保存结果…':'原保存结果待核对'}</span><button type="button" className="admin-button" disabled={saveBusy} onClick={operation.query}>核查保存结果</button><button type="button" className="admin-button" disabled={saveBusy} onClick={operation.retry}>重试原保存</button></div>}
  {actionResult&&<p role="status">{actionResult.status==='success'?(actionResult.instanceState==='rejected'?'审批已驳回':'审批已同意'):'操作未执行'}</p>}
  {(accepted||actionUnknown||actionBusy&&actionOperation.packet)&&<div className="workflow-task-toolbar"><span role="status">{accepted?'审批处理中':actionBusy?'正在核对审批结果…':'原审批结果待核对'}</span><button type="button" className="admin-button" disabled={actionBusy} onClick={actionOperation.query}>核查审批结果</button><button type="button" className="admin-button" disabled={actionBusy} onClick={actionOperation.retry}>重试原审批</button></div>}
  {!current||current.loading?<div className="app-surface app-state" role="status">正在读取审批预览…</div>
   :current.error?<div className="app-surface app-state"><p role="alert">{current.error}</p><button type="button" className="admin-button" onClick={()=>setEpoch(v=>v+1)}>重试读取</button></div>
   :preview?<div className="workflow-task-workspace"><section className="app-surface workflow-task-card" aria-label="当前审批数据"><div className="workflow-task-section-heading"><h2>当前记录</h2><span>最新 v{preview.record.recordVersion}</span></div><div className="workflow-task-fields">{preview.fields.map(field=>editable.has(field.id)?<TaskEditableField key={field.id} field={field} value={Object.hasOwn(changes,field.id)?changes[field.id]:preview.record.values[field.id]} preview={preview} actorId={actorId} path={path} scope={scope} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} readOnly={blocked||needsRefresh} onChange={value=>{setActionWarning('');setSaved(false);setChanges(previous=>{const next={...previous};if(JSON.stringify(value)===JSON.stringify(preview.record.values[field.id]))delete next[field.id];else next[field.id]=value;return next;});}}/>:<ReadonlyField key={field.id} field={field} preview={preview}/>)}</div>{!preview.fields.length&&<p>没有可展示的字段</p>}{editable.size>0&&<div><button type="button" className="admin-button" disabled={!dirty||blocked||needsRefresh} onClick={save}>保存修改</button></div>}<div className="workflow-task-decisions"><button type="button" className="admin-button primary" disabled={blocked||needsRefresh} onClick={agree}>同意</button><button type="button" className="admin-button" disabled={blocked||needsRefresh} onClick={openReject}>驳回</button></div></section><WorkflowHistory actorId={actorId} location={location} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onCompare={detail=>{comparisonOrigin.current=document.activeElement as HTMLElement|null;setComparison({scope,detail});}}/></div>:null}
  </div>
  {preview&&rejection?.scope===scope&&<FormsDialog title="驳回申请" onClose={()=>setRejection(null)}>
   <div className="workflow-reject-dialog"><label>驳回原因（必填，最多500字）<input required value={rejection.reason} onChange={event=>setRejection({scope,reason:event.target.value})}/></label>
    <div className="forms-dialog-actions"><button type="button" data-forms-close>取消</button><button type="button" className="forms-primary" disabled={!validReason(rejection.reason)||blocked||dirty||needsRefresh} onClick={reject}>确认驳回</button></div>
   </div>
  </FormsDialog>}
  {showComparison&&comparison&&preview&&<WorkflowComparison detail={comparison.detail} preview={preview} onBack={()=>{setComparison(null);requestAnimationFrame(()=>comparisonOrigin.current?.isConnected&&comparisonOrigin.current.focus());}}/>}
 </section>;
}
