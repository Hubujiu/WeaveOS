import {useEffect,useRef,useState} from 'react';
import {Modal} from '../../Modal';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {applicationApiEnvelope,ApplicationError} from '../api';
import type {HistoryChange,HistoryEvent} from './contracts';
import {recordHistoryValue,validRecordHistory} from './historyContract';
type Props={actorId:string;appId:string;viewId:string;recordId:string;onClose:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void};
export function RecordHistoryPanel(props:Props){return <HistoryScope key={JSON.stringify([props.actorId,props.appId,props.viewId,props.recordId])} {...props}/>;}
function HistoryScope({actorId,appId,viewId,recordId,onClose,onUnauthorized,onIdentityMismatch}:Props){
 const [token,setToken]=useState<string|null>(null),[epoch,setEpoch]=useState(0),[state,setState]=useState<{items:HistoryEvent[];next:string|null;loading:boolean;error:string}>({items:[],next:null,loading:true,error:''});
 const handlers=useRef({onUnauthorized,onIdentityMismatch});handlers.current={onUnauthorized,onIdentityMismatch};
 const path=`applications/${appId}/forms/${viewId}/records/${recordId}/history`;
 useEffect(()=>{
  const controller=new AbortController();let active=true;setState({items:[],next:null,loading:true,error:''});
  void(async()=>{await Promise.resolve();if(controller.signal.aborted)return;try{
   const query=new URLSearchParams({pageSize:'20'});if(token)query.set('pageToken',token);
   const reply=await applicationApiEnvelope<unknown>(actorId,path+'?'+query,'GET',undefined,controller.signal,200);
   if(!active||controller.signal.aborted)return;if(!validRecordHistory(reply.data,reply.meta?.pagination,20))throw new Error('记录保存历史响应格式不正确');
   setState({items:reply.data.items,next:reply.meta!.pagination!.nextPageToken,loading:false,error:''});
  }catch(error){if(!active||controller.signal.aborted)return;const message=error instanceof ApplicationError&&error.status===403?'没有当前记录的历史权限。':error instanceof ApplicationError&&error.status===404?'记录或保存历史已不可读取。':error instanceof Error?error.message:'无法读取记录保存历史';setState({items:[],next:null,loading:false,error:message});if(error instanceof ApplicationError&&error.status===401)handlers.current.onUnauthorized();else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')handlers.current.onIdentityMismatch();}})();
  return()=>{active=false;controller.abort();};
 },[actorId,path,token,epoch]);
 const columns:TableColumn<HistoryChange>[]=[
  {key:'fieldLabel',header:'字段',sortable:false,cell:change=><span>{change.fieldLabel}{change.fieldDeleted?' · 字段已删除':''}</span>},
  {key:'before',header:'保存前',sortable:false,cell:change=><span>{recordHistoryValue(change,'before')}</span>},
  {key:'after',header:'保存后',sortable:false,cell:change=><span>{recordHistoryValue(change,'after')}</span>},
 ];
 return <Modal title="记录保存历史" onClose={onClose}>
  <div className="record-history-panel"><div className="workflow-instance-buttons"><button type="button" className="admin-button" disabled={state.loading} onClick={()=>{setToken(null);setEpoch(v=>v+1);}}>刷新历史</button></div>
  <p className="workflow-history-meta">仅显示当前允许查看的保存变更。标签为当前或最后已知名称；结构转换不提供完整历史快照。</p>
  {state.loading&&<p role="status">正在读取记录保存历史…</p>}{state.error&&<p role="alert">{state.error} 请刷新历史重试。</p>}
  {!state.loading&&!state.error&&!state.items.length&&<p>暂无可读取的保存历史</p>}
  {state.items.map(event=><article key={event.id} className="record-history-event"><h3>{event.origin==='task_save'?'节点保存':'普通保存'} · v{event.recordVersionBefore} → v{event.recordVersionAfter}</h3><p className="workflow-history-meta">操作人 {event.actorId} · <time dateTime={event.occurredAt}>{new Date(event.occurredAt).toLocaleString('zh-CN',{hour12:false})}</time></p><div className="arca-source personnel-source-scope"><Table className="personnel-source-table" ariaLabel={'记录变更 '+event.id} columns={columns} data={event.changes} getRowId={change=>change.fieldId} rowHeight={72} minWidth={640}/></div></article>)}
  {state.next&&<button type="button" className="admin-button" disabled={state.loading} onClick={()=>setToken(state.next)}>下一批历史</button>}
  </div>
 </Modal>;
}
