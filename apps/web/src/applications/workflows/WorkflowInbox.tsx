import {useCallback,useEffect,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {useResourceQuery} from '../../usePersonnelQuery';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {validInboxPage,type InboxItem,type InboxPage} from './inboxContract';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';

type Parameters={page:number;pageSize:number};
const columns:TableColumn<InboxItem>[]=[
 {key:'flowName',header:'流程',dataType:'text',sortable:false,cell:row=>row.flowName},
 {key:'recordId',header:'记录编号',dataType:'text',sortable:false,cell:row=><span title={row.recordId}>{row.recordId}</span>},
 {key:'nodeId',header:'节点编号',dataType:'text',sortable:false,cell:row=><span title={row.nodeId}>{row.nodeId}</span>},
 {key:'createdAt',header:'时间',dataType:'time',sortable:false,cell:row=><time dateTime={row.createdAt}>{new Date(row.createdAt).toLocaleString('zh-CN',{hour12:false})}</time>},
];
export function WorkflowInbox({actorId,onUnauthorized,onIdentityMismatch}:{actorId:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [parameters,setParameters]=useState<Parameters>({page:1,pageSize:20});
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  // As in RecordWorkspace, StrictMode's abandoned initial setup does no request.
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,'workflow-tasks/search',body,signal);
  if(!validInboxPage(raw,body.page,body.pageSize))throw new Error('待办响应格式不正确');
  return raw;
 },[actorId]);
 const query=useResourceQuery<InboxPage,Parameters>('workflow-inbox:'+actorId,parameters,{items:[],total:0,...parameters,queryVersion:''},transport);
 const failure=query.error;
 const changed=failure instanceof ApplicationError&&failure.code==='APPLICATION_QUERY_CHANGED';
 const expired=failure instanceof ApplicationError&&['APPLICATION_QUERY_CONTEXT_EXPIRED','COMMON_QUERY_CONTEXT_EXPIRED'].includes(failure.code);
 useEffect(()=>{
  if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();
  else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
 },[failure,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(old=>({...old,page:1}));query.refresh();};
 return <section className="workflow-inbox" aria-label="个人审批待办">
  <div className="app-page-heading"><h1>我的待办</h1></div>
  <div className="workflow-inbox-toolbar"><span className="workflow-inbox-current">待我审批{!query.loading&&!failure?' '+query.data.total:''}</span><button type="button" className="admin-button" disabled={query.loading} onClick={refresh}>刷新待办</button></div>
  {query.loading?<div className="app-surface app-state" role="status">正在读取待办…</div>
   :failure?<div className="app-surface app-state"><p role="alert">{changed?'待办已变化，请刷新后重新查看。':expired?'待办查询已过期，请刷新后重新查看。':failure.message}</p>{!changed&&!expired&&<button type="button" className="admin-button" onClick={query.recheck}>重试</button>}</div>
   :<section className="member-table surface workflow-inbox-table" aria-label="待办列表"><div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="待办" columns={columns} data={query.data.items} getRowId={row=>row.id} rowHeight={56} fillViewport paginated manualPagination manualSorting recordCount={query.data.total} page={query.data.page} pageSize={query.data.pageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={page=>setParameters(old=>({...old,page}))} onPageSizeChange={pageSize=>setParameters({page:1,pageSize})} emptyState="暂无待办"/></div></section>}
 </section>;
}
