import {useEffect,useRef,useState} from 'react';
import {applicationApi,ApplicationError} from '../api';
import {FieldRenderer} from '../forms/FieldRenderer';
import type {Field} from '../forms/contracts';
import {validTaskPreview,type TaskLocation,type TaskPreview} from './taskPreviewContract';
import '../forms/forms.css';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {useApplicationOperation} from '../useApplicationOperation';
import {isMutationResult,type MutationResult,type Values} from '../records/contracts';
import type {WorkflowOperation} from './taskActionContract';

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
export function WorkflowTaskPage({actorId,location,onBack,onUnauthorized,onIdentityMismatch,registerLeaveGuard,requestSectionLeave,onDirtyChange}:{actorId:string;location:TaskLocation;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;onDirtyChange:(dirty:boolean)=>void}){
 const scope=JSON.stringify([actorId,location.appId,location.viewId,location.recordId,location.instanceId,location.taskId]);
 const [epoch,setEpoch]=useState(0);
 const [changes,setChanges]=useState<Values>({});
 const [saved,setSaved]=useState(false);
 const [actionResult,setActionResult]=useState<WorkflowOperation|null>(null);
 const [actionWarning,setActionWarning]=useState('');
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
 const refresh=()=>requestSectionLeave(()=>{setChanges({});operation.dismissError();actionOperation.dismissError();setActionResult(null);setActionWarning('');setEpoch(v=>v+1);});
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
 const current=state.scope===scope?state:null,preview=current?.data;
 const editable=new Set(preview?.editableFieldIds.filter(id=>{
  const field=preview.fields.find(item=>item.id===id);return field&&!['member','department'].includes(field.kind);
 })??[]);
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
 const message=actionWarning||(errorCode==='WORKFLOW_BASIS_EXPIRED'?'审批依据已过期，请刷新预览后重新核对。':errorCode==='WORKFLOW_BASIS_CHANGED'?'审批数据已变化，请刷新预览后重新核对。':errorCode==='WORKFLOW_TASK_CHANGED'?'任务已变化或已结束，请返回待办重新核对。':operation.message||(accepted?'':actionOperation.message));
 return <section className="workflow-task-page" aria-label="审批任务详情">
  <div className="workflow-task-toolbar"><button type="button" className="admin-button" onClick={onBack}>返回待办</button><button type="button" className="admin-button" disabled={!current||current.loading||busy||unknown||accepted} onClick={refresh}>刷新预览</button></div>
  <div className="app-page-heading"><h1>审批处理</h1></div><p className="workflow-task-reference">记录 {location.recordId}</p>
  {saved&&<p role="status">修改已保存</p>}
  {message&&<p role="alert">{message}</p>}
  {(saveUnknown||saveBusy&&operation.packet)&&<div className="workflow-task-toolbar"><span role="status">{saveBusy?'正在核对保存结果…':'原保存结果待核对'}</span><button type="button" className="admin-button" disabled={saveBusy} onClick={operation.query}>核查保存结果</button><button type="button" className="admin-button" disabled={saveBusy} onClick={operation.retry}>重试原保存</button></div>}
  {actionResult&&<p role="status">{actionResult.status==='success'?'审批已同意':'操作未执行'}</p>}
  {(accepted||actionUnknown||actionBusy&&actionOperation.packet)&&<div className="workflow-task-toolbar"><span role="status">{accepted?'审批处理中':actionBusy?'正在核对审批结果…':'原审批结果待核对'}</span><button type="button" className="admin-button" disabled={actionBusy} onClick={actionOperation.query}>核查审批结果</button><button type="button" className="admin-button" disabled={actionBusy} onClick={actionOperation.retry}>重试原审批</button></div>}
  {!current||current.loading?<div className="app-surface app-state" role="status">正在读取审批预览…</div>
   :current.error?<div className="app-surface app-state"><p role="alert">{current.error}</p><button type="button" className="admin-button" onClick={()=>setEpoch(v=>v+1)}>重试读取</button></div>
   :preview?<section className="app-surface workflow-task-card" aria-label="当前审批数据"><div className="workflow-task-section-heading"><h2>当前记录</h2><span>最新 v{preview.record.recordVersion}</span></div><div className="workflow-task-fields">{preview.fields.map(field=>editable.has(field.id)?<FieldRenderer key={field.id} field={field} value={Object.hasOwn(changes,field.id)?changes[field.id]:preview.record.values[field.id]} readOnly={blocked||needsRefresh} idPrefix="task-edit" onChange={value=>{setActionWarning('');setSaved(false);setChanges(previous=>{const next={...previous};if(JSON.stringify(value)===JSON.stringify(preview.record.values[field.id]))delete next[field.id];else next[field.id]=value;return next;});}}/>:<ReadonlyField key={field.id} field={field} preview={preview}/>)}</div>{!preview.fields.length&&<p>没有可展示的字段</p>}{editable.size>0&&<div><button type="button" className="admin-button" disabled={!dirty||blocked||needsRefresh} onClick={save}>保存修改</button></div>}<div><button type="button" className="admin-button primary" disabled={blocked||needsRefresh} onClick={agree}>同意</button></div></section>:null}
 </section>;
}
