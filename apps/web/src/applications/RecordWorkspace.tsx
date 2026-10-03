import {useCallback,useEffect,useRef,useState} from 'react';
import {applicationApi,applicationReadPost,ApplicationError} from './api';
import {RecordsPanel} from './records/RecordsPanel';
import type {RecordItem,RecordPage,RecordSort,RuntimeView} from './records/contracts';
import {parseRecordPage,parseRuntimeView} from './recordReadContracts';
import {useResourceQuery} from '../usePersonnelQuery';

export type RecordWorkspaceProps={
 actorId:string;appId:string;viewId:string;
 onCreate:(view:RuntimeView,queryVersion?:string)=>void;
 onOpenRecord:(view:RuntimeView,record:RecordItem,queryVersion:string)=>void;
 onRuntimeReady?:(view:RuntimeView)=>void;
 onUnauthorized:()=>void;onIdentityMismatch:()=>void;
};
type RuntimeState={scope:string;generation:number;view:RuntimeView|null;error:string|null;errorCode:string|null;loading:boolean};
const EMPTY_PAGE:RecordPage={items:[],total:0,page:1,pageSize:20,sort:null,queryVersion:'',schemaVersion:0,viewVersion:0};
const genericError='无法读取记录，请重试';

export function RecordWorkspace(props:RecordWorkspaceProps){
 const {actorId,appId,viewId}=props;
 const scope=JSON.stringify([actorId,appId,viewId]);
 const active=useRef(scope);active.current=scope;
 const generation=useRef(0),[reload,setReload]=useState(0);
 const [state,setState]=useState<RuntimeState>({scope,generation:0,view:null,error:null,errorCode:null,loading:true});
 const report=useCallback((error:unknown,expected:string)=>{
  if(active.current!==expected)return;
  if(error instanceof ApplicationError&&error.status===401)props.onUnauthorized();
  else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')props.onIdentityMismatch();
 },[props.onUnauthorized,props.onIdentityMismatch]);
 useEffect(()=>{
  const current=++generation.current,controller=new AbortController();
  setState({scope,generation:current,view:null,error:null,errorCode:null,loading:true});
  applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal)
   .then(raw=>{const view=parseRuntimeView(raw,{appId,viewId});if(!controller.signal.aborted&&active.current===scope&&generation.current===current)setState({scope,generation:current,view,error:null,errorCode:null,loading:false});})
   .catch(error=>{if(controller.signal.aborted||active.current!==scope||generation.current!==current)return;report(error,scope);setState({scope,generation:current,view:null,error:genericError,errorCode:error instanceof ApplicationError?error.code:null,loading:false});});
  return()=>{controller.abort();};
 },[actorId,appId,viewId,scope,reload,report]);
 const current=state.scope===scope?state:null;
 useEffect(()=>{if(current?.view)props.onRuntimeReady?.(current.view);},[current?.view,props.onRuntimeReady]);
 const refresh=()=>{setState(old=>old.scope===scope?{...old,view:null,error:null,errorCode:null,loading:true}:old);setReload(value=>value+1);};
 if(!current||current.loading)return <section className="record-workspace" aria-label="记录列表"><p role="status">正在读取记录…</p></section>;
 if(!current.view)return <section className="record-workspace" aria-label="记录列表"><p role="alert">{current.errorCode==='APPLICATION_SCHEMA_NOT_READY'?'表单尚未配置，请先完成表单配置。':current.error??genericError}</p><button className="admin-button" type="button" onClick={refresh}>重试</button></section>;
 const view=current.view;
 const createOnly=view.capabilities.create&&view.capabilities.read==='none'&&!view.capabilities.search;
 if(createOnly)return <section className="record-workspace" aria-label="记录列表"><button className="admin-button" type="button" onClick={()=>props.onCreate(view)}>新建记录</button></section>;
 if(view.capabilities.read==='none'||!view.capabilities.search)return <section className="record-workspace" aria-label="记录列表"><p>当前视图没有可读取的记录</p>{view.capabilities.create&&<button className="admin-button" type="button" onClick={()=>props.onCreate(view)}>新建记录</button>}</section>;
 const updateRuntime=(next:RuntimeView)=>setState(old=>old.scope===scope&&old.generation===generation.current?{...old,view:next}:old);
 return <RecordQuery key={`${scope}:${reload}`} {...props} scope={scope} view={view} onRefresh={refresh} onRuntimeUpdate={updateRuntime}/>;
}

