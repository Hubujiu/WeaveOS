import {useCallback,useEffect,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {useResourceQuery} from '../../usePersonnelQuery';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {validWorkflowStartedPage,type WorkflowStartedItem,type WorkflowStartedPage} from './startedContract';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';

type Parameters={page:number;pageSize:number};
export function WorkflowStarted({actorId,onUnauthorized,onIdentityMismatch,onOpenInbox,onOpenApproved,onOpenHistory}:{actorId:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onOpenInbox:()=>void;onOpenApproved:()=>void;onOpenHistory:(item:WorkflowStartedItem)=>void}){
 const states={starting:'正在发起',active:'审批中',completed:'已通过',rejected:'已驳回',withdrawn:'已撤回',no_effect:'未执行'};
 const columns:TableColumn<WorkflowStartedItem>[]=[
 {key:'name',header:'流程',dataType:'text',sortable:false,cell:row=><div><button type="button" className="text-button table-cell" title={row.instance.name} aria-label={'查看记录审批轨迹：'+row.instance.name} onClick={()=>onOpenHistory(row)}>{row.instance.name}</button>{'archived' in row&&row.flowNameSource==='legacy_last_known'&&<span className="workflow-history-meta">最后已知名称</span>}</div>},
 {key:'recordId',header:'记录编号',dataType:'text',sortable:false,cell:row=><span title={row.recordId}>{row.recordId}</span>},
 {key:'state',header:'状态',dataType:'text',sortable:false,cell:row=><div><span>{states[row.instance.state]}</span>{'archived' in row&&<span className="workflow-started-archive workflow-history-meta">已归档</span>}</div>},
 {key:'createdAt',header:'发起时间',dataType:'time',sortable:false,cell:row=><time dateTime={row.instance.createdAt}>{new Date(row.instance.createdAt).toLocaleString('zh-CN',{hour12:false})}</time>},
 {key:'updatedAt',header:'更新时间',dataType:'time',sortable:false,cell:row=><time dateTime={row.instance.updatedAt}>{new Date(row.instance.updatedAt).toLocaleString('zh-CN',{hour12:false})}</time>},
 ];

 const [parameters,setParameters]=useState<Parameters>({page:1,pageSize:20});
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  // As in RecordWorkspace, StrictMode's abandoned initial setup does no request.
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,'workflow-started/search',body,signal);
  if(!validWorkflowStartedPage(raw,body.page,body.pageSize,actorId))throw new Error('发起记录响应格式不正确');
  return raw;
 },[actorId]);
 const query=useResourceQuery<WorkflowStartedPage,Parameters>('workflow-started:'+actorId,parameters,{items:[],total:0,...parameters,queryVersion:''},transport);
 const failure=query.error;
 const changed=failure instanceof ApplicationError&&failure.code==='APPLICATION_QUERY_CHANGED';
 const expired=failure instanceof ApplicationError&&['APPLICATION_QUERY_CONTEXT_EXPIRED','COMMON_QUERY_CONTEXT_EXPIRED'].includes(failure.code);
 useEffect(()=>{
  if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();
  else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
 },[failure,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(old=>({...old,page:1}));query.refresh();};
 return <section className="workflow-inbox" aria-label="个人发起记录">
  <div className="app-page-heading"><h1>我发起的</h1></div>
  <div className="workflow-inbox-toolbar"><button type="button" className="admin-button" onClick={onOpenInbox}>待我审批</button><button type="button" className="admin-button" onClick={onOpenApproved}>我已审批</button><span className="workflow-inbox-current">我发起的{!query.loading&&!failure?' '+query.data.total:''}</span><button type="button" className="admin-button workflow-inbox-refresh" disabled={query.loading} onClick={refresh}>刷新发起记录</button></div>
  {query.loading?<div className="app-surface app-state" role="status">正在读取发起记录…</div>
   :failure?<div className="app-surface app-state"><p role="alert">{changed?'发起记录已变化，请刷新后重新查看。':expired?'发起查询已过期，请刷新后重新查看。':failure.message}</p>{!changed&&!expired&&<button type="button" className="admin-button" onClick={query.recheck}>重试</button>}</div>
   :<section className="member-table surface workflow-inbox-table" aria-label="发起列表"><div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="我发起的" columns={columns} data={query.data.items} getRowId={row=>row.instance.id} rowHeight={56} minWidth={880} fillViewport paginated manualPagination manualSorting recordCount={query.data.total} page={query.data.page} pageSize={query.data.pageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={page=>setParameters(old=>({...old,page}))} onPageSizeChange={pageSize=>setParameters({page:1,pageSize})} emptyState="暂无发起记录"/></div></section>}
 </section>;
}
