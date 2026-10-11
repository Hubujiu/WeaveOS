import {useCallback,useEffect,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {WorkflowInstanceActions} from './WorkflowInstanceActions';
import type {RegisterLeaveGuard} from '../forms';
import {useResourceQuery} from '../../usePersonnelQuery';
import {TablePagination} from '../../vendor/arca/components/motion/table/table-pagination';
import {validRecordWorkflows,type RecordWorkflow,type RecordWorkflowPage} from './recordWorkflowsContract';

type Props={actorId:string;appId:string;viewId:string;recordId:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;onOpenAction:(action:()=>void)=>void;onDirtyChange:(dirty:boolean)=>void};
type Parameters={page:number;pageSize:number};
const states:Record<RecordWorkflow['state'],string>={starting:'正在发起',active:'审批中',completed:'已通过',rejected:'已驳回',withdrawn:'已撤回',no_effect:'未执行'};
export function RecordWorkflows(props:Props){return <RecordWorkflowsScope key={JSON.stringify([props.actorId,props.appId,props.viewId,props.recordId])} {...props}/>;}
function RecordWorkflowsScope({actorId,appId,viewId,recordId,onUnauthorized,onIdentityMismatch,registerLeaveGuard,requestSectionLeave,onOpenAction,onDirtyChange}:Props){
 const [selected,setSelected]=useState<RecordWorkflow|null>(null);
 const [parameters,setParameters]=useState<Parameters>({page:1,pageSize:20});
 const path=`applications/${appId}/forms/${viewId}/records/${recordId}/workflow-instances/search`;
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,path,body,signal);
  if(!validRecordWorkflows(raw,body.page,body.pageSize))throw new Error('关联流程响应格式不正确');
  return raw;
 },[actorId,path]);
 const query=useResourceQuery<RecordWorkflowPage,Parameters>(JSON.stringify(['record-workflows',actorId,path]),parameters,{items:[],total:0,...parameters,queryVersion:''},transport);
 const failure=query.error;
 const changed=failure instanceof ApplicationError&&failure.code==='APPLICATION_QUERY_CHANGED';
 const expired=failure instanceof ApplicationError&&['APPLICATION_QUERY_CONTEXT_EXPIRED','COMMON_QUERY_CONTEXT_EXPIRED'].includes(failure.code);
 useEffect(()=>{
  if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();
  else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
 },[failure,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(old=>({...old,page:1}));query.refresh();};
 return <section className="app-surface record-workflow-card" aria-label="关联流程">
  <div className="workflow-task-section-heading"><h2>关联流程{!query.loading&&!failure?' '+query.data.total:''}</h2><button type="button" className="text-button" disabled={query.loading} onClick={refresh}>刷新流程</button></div>
  {query.loading?<p role="status">正在读取关联流程…</p>:failure?<div><p role="alert">{changed?'关联流程已变化，请刷新流程。':expired?'关联流程查询已过期，请刷新流程。':failure.message}</p>{!changed&&!expired&&<button type="button" className="admin-button" onClick={query.recheck}>重试流程读取</button>}</div>:<>
   {!query.data.items.length&&<p>暂无关联流程</p>}
   {query.data.items.map(flow=><article key={flow.id} className="record-workflow-item"><h3>{flow.name||'未命名流程'}</h3><p className="record-workflow-state">{states[flow.state]} · 定义 v{flow.definitionVersion}</p><p className="workflow-history-meta">实例 {flow.id}</p><p className="workflow-history-meta"><time dateTime={flow.updatedAt}>{new Date(flow.updatedAt).toLocaleString('zh-CN',{hour12:false})}</time></p><button type="button" className="text-button" aria-label={'查看流程操作：'+(flow.name||'未命名流程')} onClick={()=>onOpenAction(()=>setSelected(flow))}>查看流程操作</button></article>)}
   {query.data.total>20&&<div className="arca-source personnel-source-scope"><TablePagination pageIndex={query.data.page-1} pageSize={20} pageCount={Math.max(1,Math.ceil(query.data.total/20))} recordCount={query.data.total} sizes={[20]} rowsPerPageLabel="每页" info="{from}–{to} / {count}" previousPageLabel="上一页" nextPageLabel="下一页" onPageIndexChange={index=>setParameters({page:index+1,pageSize:20})} onPageSizeChange={()=>{}}/></div>}
  </>}
  {selected&&<WorkflowInstanceActions key={selected.id} actorId={actorId} location={{appId,viewId,recordId,instanceId:selected.id}} name={selected.name||'未命名流程'} onClose={()=>setSelected(null)} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onDirtyChange={onDirtyChange} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave}/>}
 </section>;
}
