import {useEffect,useState} from 'react';
import {applicationApi,ApplicationError} from '../api';
import {FieldRenderer} from '../forms/FieldRenderer';
import type {Field} from '../forms/contracts';
import {validTaskPreview,type TaskLocation,type TaskPreview} from './taskPreviewContract';
import '../forms/forms.css';

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
export function WorkflowTaskPage({actorId,location,onBack,onUnauthorized,onIdentityMismatch}:{actorId:string;location:TaskLocation;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const scope=JSON.stringify([actorId,location.appId,location.viewId,location.recordId,location.instanceId,location.taskId]);
 const [epoch,setEpoch]=useState(0);
 const [state,setState]=useState<ReadState>({scope,loading:true,data:null,error:''});
 const path=`applications/${location.appId}/forms/${location.viewId}/records/${location.recordId}/workflow-instances/${location.instanceId}/tasks/${location.taskId}`;
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
 return <section className="workflow-task-page" aria-label="审批任务详情">
  <div className="workflow-task-toolbar"><button type="button" className="admin-button" onClick={onBack}>返回待办</button><button type="button" className="admin-button" disabled={!current||current.loading} onClick={()=>setEpoch(v=>v+1)}>刷新预览</button></div>
  <div className="app-page-heading"><h1>审批处理</h1></div><p className="workflow-task-reference">记录 {location.recordId}</p>
  {!current||current.loading?<div className="app-surface app-state" role="status">正在读取审批预览…</div>
   :current.error?<div className="app-surface app-state"><p role="alert">{current.error}</p><button type="button" className="admin-button" onClick={()=>setEpoch(v=>v+1)}>重试读取</button></div>
   :preview?<section className="app-surface workflow-task-card" aria-label="当前审批数据"><div className="workflow-task-section-heading"><h2>当前记录</h2><span>最新 v{preview.record.recordVersion}</span></div><div className="workflow-task-fields">{preview.fields.map(field=><ReadonlyField key={field.id} field={field} preview={preview}/>)}</div>{!preview.fields.length&&<p>没有可展示的字段</p>}</section>:null}
 </section>;
}
