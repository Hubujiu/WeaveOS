import {useEffect,useRef,useState} from 'react';
import {applicationApi,ApplicationError} from '../api';
import {WorkflowHistory,WorkflowComparison} from './WorkflowHistory';
import {validWorkflowEventDetail,type WorkflowEventDetail} from './historyContract';
import {parseComparisonSnapshot,type ComparisonSnapshot} from './comparisonSnapshot';

type Props={actorId:string;location:{appId:string;viewId:string;recordId:string};onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void};
export function WorkflowRecordHistory(props:Props){
 const {actorId,location}=props;
 return <WorkflowRecordHistoryScope key={JSON.stringify([actorId,location.appId,location.viewId,location.recordId])} {...props}/>;
}
function WorkflowRecordHistoryScope({actorId,location,onBack,onUnauthorized,onIdentityMismatch}:Props){
 const [mode,setMode]=useState(false),[comparison,setComparison]=useState<{detail:WorkflowEventDetail;snapshot:ComparisonSnapshot}|null>(null),[error,setError]=useState('');
 const controller=useRef<AbortController|null>(null),origin=useRef<HTMLElement|null>(null);
 useEffect(()=>()=>controller.current?.abort(),[]);
 const close=()=>{controller.current?.abort();setMode(false);setComparison(null);requestAnimationFrame(()=>origin.current?.focus());};
 const compare=async(original:WorkflowEventDetail)=>{
  origin.current=document.activeElement instanceof HTMLElement?document.activeElement:null;
  controller.current?.abort();const pending=new AbortController();controller.current=pending;
  setMode(true);setComparison(null);setError('');
  try{
   const base=`applications/${location.appId}/forms/${location.viewId}`;
   const detail=await applicationApi<unknown>(actorId,base+'/records/'+location.recordId+'/workflow-events/'+original.event.id,'GET',undefined,pending.signal,200);
   if(pending.signal.aborted)return;
   if(!validWorkflowEventDetail(detail,original.event))throw new Error('审批依据响应格式不正确');
   if(detail.basis.status==='unavailable')throw new Error('当时审批依据不可用');
   const view=await applicationApi<unknown>(actorId,base+'/runtime','GET',undefined,pending.signal,200);
   if(pending.signal.aborted)return;
   const record=await applicationApi<unknown>(actorId,base+'/records/'+location.recordId,'GET',undefined,pending.signal,200);
   if(pending.signal.aborted)return;
   const snapshot=parseComparisonSnapshot(view,record,location,actorId);
   if(!snapshot)throw new Error('当前记录响应格式不正确');
   setComparison({detail,snapshot});
  }catch(failure){
   if(pending.signal.aborted)return;
   setMode(false);setComparison(null);setError(failure instanceof Error?failure.message:'无法读取当前记录，请重试。');
   if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();
   else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
  }
 };
 return <>
  <div hidden={mode}><section className="workflow-task-page" aria-label="记录审批轨迹">
   <div className="app-page-heading"><h1>记录审批轨迹</h1></div>
   <div className="workflow-task-toolbar"><button type="button" className="admin-button" onClick={onBack}>返回我发起的</button></div>
   <p className="workflow-history-meta">同一记录下当前有权查看的审批历史 · 记录 {location.recordId}</p>
   {error?<div className="app-surface app-state"><p role="alert">{error}</p><button type="button" className="admin-button" onClick={()=>setError('')}>重新读取历史</button></div>:<WorkflowHistory actorId={actorId} location={location} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onCompare={detail=>void compare(detail)}/>}
  </section></div>
  {mode&&(comparison?<WorkflowComparison detail={comparison.detail} snapshot={comparison.snapshot} backLabel="返回记录审批轨迹" onBack={close}/>:<section className="workflow-task-page" aria-label="版本差异读取"><div className="app-page-heading"><h1>版本差异</h1></div><button type="button" className="admin-button" onClick={close}>返回记录审批轨迹</button><p role="status">正在核对历史与当前记录…</p></section>)}
 </>;
}
