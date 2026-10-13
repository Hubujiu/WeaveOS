import {useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {applicationApi,applicationApiEnvelope,ApplicationError} from '../api';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {historyComparison,historyValue,validWorkflowEventDetail,validWorkflowHistory,type HistoricalField,type WorkflowEvent,type WorkflowEventDetail} from './historyContract';
import type {TaskLocation,TaskPreview} from './taskPreviewContract';

type Props={actorId:string;location:Pick<TaskLocation,'appId'|'viewId'|'recordId'>;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onCompare?:(detail:WorkflowEventDetail)=>void};
type PageState={items:WorkflowEvent[];next:string|null;loading:boolean;error:string};
type DetailState={event:WorkflowEvent;data:WorkflowEventDetail|null;loading:boolean;error:string}|null;
const actions:Record<WorkflowEvent['action'],string>={start:'发起申请',agree:'已同意',reject:'已驳回',withdraw:'已撤回',return:'已退回'};
function readError(error:unknown):string{
 if(error instanceof ApplicationError){
  if(error.status===403)return '没有当前记录的历史权限。';
  if(error.status===404)return '这条记录或审批历史已不可读取。';
  if(['APPLICATION_QUERY_CHANGED','APPLICATION_QUERY_CONTEXT_EXPIRED','COMMON_QUERY_CONTEXT_EXPIRED'].includes(error.code))return '历史查询已变化或过期，请刷新历史。';
  return error.message;
 }
 return error instanceof Error?error.message:'无法读取审批历史，请重试。';
}

export function WorkflowHistory(props:Props){
 const {actorId,location}=props;
 const scope=JSON.stringify([actorId,location.appId,location.viewId,location.recordId]);
 return <WorkflowHistoryScope key={scope} {...props}/>;
}
function WorkflowHistoryScope({actorId,location,onUnauthorized,onIdentityMismatch,onCompare}:Props){
 const path=`applications/${location.appId}/forms/${location.viewId}/records/${location.recordId}/workflow-events`;
 const [page,setPage]=useState<PageState>({items:[],next:null,loading:true,error:''});
 const [detail,setDetail]=useState<DetailState>(null);
 const listController=useRef<AbortController|null>(null),detailController=useRef<AbortController|null>(null);
 const listSequence=useRef(0),detailSequence=useRef(0),busy=useRef(false),items=useRef<WorkflowEvent[]>([]);
 const handlers=useRef({onUnauthorized,onIdentityMismatch});handlers.current={onUnauthorized,onIdentityMismatch};
 const handleIdentity=(error:unknown)=>{
  if(error instanceof ApplicationError&&error.status===401)handlers.current.onUnauthorized();
  else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')handlers.current.onIdentityMismatch();
 };
 const clearDetail=()=>{detailController.current?.abort();detailSequence.current++;setDetail(null);};
 const load=useCallback(async(cursor:string|null)=>{
  listController.current?.abort();const controller=new AbortController();listController.current=controller;
  const sequence=++listSequence.current;busy.current=true;
  if(!cursor){items.current=[];clearDetail();}
  setPage(previous=>({items:cursor?previous.items:[],next:null,loading:true,error:''}));
  try{
   await Promise.resolve();if(controller.signal.aborted)return;
   const params=new URLSearchParams({pageSize:'20'});if(cursor)params.set('pageToken',cursor);
   const response=await applicationApiEnvelope<unknown>(actorId,path+'?'+params.toString(),'GET',undefined,controller.signal,200);
   if(controller.signal.aborted||sequence!==listSequence.current)return;
   if(!validWorkflowHistory(response.data,response.meta?.pagination,20))throw new Error('审批历史响应格式不正确');
   const existing=new Set(items.current.map(event=>event.id));
   if(cursor&&response.data.items.some(event=>existing.has(event.id)))throw new Error('审批历史分页重复，请刷新历史。');
   items.current=cursor?[...items.current,...response.data.items]:response.data.items;
   setPage({items:items.current,next:response.meta!.pagination!.nextPageToken,loading:false,error:''});
  }catch(error){
   if(controller.signal.aborted||sequence!==listSequence.current)return;
   items.current=[];clearDetail();setPage({items:[],next:null,loading:false,error:readError(error)});handleIdentity(error);
  }finally{if(sequence===listSequence.current)busy.current=false;}
 },[actorId,path]);
 useEffect(()=>{
  void load(null);
  return()=>{listSequence.current++;detailSequence.current++;listController.current?.abort();detailController.current?.abort();};
 },[load]);
 const open=async(event:WorkflowEvent)=>{
  detailController.current?.abort();const controller=new AbortController();detailController.current=controller;
  const sequence=++detailSequence.current;setDetail({event,data:null,loading:true,error:''});
  try{
   const data=await applicationApi<unknown>(actorId,path+'/'+event.id,'GET',undefined,controller.signal,200);
   if(controller.signal.aborted||sequence!==detailSequence.current)return;
   if(!validWorkflowEventDetail(data,event))throw new Error('审批依据响应格式不正确');
   setDetail({event,data,loading:false,error:''});
  }catch(error){
   if(controller.signal.aborted||sequence!==detailSequence.current)return;
   setDetail({event,data:null,loading:false,error:readError(error)});handleIdentity(error);
  }
 };
 return <section className="app-surface workflow-history-card" aria-label="审批轨迹">
  <div className="workflow-task-section-heading"><h2>审批轨迹</h2><button type="button" className="text-button" disabled={page.loading} onClick={()=>void load(null)}>刷新历史</button></div>
  {page.error?<p role="alert">{page.error}</p>:null}
  {page.items.map(event=><article key={event.id} className="workflow-history-event">
   <h3>{event.flowName}</h3>{event.flowNameSource==='legacy_last_known'&&<span className="workflow-history-meta">最后已知名称</span>}
   <p>{event.outcome==='no_effect'?'操作未执行':actions[event.action]}</p>
   <p className="workflow-history-meta">操作人 {event.actorId}</p>
   {event.nodeId&&<p className="workflow-history-meta">节点 {event.nodeId}</p>}
   {event.targetNodeId&&<p className="workflow-history-meta">目标节点 {event.targetNodeId}</p>}
   <p className="workflow-history-meta"><time dateTime={event.occurredAt}>{new Date(event.occurredAt).toLocaleString('zh-CN',{hour12:false})}</time> · v{event.recordVersion}</p>
   <button type="button" className="text-button" onClick={()=>void open(event)}>查看审批依据</button>
  </article>)}
  {page.loading&&<p role="status">正在读取审批历史…</p>}
  {!page.loading&&!page.error&&!page.items.length&&<p>暂无已确认的审批历史</p>}
  {page.next&&<button type="button" className="admin-button" disabled={page.loading} onClick={()=>{if(!busy.current)void load(page.next);}}>加载更多历史</button>}
  {detail&&<section className="workflow-history-basis" aria-label="历史审批依据">
   <div className="workflow-task-section-heading"><h2>历史审批依据</h2><button type="button" className="text-button" onClick={clearDetail}>关闭依据</button></div>
   {detail.loading?<p role="status">正在读取当时依据…</p>:detail.error?<p role="alert">{detail.error}</p>:detail.data?.basis.status==='unavailable'?<p>当时审批依据不可用</p>:detail.data?<>
    <p className="workflow-history-meta">审批时记录 v{detail.event.recordVersion}</p>
    {detail.data.basis.fields.length?<dl>{detail.data.basis.fields.map(field=><div key={field.fieldId}><dt>{field.fieldName}</dt><dd>{historyValue(field)}</dd></div>)}</dl>:<p>当时可见的审批依据为空</p>}
    {onCompare&&<button type="button" className="admin-button" onClick={()=>onCompare(detail.data!)}>查看版本差异</button>}
   </>:null}
  </section>}
 </section>;
}

function currentField(field:HistoricalField,preview:TaskPreview):HistoricalField|undefined{
 const current=preview.fields.find(item=>item.id===field.fieldId);
 if(!current||!Object.hasOwn(preview.record.values,field.fieldId))return undefined;
 const valueLabels:HistoricalField['valueLabels']={};
 for(const [id,label] of Object.entries(preview.record.referenceDisplays[field.fieldId]??{}))valueLabels[id]={label:label.label,deleted:label.deleted};
 if(current.kind==='single_select'||current.kind==='multi_select')for(const option of current.config.options)if(!valueLabels[option.id])valueLabels[option.id]={label:option.label,deleted:false};
 return {fieldId:current.id,fieldName:current.name,fieldKind:current.kind,value:preview.record.values[current.id],valueLabels};
}
export function WorkflowComparison({detail,preview,onBack}:{detail:WorkflowEventDetail;preview:TaskPreview;onBack:()=>void}){
 const heading=useRef<HTMLHeadingElement|null>(null);
 useLayoutEffect(()=>{heading.current?.scrollIntoView({block:'start',behavior:'instant'});},[]);
 const columns:TableColumn<HistoricalField>[]=[
  {key:'fieldName',header:'字段',dataType:'text',sortable:false,cell:field=><span>{field.fieldName}</span>},
  {key:'before',header:'审批时 v'+detail.event.recordVersion,dataType:'text',sortable:false,cell:field=><span>{historyValue(field)}</span>},
  {key:'after',header:'当前 v'+preview.record.recordVersion,dataType:'text',sortable:false,cell:field=><span>{historyComparison(field,currentField(field,preview)).after}</span>},
 ];
 return <section className="workflow-comparison" aria-label="版本对照">
  <div className="app-page-heading"><h1 ref={heading}>版本差异</h1></div>
  <section className="app-surface workflow-comparison-card">
   <div className="workflow-comparison-versions"><div><span>已审批依据</span><p className="workflow-readonly-value">v{detail.event.recordVersion} · {new Date(detail.event.occurredAt).toLocaleString('zh-CN',{hour12:false})}</p></div><div><span>当前记录</span><p className="workflow-readonly-value">v{preview.record.recordVersion} · {new Date(preview.record.updatedAt).toLocaleString('zh-CN',{hour12:false})}</p></div></div>
   <div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="版本差异" columns={columns} data={detail.basis.fields} getRowId={field=>field.fieldId} rowHeight={56} minWidth={660} emptyState="当时可见的审批依据为空"/></div>
   <p className="workflow-history-meta">仅对照当前可读取的已保存记录；未保存修改不参与对照。</p>
   <div className="workflow-comparison-back"><button type="button" className="admin-button" onClick={onBack}>返回审批处理</button></div>
  </section>
 </section>;
}