function RecordQuery(props:RecordWorkspaceProps&{scope:string;view:RuntimeView;onRefresh:()=>void;onRuntimeUpdate:(view:RuntimeView)=>void}){
 const {actorId,appId,viewId,view,scope}=props;
 const [parameters,setParameters]=useState({page:1,pageSize:20,filter:null as null,sort:null as RecordSort});
 const [selectedRowIds,setSelectedRowIds]=useState<string[]>([]);
 const [columnWidths,setColumnWidths]=useState<Record<string,number>>({});
 const [columnOrder,setColumnOrder]=useState<string[]>([]);
 const active=useRef(scope);active.current=scope;
 const viewRef=useRef(view);viewRef.current=view;
 const parsedPage=useRef<RecordPage|null>(null);
 const runtimeBusy=useRef(false);
 const fetchRuntime=useCallback(async(signal:AbortSignal)=>{
  if(runtimeBusy.current)throw new Error(genericError);
  runtimeBusy.current=true;
  try{return parseRuntimeView(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,signal),{appId,viewId});}
  finally{runtimeBusy.current=false;}
 },[actorId,appId,viewId]);
 const transport=useCallback(async(body:{page:number;pageSize:number;filter:null;sort:RecordSort;queryVersion?:string},signal:AbortSignal)=>{
  // StrictMode replays an effect's setup/cleanup synchronously. Defer the
  // initial fetch one microtask so the aborted setup never reaches the API.
  await Promise.resolve();
  if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  let raw=await applicationReadPost<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/search`,body,signal);
  const versionMismatch=!!raw&&typeof raw==='object'&&('schemaVersion'in raw)&&('viewVersion'in raw)&&((raw as any).schemaVersion!==viewRef.current.schemaVersion||(raw as any).viewVersion!==viewRef.current.viewVersion);
  if(versionMismatch){const fresh=await fetchRuntime(signal);if(active.current!==scope||signal.aborted)throw new Error(genericError);viewRef.current=fresh;props.onRuntimeUpdate(fresh);if(!raw||typeof raw!=='object'||(raw as any).schemaVersion!==fresh.schemaVersion||(raw as any).viewVersion!==fresh.viewVersion)throw new Error(genericError);}
  const page=parseRecordPage(raw,viewRef.current,actorId,body);
  parsedPage.current=page;
  return page;
 },[actorId,appId,viewId,scope,fetchRuntime,props.onRuntimeUpdate]);
 const normalize=(error:unknown)=>error instanceof ApplicationError?error:new Error(genericError);
 const query=useResourceQuery<RecordPage,typeof parameters,Error>(scope,parameters,{...EMPTY_PAGE,page:parameters.page,pageSize:parameters.pageSize},transport,normalize);
 useEffect(()=>{if(query.error instanceof ApplicationError&&query.error.status===401)props.onUnauthorized();else if(query.error instanceof ApplicationError&&query.error.code==='AUTH_SESSION_CHANGED')props.onIdentityMismatch();},[query.error,props.onUnauthorized,props.onIdentityMismatch]);
 const message=query.error instanceof ApplicationError&&(query.error.code==='APPLICATION_QUERY_CHANGED'||query.error.code==='APPLICATION_QUERY_CONTEXT_EXPIRED')?'记录上下文已变化，请刷新后重试':query.error?.message??null;
 const disabled=!!query.error||query.loading;
 return <section className="record-workspace" aria-label="记录工作区">
  <div className="table-toolbar"><button className="admin-button" type="button" onClick={props.onRefresh}>刷新</button>{view.capabilities.create&&<button className="admin-button" type="button" disabled={disabled} onClick={()=>props.onCreate(view,query.data.queryVersion||undefined)}>新建记录</button>}</div>
  <fieldset className="record-table-frame" disabled={disabled}><legend className="sr-only">记录操作</legend><RecordsPanel view={viewRef.current} actorId={actorId} page={query.data} loading={query.loading} error={message} hiddenColumnIds={[]} columnWidths={columnWidths} columnOrder={columnOrder} selectedRowIds={selectedRowIds} onSelectionChange={setSelectedRowIds} onPageChange={page=>setParameters(current=>({...current,page}))} onPageSizeChange={pageSize=>{setSelectedRowIds([]);setParameters(current=>({...current,page:1,pageSize}));}} onSortChange={sort=>{setSelectedRowIds([]);setParameters(current=>({...current,page:1,sort}));}} onColumnWidthsChange={setColumnWidths} onColumnOrderChange={setColumnOrder} onOpenRecord={record=>{if(disabled)return;void (async()=>{try{
   const accept=await query.prepareChange(parameters);
   if(!accept||!accept())return;
   const accepted=parsedPage.current;
   const row=accepted?.items.find(item=>item.id===record.id);
   if(!accepted?.queryVersion||!row)return;
   props.onOpenRecord(viewRef.current,row,accepted.queryVersion);
  }catch{/* The query retains the old page and exposes the stale-context retry state. */}})();}}/></fieldset>
 </section>;
}
