import {useEffect,useLayoutEffect,useRef,useState} from 'react';
import {Modal} from '../../Modal';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {applicationApiEnvelope,ApplicationError} from '../api';
import {useApplicationOperation,type ApplicationMutationRequest} from '../useApplicationOperation';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {isDraftMutationResult,type DraftMutationResult,type DraftSummary,type RuntimeView} from './contracts';
import {validDraftPage} from './draftContract';

type Props={actorId:string;view:RuntimeView;onOpen:(id:string)=>void;onClose:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void};
type Job={key:string;request:ApplicationMutationRequest};
type Operation=ReturnType<typeof useApplicationOperation<DraftMutationResult>>;
export function DraftsPanel(props:Props){return <DraftScope key={JSON.stringify([props.actorId,props.view.appId,props.view.viewId])} {...props}/>;}
function DraftScope({actorId,view,onOpen,onClose,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const [token,setToken]=useState<string|null>(null),[epoch,setEpoch]=useState(0),[page,setPage]=useState<{items:DraftSummary[];next:string|null;loading:boolean;error:string}>({items:[],next:null,loading:true,error:''});
 const [selected,setSelected]=useState<DraftSummary|null>(null),[job,setJob]=useState<Job|null>(null),[mutationError,setMutationError]=useState('');
 const pending=useRef<Job|null>(null),operationRef=useRef<Operation|null>(null),released=useRef(false);
 const handlers=useRef({onUnauthorized,onIdentityMismatch});handlers.current={onUnauthorized,onIdentityMismatch};
 const base=`applications/${view.appId}/forms/${view.viewId}/drafts`;
 useEffect(()=>{
  const controller=new AbortController();let active=true;setPage({items:[],next:null,loading:true,error:''});
  void(async()=>{await Promise.resolve();if(controller.signal.aborted)return;try{
   const query=new URLSearchParams({pageSize:'20'});if(token)query.set('pageToken',token);
   const reply=await applicationApiEnvelope<unknown>(actorId,base+'?'+query,'GET',undefined,controller.signal,200);
   if(!active||controller.signal.aborted)return;
   if(!validDraftPage(reply.data,reply.meta?.pagination,view.viewId,view.tableId,20))throw new Error('草稿列表响应格式不正确');
   setPage({items:reply.data.items,next:reply.meta!.pagination!.nextPageToken,loading:false,error:''});
  }catch(error){if(!active||controller.signal.aborted)return;setPage({items:[],next:null,loading:false,error:error instanceof Error?error.message:'无法读取草稿列表'});if(error instanceof ApplicationError&&error.status===401)handlers.current.onUnauthorized();else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')handlers.current.onIdentityMismatch();}})();
  return()=>{active=false;controller.abort();};
 },[actorId,base,view.tableId,view.viewId,token,epoch]);
 useEffect(()=>{onDirtyChange(!!job);return()=>onDirtyChange(false);},[job,onDirtyChange]);
 useLayoutEffect(()=>{
  if(!selected)return;
  return registerLeaveGuard({kind:'draft',actorId,appId:view.appId,viewId:view.viewId,draftId:selected.id},createLeaveController(()=>{
   const status=operationRef.current?.getPendingStatus()??(pending.current?'preflight':'clean');return {status:released.current?'clean':status,fingerprint:JSON.stringify([status,pending.current?.key,operationRef.current?.packet?.operationId])};
  },decision=>{if(decision==='retain_operation')released.current=true;else if(decision==='discard'&&operationRef.current?.getPendingStatus()==='preflight'){operationRef.current.cancelPreflight();pending.current=null;setJob(null);}}));
 },[actorId,view.appId,view.viewId,selected?.id,registerLeaveGuard]);
 const refresh=()=>{if(pending.current)return;setMutationError('');setToken(null);setEpoch(value=>value+1);};
 const settle=(confirmed:boolean,message='')=>{pending.current=null;setJob(null);setSelected(null);if(confirmed)refresh();else setMutationError(message);};
 const remove=()=>{
  if(!selected||pending.current)return;
  const resource={kind:'draft' as const,appId:view.appId,viewId:view.viewId,id:selected.id};
  const next={key:crypto.randomUUID(),request:{resource,path:base+'/'+selected.id,method:'DELETE' as const,expectedStatus:204 as const,expectedDraftVersion:selected.draftVersion}};released.current=false;pending.current=next;setJob(next);
 };
 const columns:TableColumn<DraftSummary>[]=[
  {key:'id',header:'草稿 ID',sortable:false,cell:row=><button className="text-button table-cell" type="button" aria-label={'打开草稿：'+row.id} disabled={!!job||!!mutationError} onClick={()=>onOpen(row.id)}>{row.id}</button>},
  {key:'targetRecordId',header:'目标',sortable:false,cell:row=><span>{row.targetRecordId?'编辑记录 '+row.targetRecordId:'新建记录'}</span>},
  {key:'updatedAt',header:'更新时间',sortable:false,cell:row=><time dateTime={row.updatedAt}>{new Date(row.updatedAt).toLocaleString('zh-CN',{hour12:false})}</time>},
  {key:'hasConflicts',header:'状态',sortable:false,cell:row=><span>{row.hasConflicts?'需要核对':'已保存'}</span>},
  {key:'actions',header:'操作',sortable:false,cell:row=><button className="text-button" type="button" aria-label={'删除草稿：'+row.id} disabled={!!job||!!mutationError} onClick={()=>{released.current=false;setSelected(row);}}>删除</button>},
 ];
 return <Modal title="我的草稿" onClose={()=>requestSectionLeave(onClose)}>
  <div className="workflow-instance-buttons"><button className="admin-button" type="button" disabled={page.loading||!!job} onClick={refresh}>刷新草稿列表</button></div>
  {page.error&&<p role="alert">{page.error}</p>}{mutationError&&<p role="alert">{mutationError} 请刷新草稿列表后核对。</p>}
  {!page.error&&<div className="arca-source personnel-source-scope"><Table className="personnel-source-table" ariaLabel="我的草稿列表" columns={columns} data={page.items} getRowId={row=>row.id} rowHeight={48} minWidth={680} loading={page.loading} emptyState="暂无草稿"/></div>}
  {page.next&&<button className="admin-button" type="button" disabled={page.loading||!!job} onClick={()=>setToken(page.next)}>下一批草稿</button>}
  {selected&&<Modal title="删除草稿" onClose={()=>job?requestSectionLeave(()=>setSelected(null)):setSelected(null)}>
   <p>删除这份草稿？不会删除或修改正式记录。</p><p>{selected.id}</p>
   {job?<DraftDelete actorId={actorId} job={job} operationRef={operationRef} settle={settle} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>:<div className="workflow-instance-confirm"><button className="admin-button" type="button" onClick={()=>setSelected(null)}>取消</button><button className="admin-button" type="button" onClick={remove}>删除草稿</button></div>}
  </Modal>}
 </Modal>;
}
function DraftDelete({actorId,job,operationRef,settle,onUnauthorized,onIdentityMismatch}:{actorId:string;job:Job;operationRef:{current:Operation|null};settle:(confirmed:boolean,message?:string)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const resource=job.request.resource;
 const operation=useApplicationOperation<DraftMutationResult>({actorId,scope:`draft-editor:${resource.appId}:${resource.viewId}:${resource.id}`,resource,confirmed:()=>settle(true),unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch,valid:(result,packet)=>isDraftMutationResult(result,packet.operationId??'',resource.id)});
 const latest=useRef(operation);latest.current=operation;operationRef.current=operation;
 useEffect(()=>()=>{operationRef.current=null;},[]);
 useEffect(()=>{let active=true;void Promise.resolve().then(()=>{if(active)latest.current.start(job.request);});return()=>{active=false;};},[job]);
 useEffect(()=>{if(operation.phase==='error')settle(false,operation.message);},[operation.phase,operation.message]);
 return operation.phase==='unconfirmed'?<div><p role="status">草稿删除结果待确认</p><p role="alert">{operation.message}</p><div className="workflow-instance-buttons"><button className="admin-button" type="button" onClick={operation.query}>核查草稿结果</button><button className="admin-button" type="button" onClick={operation.retry}>重试原草稿操作</button></div></div>:<p role="status">正在删除草稿…</p>;
}
