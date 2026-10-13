import {useCallback,useEffect,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {useResourceQuery} from '../../usePersonnelQuery';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {validWorkflowApprovalsPage,type WorkflowApproval,type WorkflowApprovalsPage} from './approvalsContract';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';

type Parameters={page:number;pageSize:number};
export function WorkflowApprovals({actorId,onUnauthorized,onIdentityMismatch,onOpenInbox}:{actorId:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onOpenInbox:()=>void}){
 const columns:TableColumn<WorkflowApproval>[]=[
 {key:'flowName',header:'流程',dataType:'text',sortable:false,cell:row=><span className="table-cell" title={row.event.flowName}>{row.event.flowName}</span>},
 {key:'recordId',header:'记录编号',dataType:'text',sortable:false,cell:row=><span title={row.recordId}>{row.recordId}</span>},
 {key:'nodeId',header:'节点编号',dataType:'text',sortable:false,cell:row=><span title={row.event.nodeId??undefined}>{row.event.nodeId??'—'}</span>},
 {key:'action',header:'操作',dataType:'text',sortable:false,cell:row=><span>{{agree:'同意',reject:'驳回',return:'退回'}[row.event.action as 'agree'|'reject'|'return']}</span>},
 {key:'occurredAt',header:'时间',dataType:'time',sortable:false,cell:row=><time dateTime={row.event.occurredAt}>{new Date(row.event.occurredAt).toLocaleString('zh-CN',{hour12:false})}</time>},
];

 const [parameters,setParameters]=useState<Parameters>({page:1,pageSize:20});
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  // As in RecordWorkspace, StrictMode's abandoned initial setup does no request.
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,'workflow-approvals/search',body,signal);
  if(!validWorkflowApprovalsPage(raw,body.page,body.pageSize,actorId))throw new Error('已审批响应格式不正确');
  return raw;
 },[actorId]);
 const query=useResourceQuery<WorkflowApprovalsPage,Parameters>('workflow-approvals:'+actorId,parameters,{items:[],total:0,...parameters,queryVersion:''},transport);
 const failure=query.error;
 const changed=failure instanceof ApplicationError&&failure.code==='APPLICATION_QUERY_CHANGED';
 const expired=failure instanceof ApplicationError&&['APPLICATION_QUERY_CONTEXT_EXPIRED','COMMON_QUERY_CONTEXT_EXPIRED'].includes(failure.code);
 useEffect(()=>{
  if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();
  else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
 },[failure,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(old=>({...old,page:1}));query.refresh();};
 return <section className="workflow-inbox" aria-label="个人已审批记录">
  <div className="app-page-heading"><h1>我已审批</h1></div>
  <div className="workflow-inbox-toolbar"><button type="button" className="admin-button" onClick={onOpenInbox}>待我审批</button><span className="workflow-inbox-current">我已审批{!query.loading&&!failure?' '+query.data.total:''}</span><button type="button" className="admin-button" disabled={query.loading} onClick={refresh}>刷新已审批</button></div>
  {query.loading?<div className="app-surface app-state" role="status">正在读取已审批记录…</div>
   :failure?<div className="app-surface app-state"><p role="alert">{changed?'已审批记录已变化，请刷新后重新查看。':expired?'已审批查询已过期，请刷新后重新查看。':failure.message}</p>{!changed&&!expired&&<button type="button" className="admin-button" onClick={query.recheck}>重试</button>}</div>
   :<section className="member-table surface workflow-inbox-table" aria-label="已审批列表"><div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="已审批" columns={columns} data={query.data.items} getRowId={row=>row.event.id} rowHeight={56} minWidth={880} fillViewport paginated manualPagination manualSorting recordCount={query.data.total} page={query.data.page} pageSize={query.data.pageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={page=>setParameters(old=>({...old,page}))} onPageSizeChange={pageSize=>setParameters({page:1,pageSize})} emptyState="暂无已审批记录"/></div></section>}
 </section>;
}
