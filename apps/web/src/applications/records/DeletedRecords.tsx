import {useCallback,useEffect,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {useResourceQuery} from '../../usePersonnelQuery';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {validDeletedRecordPage,type DeletedRecordItem,type DeletedRecordPage} from './deletedContract';
import {RecordLifecycleDialog,type RecordLifecycleSelection} from './RecordLifecycleDialog';
import type {RegisterLeaveGuard} from '../forms/leaveGuard';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';
type Props={actorId:string;appId:string;viewId:string;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(value:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void};
export function DeletedRecords({actorId,appId,viewId,onBack,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const [parameters,setParameters]=useState({page:1,pageSize:20}),[selected,setSelected]=useState<RecordLifecycleSelection|null>(null);
 const transport=useCallback(async(body:{page:number;pageSize:number;queryVersion?:string},signal:AbortSignal)=>{
  const result=await applicationReadPost<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/deleted/search`,body,signal);
  if(!validDeletedRecordPage(result,body.page,body.pageSize))throw new Error('已删除记录响应格式不正确');return result;
 },[actorId,appId,viewId]);
 const query=useResourceQuery<DeletedRecordPage,{page:number;pageSize:number}>(JSON.stringify(['deleted-records',actorId,appId,viewId]),parameters,{items:[],total:0,...parameters,queryVersion:'',schemaVersion:1},transport);
 useEffect(()=>{const e=query.error;if(e instanceof ApplicationError&&e.status===401)onUnauthorized();else if(e instanceof ApplicationError&&e.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();if(e)setSelected(null);},[query.error,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(p=>({...p,page:1}));query.refresh();};
 const columns:TableColumn<DeletedRecordItem>[]=[
  {key:'id',header:'记录编号',width:'440px',sortable:false,cell:row=><span>{row.id}</span>},
  {key:'deletedAt',header:'删除时间',width:'280px',sortable:false,cell:row=><span>{row.deletedAt}</span>},
  {key:'recordVersion',header:'记录版本',width:'180px',sortable:false,cell:row=><span>v{row.recordVersion}</span>},
  {key:'actions',header:'操作',width:'152px',sortable:false,cell:row=><button type="button" className="text-button" onClick={()=>setSelected({id:row.id,recordVersion:row.recordVersion,schemaVersion:query.data.schemaVersion,action:'restoration'})}>恢复</button>},
 ];
 return <section className="deleted-records" aria-label="已删除记录">
  <div className="app-page-heading"><h1>已删除记录</h1><button type="button" className="admin-button primary" disabled={!!selected} onClick={onBack}>返回记录</button></div>
  <section className="app-surface deleted-records-card"><div className="workflow-manager-actions"><p>仅显示记录编号、删除时间和版本；恢复不会重新发起审批</p><button type="button" className="admin-button" disabled={query.loading||!!selected} onClick={refresh}>刷新列表</button></div>
   {query.loading?<p role="status">正在读取已删除记录…</p>:query.error?<p role="alert">{query.error.message}</p>:<div className="arca-source personnel-source-scope"><Table className="personnel-source-table" ariaLabel="已删除记录" columns={columns} data={query.data.items} getRowId={row=>row.id} rowHeight={56} minWidth={1052} fillViewport paginated manualPagination manualSorting recordCount={query.data.total} page={query.data.page} pageSize={query.data.pageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={page=>setParameters(p=>({...p,page}))} onPageSizeChange={pageSize=>setParameters({page:1,pageSize})} emptyState="暂无已删除记录"/></div>}
  </section>
  {selected&&<RecordLifecycleDialog requestSectionLeave={requestSectionLeave} key={selected.id} actorId={actorId} appId={appId} viewId={viewId} selection={selected} onClose={()=>setSelected(null)} onConfirmed={()=>{setSelected(null);refresh();}} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onDirtyChange={onDirtyChange} registerLeaveGuard={registerLeaveGuard}/>}
 </section>;
}
