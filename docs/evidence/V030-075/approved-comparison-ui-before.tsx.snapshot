import {useEffect,useState} from 'react';
import {applicationApi,ApplicationError} from '../api';
import {validPersonalApprovalDetail} from './approvedDetailContract';
import {historyValue,type WorkflowEventDetail} from './historyContract';

type Location={appId:string;viewId:string;recordId:string;eventId:string};
type Props={actorId:string;location:Location;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void};
export function WorkflowApprovedDetail({actorId,location,onBack,onUnauthorized,onIdentityMismatch}:Props){
 const [epoch,setEpoch]=useState(0),[value,setValue]=useState<WorkflowEventDetail|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState('');
 const path=`applications/${location.appId}/forms/${location.viewId}/records/${location.recordId}/workflow-events/${location.eventId}`;
 useEffect(()=>{
  const controller=new AbortController();setValue(null);setError('');setLoading(true);
  void(async()=>{
   try{
    await Promise.resolve();if(controller.signal.aborted)return;
    const data=await applicationApi<unknown>(actorId,path,'GET',undefined,controller.signal,200);
    if(controller.signal.aborted)return;
    if(!validPersonalApprovalDetail(data,location.eventId,actorId))throw new Error('审批依据响应格式不正确');
    setValue(data);
   }catch(failure){
    if(controller.signal.aborted)return;setValue(null);setError(failure instanceof Error?failure.message:'无法读取审批依据，请重试。');
    if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();
    else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
   }finally{if(!controller.signal.aborted)setLoading(false);}
  })();
  return()=>controller.abort();
 },[actorId,path,location.eventId,epoch,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setValue(null);setEpoch(n=>n+1);};
 return <section className="workflow-task-page" aria-label="历史审批依据">
  <div className="app-page-heading"><h1>历史审批依据</h1></div>
  <div className="workflow-task-toolbar"><button type="button" className="admin-button" onClick={onBack}>返回我已审批</button><button type="button" className="admin-button" disabled={loading} onClick={refresh}>刷新依据</button></div>
  {loading?<div className="app-surface app-state" role="status">正在读取当时依据…</div>:error?<div className="app-surface app-state"><p role="alert">{error}</p><button type="button" className="admin-button" onClick={refresh}>重试</button></div>:value?<section className="app-surface workflow-task-card">
   <div className="workflow-task-section-heading"><h2>{value.event.flowName}</h2>{value.event.flowNameSource==='legacy_last_known'&&<span>最后已知名称</span>}</div>
   <p className="workflow-task-reference">{{agree:'已同意',reject:'已驳回',return:'已退回'}[value.event.action as 'agree'|'reject'|'return']} · 操作人 {value.event.actorId}</p>
   {value.event.nodeId&&<p className="workflow-task-reference">节点 {value.event.nodeId}</p>}
   {value.event.targetNodeId&&<p className="workflow-task-reference">目标节点 {value.event.targetNodeId}</p>}
   <p className="workflow-task-reference"><time dateTime={value.event.occurredAt}>{new Date(value.event.occurredAt).toLocaleString('zh-CN',{hour12:false})}</time> · 审批时记录 v{value.event.recordVersion}</p>
   {value.basis.status==='unavailable'?<p>当时审批依据不可用</p>:value.basis.fields.length===0?<p>当时可见的审批依据为空</p>:<div className="workflow-task-fields">{value.basis.fields.map(field=><div className="forms-rendered-field" key={field.fieldId}><span>{field.fieldName}</span><div className="workflow-readonly-value">{historyValue(field)}</div></div>)}</div>}
  </section>:null}
 </section>;
}
