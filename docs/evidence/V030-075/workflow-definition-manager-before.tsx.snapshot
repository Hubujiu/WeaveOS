import {useCallback,useEffect,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {useResourceQuery} from '../../usePersonnelQuery';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {validWorkflowManagerPage,type WorkflowManagerItem,type WorkflowManagerPage} from './managerContract';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';

type Parameters={page:number;pageSize:number};
const states={disabled:'已关闭',enabled:'已启用',closing:'关闭中'};
const events={manual:'手动', 'record.created':'记录新增', 'record.updated':'记录修改'};
export function WorkflowManager({actorId,appId,viewId,onBack,onUnauthorized,onIdentityMismatch}:{actorId:string;appId:string;viewId:string;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [parameters,setParameters]=useState<Parameters>({page:1,pageSize:20});
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,'applications/'+appId+'/forms/'+viewId+'/workflows/search',body,signal);
  if(!validWorkflowManagerPage(raw,body.page,body.pageSize))throw new Error('流程列表响应格式不正确，请重新读取。');
  return raw;
 },[actorId,appId,viewId]);
 const query=useResourceQuery<WorkflowManagerPage,Parameters>(JSON.stringify(['workflow-manager',actorId,appId,viewId]),parameters,{items:[],total:0,...parameters,queryVersion:''},transport);
 const failure=query.error,changed=failure instanceof ApplicationError&&failure.code==='COMMON_QUERY_CHANGED',expired=failure instanceof ApplicationError&&failure.code==='COMMON_QUERY_CONTEXT_EXPIRED';
 useEffect(()=>{if(failure instanceof ApplicationError&&failure.status===401)onUnauthorized();else if(failure instanceof ApplicationError&&failure.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();},[failure,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(old=>({...old,page:1}));query.refresh();};
 const columns:TableColumn<WorkflowManagerItem>[]=[
  {key:'name',header:'流程名称',dataType:'text',sortable:false,cell:row=><span title={row.name}>{row.name}</span>},
  {key:'triggerEvents',header:'触发方式',dataType:'text',sortable:false,cell:row=><span>{row.currentVersion===0?'未发布':row.triggerEvents.length?row.triggerEvents.map(e=>events[e]).join('、'):'无触发'}</span>},
  {key:'state',header:'状态',dataType:'text',sortable:false,cell:row=><span>{states[row.state]}</span>},
  {key:'inFlightCount',header:'进行中',dataType:'number',sortable:false,cell:row=><span>{row.inFlightCount}</span>},
 ];
 return <section className="workflow-manager" aria-label="流程管理">
  <div className="app-page-heading"><h1>审批流程</h1><div className="workflow-manager-actions"><button type="button" className="admin-button" disabled={query.loading} onClick={refresh}>刷新流程</button><button type="button" className="admin-button" onClick={onBack}>返回记录</button></div></div>
  {query.loading?<section className="app-surface app-state" role="status">正在读取流程…</section>:failure?<section className="app-surface app-state"><p role="alert">{changed?'流程已变化，请刷新后重新查看。':expired?'流程查询已过期，请刷新后重新查看。':failure.message}</p>{!changed&&!expired&&<button type="button" className="admin-button" onClick={query.recheck}>重试</button>}</section>:<section className="member-table surface workflow-manager-table" aria-label="审批流程列表"><div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="审批流程" columns={columns} data={query.data.items} getRowId={row=>row.id} rowHeight={56} minWidth={720} fillViewport paginated manualPagination manualSorting recordCount={query.data.total} page={query.data.page} pageSize={query.data.pageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={page=>setParameters(old=>({...old,page}))} onPageSizeChange={pageSize=>setParameters({page:1,pageSize})} emptyState="暂无流程"/></div></section>}
 </section>;
}
