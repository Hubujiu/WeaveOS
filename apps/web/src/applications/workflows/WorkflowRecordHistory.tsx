import {WorkflowHistory} from './WorkflowHistory';
export function WorkflowRecordHistory({actorId,location,onBack,onUnauthorized,onIdentityMismatch}:{actorId:string;location:{appId:string;viewId:string;recordId:string};onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 return <section className="workflow-task-page" aria-label="记录审批轨迹">
  <div className="app-page-heading"><h1>记录审批轨迹</h1></div>
  <div className="workflow-task-toolbar"><button type="button" className="admin-button" onClick={onBack}>返回我发起的</button></div>
  <p className="workflow-history-meta">同一记录下当前有权查看的审批历史 · 记录 {location.recordId}</p>
  <WorkflowHistory actorId={actorId} location={location} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
 </section>;
}
