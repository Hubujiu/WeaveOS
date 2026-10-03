import {useCallback,useEffect,useRef,useState} from 'react';
import {Modal} from '../Modal';
import {applicationApi,applicationApiEnvelope,ApplicationError,type ApplicationEnvelope} from './api';
import type {RegisterLeaveGuard} from './forms';
import {RecordWorkspace} from './RecordWorkspace';
import {RecordForm} from './records/RecordForm';
import type {MutationResult,RecordItem,RuntimeField,RuntimeView,UUID} from './records/contracts';
import {parseRecordItem,parseRuntimeView} from './recordReadContracts';
import {createNewRecordIdentity,type RecordEditorIdentity} from './records/recordState';
import {projectRuntimeFields,scopeAllows} from './records/runtimeModel';
import {scopedUnconfirmed} from './recovery';
import type {LoadReferenceCandidates,ReferenceCandidate} from './forms';

const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
type Editor={identity:RecordEditorIdentity;view:RuntimeView;record?:RecordItem;mode:'create'|'edit'|'read';queryVersion?:string};
type PendingRead={identity:RecordEditorIdentity;view:RuntimeView|null;recordId:UUID;queryVersion?:string;mode:'read'|'edit';confirmedWrite:boolean};

export function RecordRoute({actorId,appId,viewId,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:{
 actorId:string;appId:string;viewId:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;
 registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;
}){
 const [editor,setEditor]=useState<Editor|null>(null),[pendingRead,setPendingRead]=useState<PendingRead|null>(null);
 const [readError,setReadError]=useState(''),[saved,setSaved]=useState(false),[workspaceRefresh,setWorkspaceRefresh]=useState(0);
 const generation=useRef(0),openedRecovery=useRef(new Set<string>()),readController=useRef<AbortController|null>(null),originFocus=useRef<HTMLElement|null>(null);
 useEffect(()=>()=>{generation.current++;readController.current?.abort();},[]);
 const closeEditor=useCallback(()=>{generation.current++;readController.current?.abort();setEditor(null);setPendingRead(null);setReadError('');onDirtyChange(false);},[onDirtyChange]);
 const captureOriginFocus=()=>{originFocus.current=document.activeElement instanceof HTMLElement?document.activeElement:null;};
 const guardedClose=()=>requestSectionLeave(()=>{
  const target=originFocus.current;
  closeEditor();
  // When the Shell discard dialog and record dialog close in one React commit,
  // the nested native-dialog cleanups can otherwise leave focus on <body>.
  if(target)requestAnimationFrame(()=>{if(target.isConnected)target.focus();});
 });
 const handleError=(error:unknown)=>{
  if(error instanceof ApplicationError&&error.status===401)onUnauthorized();
  else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
 };
 const readRecord=useCallback(async(view:RuntimeView,recordId:UUID,queryVersion?:string,identity?:RecordEditorIdentity,mode:'read'|'edit'='read')=>{
  captureOriginFocus();
  const current=++generation.current;
  readController.current?.abort();const controller=new AbortController();readController.current=controller;
  const nextIdentity=identity??{kind:'record' as const,actorId,appId,viewId,recordId};
  setReadError('');setPendingRead({identity:nextIdentity,view,recordId,queryVersion,mode,confirmedWrite:false});
  try{
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${encodeURIComponent(recordId)}`,'GET',undefined,controller.signal);
   const record=parseRecordItem(raw,view,actorId);
   if(record.id!==recordId)throw new Error('记录身份不匹配');
   if(current!==generation.current)return;
   setPendingRead(null);setEditor({identity:nextIdentity,view,record,mode,queryVersion});
  }catch(error){if(current!==generation.current||controller.signal.aborted)return;handleError(error);setPendingRead({identity:nextIdentity,view,recordId,queryVersion,mode,confirmedWrite:false});setReadError('无法读取记录，请重试读取记录');}
 },[actorId,appId,viewId,onUnauthorized,onIdentityMismatch]);
 const openNew=(view:RuntimeView,queryVersion?:string)=>{captureOriginFocus();setSaved(false);setReadError('');setEditor({identity:createNewRecordIdentity(actorId,appId,viewId),view,mode:'create',queryVersion});};
 const openRow=(view:RuntimeView,row:RecordItem,queryVersion:string)=>{
  if(view.capabilities.read==='none')return;
  setSaved(false);void readRecord(view,row.id,queryVersion);
 };
 const refreshRead=useCallback(async(identity:RecordEditorIdentity,recordId:UUID,confirmedWrite:boolean)=>{
  setWorkspaceRefresh(value=>value+1);
  const current=++generation.current;readController.current?.abort();const controller=new AbortController();readController.current=controller;
  setEditor(null);setPendingRead({identity,view:null,recordId,mode:'read',confirmedWrite});setReadError('');
  try{
   const runtime=parseRuntimeView(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal),{appId,viewId});
   if(current!==generation.current)return;
   if(runtime.capabilities.read==='none'){setPendingRead(null);setSaved(confirmedWrite);onDirtyChange(false);return;}
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${encodeURIComponent(recordId)}`,'GET',undefined,controller.signal);
   const record=parseRecordItem(raw,runtime,actorId);
   if(record.id!==recordId)throw new Error('记录身份不匹配');
   if(current!==generation.current)return;
   setPendingRead(null);setEditor({identity,view:runtime,record,mode:'read'});setSaved(confirmedWrite);onDirtyChange(false);
  }catch(error){if(current!==generation.current||controller.signal.aborted)return;handleError(error);setPendingRead({identity,view:null,recordId,mode:'read',confirmedWrite});setReadError(confirmedWrite?'记录已保存，但无法读取记录；请重试读取记录':'无法读取记录，请重试读取记录');setSaved(confirmedWrite);onDirtyChange(false);}
 },[actorId,appId,viewId,onUnauthorized,onIdentityMismatch,onDirtyChange]);
 const onConfirmed=useCallback((result:MutationResult,identity:RecordEditorIdentity)=>{
  setSaved(true);onDirtyChange(false);
  void refreshRead(identity,result.id,true);
 },[onDirtyChange,refreshRead]);
 const editAllowed=(value:Editor)=>!!value.record&&scopeAllows(value.view.capabilities.edit,actorId,value.record.createdBy)&&projectRuntimeFields(value.view,'edit',actorId,value.record).some(field=>field.editable);
 const loadCandidates=useCallback<NonNullable<Parameters<typeof RecordForm>[0]['loadCandidates']>>(async(field:RuntimeField,request,signal)=>{
  const current=editor;
  if(!current||current.mode==='read'||!['member','department'].includes(field.kind))return {items:[],nextPageToken:null};
  const projected=projectRuntimeFields(current.view,current.mode,actorId,current.record).find(item=>item.field.id===field.id);
  if(!projected?.editable)return {items:[],nextPageToken:null};
  const action=current.mode==='edit'?'edit':'create';
  const params=new URLSearchParams({fieldId:field.id,action,q:request.q,pageSize:String(request.pageSize)});
  if(request.pageToken)params.set('pageToken',request.pageToken);
  if(action==='edit'&&current.identity.kind==='record')params.set('recordId',current.identity.recordId);
  let envelope:ApplicationEnvelope<unknown>;
  try{envelope=await applicationApiEnvelope<unknown>(actorId,`applications/${appId}/forms/${viewId}/reference-candidates?${params.toString()}`,'GET',undefined,signal);}
  catch(error){handleError(error);throw error;}
  const data=envelope.data as {items?:unknown};const pagination=envelope.meta?.pagination;
  if(!data||!Array.isArray(data.items)||data.items.length>request.pageSize||!pagination||typeof pagination.hasMore!=='boolean'||!(pagination.nextPageToken===null||typeof pagination.nextPageToken==='string')||pagination.hasMore&&!pagination.nextPageToken||!pagination.hasMore&&pagination.nextPageToken!==null)throw new Error('候选项响应格式不正确');
  const items:ReferenceCandidate[]=data.items.map(value=>{
   if(!value||typeof value!=='object')throw new Error('候选项响应格式不正确');
   const item=value as Record<string,unknown>;
   if(typeof item.id!=='string'||!uuid.test(item.id)||item.status!=='active'||typeof item.label!=='string'||!item.label.trim())throw new Error('候选项响应格式不正确');
   return {id:item.id,label:item.label,status:'active'};
  });
  if(new Set(items.map(item=>item.id)).size!==items.length)throw new Error('候选项响应格式不正确');
  return {items,nextPageToken:pagination.nextPageToken};
 },[actorId,appId,viewId,editor]);
 const recoverPacket=useCallback((view:RuntimeView)=>{
  const packet=scopedUnconfirmed(actorId).find(value=>value.resource?.kind==='record'&&value.resource.appId===appId&&value.resource.viewId===viewId&&
   (!!value.resource.id||!!value.resource.creationNonce));
  if(!packet||!packet.resource)return;
  const resource=packet.resource,token=JSON.stringify([resource,packet.operationId]);
  if(openedRecovery.current.has(token))return;
  openedRecovery.current.add(token);setSaved(false);setReadError('');
  if(resource.id&&uuid.test(resource.id)&&view.capabilities.read!=='none'){
   const identity:RecordEditorIdentity={kind:'record',actorId,appId,viewId,recordId:resource.id};
   void readRecord(view,resource.id,undefined,identity,packet.method==='PATCH'?'edit':'read');
  }else if(resource.id&&uuid.test(resource.id)){
   const identity:RecordEditorIdentity={kind:'record',actorId,appId,viewId,recordId:resource.id};
   setEditor({identity,view,mode:'edit'});
  }else if(resource.creationNonce&&uuid.test(resource.creationNonce)){
   const identity:RecordEditorIdentity={kind:'new',actorId,appId,viewId,clientDraftId:resource.creationNonce};
   setEditor({identity,view,mode:'create'});
  }
 },[actorId,appId,viewId,readRecord]);
 // RecordWorkspace already verified and parsed this runtime. Its lifecycle owns
 // cancellation; this callback only offers a matching immutable unknown write.
 const runtimeReady=useCallback((view:RuntimeView)=>recoverPacket(view),[recoverPacket]);
 const refreshEditor=()=>{
  if(!editor)return;
  if(editor.record){requestSectionLeave(()=>{setSaved(false);void refreshRead(editor.identity,editor.record!.id,false);});return;}
  requestSectionLeave(()=>{
   setEditor(null);setPendingRead(null);setReadError('');setSaved(false);onDirtyChange(false);
   setWorkspaceRefresh(value=>value+1);
  });
 };
 const canRetryRead=!!pendingRead&&!!readError;
 return <section className="record-route" aria-label="记录路由">
  {saved&&<p className="record-saved" role="status">记录已保存</p>}
  <RecordWorkspace key={`${actorId}:${appId}:${viewId}:${workspaceRefresh}`} actorId={actorId} appId={appId} viewId={viewId}
   onCreate={openNew} onOpenRecord={openRow} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onRuntimeReady={runtimeReady}/>
  {(editor||pendingRead)&&<Modal title={editor?.mode==='create'?'新建记录':'记录详情'} onClose={guardedClose}>
   {pendingRead&&!readError&&<p role="status">正在读取记录…</p>}
   {readError&&<><p role="alert">{readError}</p>{canRetryRead&&<button type="button" className="admin-button" onClick={()=>{const target=pendingRead!;setReadError('');if(target.view)void readRecord(target.view,target.recordId,target.queryVersion,target.identity,target.mode);else void refreshRead(target.identity,target.recordId,target.confirmedWrite);}}>重试读取记录</button>}</>}
   {editor?.mode==='read'&&<div className="record-route-actions">{editAllowed(editor)&&<button type="button" className="admin-button" onClick={()=>{setSaved(false);setEditor({...editor,mode:'edit',queryVersion:editor.queryVersion});}}>编辑记录</button>}</div>}
   {editor&&<RecordForm key={JSON.stringify([editor.identity,editor.mode,editor.view.policyRevision,editor.view.schemaVersion,editor.view.viewVersion,editor.record?.recordVersion??'new'])} view={editor.view} identity={editor.identity} record={editor.record} mode={editor.mode} authorityKey={`${editor.view.policyRevision}:${editor.view.schemaVersion}:${editor.view.viewVersion}:${editor.record?.recordVersion??'new'}`}
    queryVersion={editor.queryVersion} loadCandidates={loadCandidates} onConfirmed={onConfirmed} onDirtyChange={onDirtyChange} onRequestDiscard={guardedClose} onDiscard={closeEditor}
    onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} registerLeaveGuard={registerLeaveGuard}
    onRefresh={refreshEditor}/>}</Modal>}
 </section>;
}
