import {useCallback,useEffect,useRef,useState} from 'react';
import {applicationApi,applicationApiEnvelope,ApplicationError,type ApplicationEnvelope} from './api';
import type {RegisterLeaveGuard} from './forms';
import {RecordWorkspace} from './RecordWorkspace';
import {RecordWorkflows} from './workflows/RecordWorkflows';
import {findRoundRecovery} from './workflows/roundRecovery';
import {reworkRuntimeView} from './workflows/reworkView';
import {validRoundPreview} from './workflows/roundContract';
import {RecordHistoryPanel} from './records/RecordHistoryPanel';
import {DraftsPanel} from './records/DraftsPanel';
import {RecordForm} from './records/RecordForm';
import type {Draft,DraftMutationResult,MutationResult,RecordItem,RuntimeField,RuntimeView,UUID} from './records/contracts';
import {validDraft,projectDraftFields} from './records/draftContract';
import {parseRecordItem,parseRuntimeView} from './recordReadContracts';
import {createNewRecordIdentity,type RecordEditorIdentity} from './records/recordState';
import {projectRuntimeFields,scopeAllows} from './records/runtimeModel';
import {findDraftRecovery} from './records/draftRecovery';
import {scopedUnconfirmed,type ApplicationPacket} from './recovery';
import type {LoadReferenceCandidates,ReferenceCandidate} from './forms';

const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
type Editor={identity:RecordEditorIdentity;view:RuntimeView;record?:RecordItem;draft?:Draft;recoveredDraft?:ApplicationPacket;mode:'create'|'edit'|'read';queryVersion?:string;reworkInstanceId?:string};
type PendingRead={identity:RecordEditorIdentity;view:RuntimeView|null;recordId:UUID;queryVersion?:string;mode:'read'|'edit';confirmedWrite:boolean;recoveredDraft?:ApplicationPacket};

export function RecordRoute({actorId,appId,viewId,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:{
 actorId:string;appId:string;viewId:string;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;
 registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;
}){
 const [createActionHost,setCreateActionHost]=useState<HTMLDivElement|null>(null),[historyRecord,setHistoryRecord]=useState<string|null>(null);
 const [editor,setEditor]=useState<Editor|null>(null),[pendingRead,setPendingRead]=useState<PendingRead|null>(null);
 const [draftsView,setDraftsView]=useState<RuntimeView|null>(null),[draftListDirty,setDraftListDirty]=useState(false);
 const [recordDirty,setRecordDirty]=useState(false),[workflowDirty,setWorkflowDirty]=useState(false),[presetDirty,setPresetDirty]=useState(false);
 useEffect(()=>{onDirtyChange(recordDirty||workflowDirty||presetDirty||draftListDirty);return()=>onDirtyChange(false);},[recordDirty,workflowDirty,presetDirty,draftListDirty,onDirtyChange]);
 const [reworkSaved,setReworkSaved]=useState(false);
 const [draftSaved,setDraftSaved]=useState(false),[pendingDraftRead,setPendingDraftRead]=useState<{id:string;confirmed:boolean;recoveredDraft?:ApplicationPacket}|null>(null);
 const [readError,setReadError]=useState(''),[saved,setSaved]=useState(false),[workspaceRefresh,setWorkspaceRefresh]=useState(0);
 const generation=useRef(0),openedRecovery=useRef(new Set<string>()),readController=useRef<AbortController|null>(null),originFocus=useRef<HTMLElement|null>(null);
 useEffect(()=>()=>{generation.current++;readController.current?.abort();},[]);
 const closeEditor=useCallback(()=>{generation.current++;readController.current?.abort();setEditor(null);setPendingRead(null);setPendingDraftRead(null);setReadError('');setRecordDirty(false);},[onDirtyChange]);
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
 const readRecord=useCallback(async(view:RuntimeView,recordId:UUID,queryVersion?:string,identity?:RecordEditorIdentity,mode:'read'|'edit'='read',recoveredDraft?:ApplicationPacket)=>{
  captureOriginFocus();
  const current=++generation.current;
  readController.current?.abort();const controller=new AbortController();readController.current=controller;
  const nextIdentity=identity??{kind:'record' as const,actorId,appId,viewId,recordId};
  setReadError('');setPendingRead({identity:nextIdentity,view,recordId,queryVersion,mode,confirmedWrite:false,recoveredDraft});
  try{
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${encodeURIComponent(recordId)}`,'GET',undefined,controller.signal);
   const record=parseRecordItem(raw,view,actorId);
   if(record.id!==recordId)throw new Error('记录身份不匹配');
   if(current!==generation.current)return;
   setPendingRead(null);setEditor({identity:nextIdentity,view,record,mode,queryVersion,recoveredDraft});
  }catch(error){if(current!==generation.current||controller.signal.aborted)return;handleError(error);setPendingRead({identity:nextIdentity,view,recordId,queryVersion,mode,confirmedWrite:false,recoveredDraft});setReadError('无法读取记录，请重试读取记录');}
 },[actorId,appId,viewId,onUnauthorized,onIdentityMismatch]);
 const openNew=(view:RuntimeView,queryVersion?:string)=>{captureOriginFocus();setDraftSaved(false);setPendingDraftRead(null);setSaved(false);setReworkSaved(false);setReadError('');setEditor({identity:createNewRecordIdentity(actorId,appId,viewId),view,mode:'create',queryVersion});};
 const openRow=(view:RuntimeView,row:RecordItem,queryVersion:string)=>{
  if(view.capabilities.read==='none')return;
  setDraftSaved(false);setPendingDraftRead(null);setSaved(false);setReworkSaved(false);void readRecord(view,row.id,queryVersion);
 };
 const refreshRead=useCallback(async(identity:RecordEditorIdentity,recordId:UUID,confirmedWrite:boolean)=>{
  setWorkspaceRefresh(value=>value+1);
  const current=++generation.current;readController.current?.abort();const controller=new AbortController();readController.current=controller;
  setEditor(null);setPendingRead({identity,view:null,recordId,mode:'read',confirmedWrite});setReadError('');
  try{
   const runtime=parseRuntimeView(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal),{appId,viewId});
   if(current!==generation.current)return;
   if(runtime.capabilities.read==='none'){setPendingRead(null);setSaved(confirmedWrite);setRecordDirty(false);return;}
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${encodeURIComponent(recordId)}`,'GET',undefined,controller.signal);
   const record=parseRecordItem(raw,runtime,actorId);
   if(record.id!==recordId)throw new Error('记录身份不匹配');
   if(current!==generation.current)return;
   setPendingRead(null);setEditor({identity,view:runtime,record,mode:'read'});setSaved(confirmedWrite);setRecordDirty(false);
  }catch(error){if(current!==generation.current||controller.signal.aborted)return;handleError(error);setPendingRead({identity,view:null,recordId,mode:'read',confirmedWrite});setReadError(confirmedWrite?'记录已保存，但无法读取记录；请重试读取记录':'无法读取记录，请重试读取记录');setSaved(confirmedWrite);setRecordDirty(false);}
 },[actorId,appId,viewId,onUnauthorized,onIdentityMismatch,onDirtyChange]);
 const onConfirmed=useCallback((result:MutationResult,identity:RecordEditorIdentity)=>{
  setDraftSaved(false);setPendingDraftRead(null);setSaved(true);setReworkSaved(!!editor?.reworkInstanceId);setRecordDirty(false);
  if(editor?.draft&&editor.view.capabilities.read==='none'){closeEditor();return;}
  void refreshRead(identity,result.id,true);
 },[onDirtyChange,refreshRead,editor,closeEditor]);
 const readDraft=useCallback(async(draftId:string,confirmedWrite:boolean,recoveredDraft?:ApplicationPacket)=>{
  const current=++generation.current;readController.current?.abort();const controller=new AbortController();readController.current=controller;
  setEditor(null);setPendingRead(null);setPendingDraftRead({id:draftId,confirmed:confirmedWrite,recoveredDraft});setReadError('');setRecordDirty(false);
  try{
   const runtime=parseRuntimeView(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal),{appId,viewId});
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/drafts/${draftId}`,'GET',undefined,controller.signal);
   if(!validDraft(raw,viewId,runtime.tableId,draftId,runtime.fields.map(field=>field.id)))throw new Error('草稿响应格式不正确');
   const mode=raw.targetRecordId?'edit':'create';
   let record:RecordItem|undefined;
   if(raw.targetRecordId&&runtime.capabilities.read!=='none'){record=parseRecordItem(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${raw.targetRecordId}`,'GET',undefined,controller.signal),runtime,actorId);if(record.id!==raw.targetRecordId)throw new Error('草稿基础记录不匹配');}
   const fields=projectDraftFields(runtime,raw,actorId,record).filter(port=>port.editable).map(port=>port.field.id);
   if(!validDraft(raw,viewId,runtime.tableId,draftId,fields))throw new Error('草稿字段权限已变化');
   if(current!==generation.current||controller.signal.aborted)return;
   setPendingDraftRead(null);setEditor({identity:{kind:'draft',actorId,appId,viewId,draftId},view:runtime,record,draft:raw,mode,recoveredDraft});
  }catch(error){if(current!==generation.current||controller.signal.aborted)return;handleError(error);setReadError(confirmedWrite?'草稿已保存，但无法读取草稿；请重试读取草稿':'无法读取草稿，请重试读取草稿');}
 },[actorId,appId,viewId,onUnauthorized,onIdentityMismatch]);
 const onDraftConfirmed=useCallback((result:DraftMutationResult)=>{setDraftSaved(true);setSaved(false);setReworkSaved(false);setRecordDirty(false);void readDraft(result.id,true);},[readDraft]);
 const editAllowed=(value:Editor)=>!!value.record&&scopeAllows(value.view.capabilities.edit,actorId,value.record.createdBy)&&projectRuntimeFields(value.view,'edit',actorId,value.record).some(field=>field.editable);
 const loadCandidates=useCallback<NonNullable<Parameters<typeof RecordForm>[0]['loadCandidates']>>(async(field:RuntimeField,request,signal)=>{
  const current=editor;
  if(!current||current.mode==='read'||!['member','department'].includes(field.kind))return {items:[],nextPageToken:null};
  const projected=projectRuntimeFields(current.view,current.mode,actorId,current.record).find(item=>item.field.id===field.id);
  if(!projected?.editable)return {items:[],nextPageToken:null};
  const action=current.mode==='edit'?'edit':'create';
  const params=new URLSearchParams({fieldId:field.id,action,q:request.q,pageSize:String(request.pageSize)});
  if(request.pageToken)params.set('pageToken',request.pageToken);
  if(action==='edit'&&current.record)params.set('recordId',current.record.id);
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
 const readRework=useCallback((instanceId:string,identity:RecordEditorIdentity,recordId:string,recovering=false)=>{
  const current=++generation.current;
  readController.current?.abort();const controller=new AbortController();readController.current=controller;
  setEditor(null);setReadError('');setSaved(false);setReworkSaved(false);setPendingRead({identity,view:null,recordId,mode:'edit',confirmedWrite:false});
  void(async()=>{try{
   const runtime=parseRuntimeView(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal),{appId,viewId});
   const record=parseRecordItem(await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${recordId}`,'GET',undefined,controller.signal),runtime,actorId);
   const preview=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/records/${recordId}/workflows/${instanceId}/round-actions`,'GET',undefined,controller.signal,200);
   if(current!==generation.current||controller.signal.aborted)return;
   if(record.id!==recordId||!validRoundPreview(preview,instanceId))throw new Error('返工资格或记录身份已变化，请刷新后重试');
   const view=reworkRuntimeView(runtime,record,preview,actorId)??(recovering?{...runtime,fields:runtime.fields.map(field=>({...field,access:{...field.access,edit:'none' as const}}))}:null);
   if(!view)throw new Error('当前返工资格、字段权限或记录版本已变化，请刷新后重试');
   setPendingRead(null);setRecordDirty(false);setEditor({identity,view,record,mode:'edit',reworkInstanceId:instanceId});
  }catch(error){if(current!==generation.current||controller.signal.aborted)return;handleError(error);setReadError(error instanceof Error?error.message:'无法读取返工记录，请刷新后重试');}})();
 },[actorId,appId,viewId,onUnauthorized,onIdentityMismatch]);
 const recoverPacket=useCallback((view:RuntimeView)=>{
  const draftPacket=findDraftRecovery(actorId,appId,viewId);
  if(draftPacket?.resource?.kind==='draft'){
   const token=JSON.stringify([draftPacket.resource,draftPacket.operationId]);if(openedRecovery.current.has(token))return;
   if(draftPacket.method==='PATCH'&&draftPacket.resource.id){openedRecovery.current.add(token);setDraftSaved(false);setSaved(false);void readDraft(draftPacket.resource.id,false,draftPacket);return;}
   if(draftPacket.method==='POST'&&draftPacket.resource.creationNonce){
    const target=draftPacket.body?.targetRecordId;
    if(target===null){openedRecovery.current.add(token);setSaved(false);setDraftSaved(false);setReadError('');setEditor({identity:{kind:'new',actorId,appId,viewId,clientDraftId:draftPacket.resource.creationNonce},view,mode:'create',recoveredDraft:draftPacket});return;}
    if(typeof target==='string'&&uuid.test(target)){openedRecovery.current.add(token);setDraftSaved(false);setSaved(false);void readRecord(view,target,undefined,{kind:'record',actorId,appId,viewId,recordId:target},'edit',draftPacket);return;}
   }
  }
  const rework=scopedUnconfirmed(actorId).find(packet=>{const resource=packet.resource;return resource?.kind==='round-record'&&resource.appId===appId&&resource.viewId===viewId&&findRoundRecovery(actorId,appId,viewId,resource.id,resource.instanceId,'round-record')===packet;});
  if(rework?.resource?.kind==='round-record'){
   const resource=rework.resource,token=JSON.stringify([resource,rework.operationId]);if(openedRecovery.current.has(token))return;openedRecovery.current.add(token);
   readRework(resource.instanceId,{kind:'record',actorId,appId,viewId,recordId:resource.id},resource.id,true);return;
  }
  const packet=scopedUnconfirmed(actorId).find(value=>value.resource?.kind==='record'&&value.resource.appId===appId&&value.resource.viewId===viewId&&
   (!!value.resource.id||!!value.resource.creationNonce));
  if(!packet||!packet.resource)return;
  const resource=packet.resource,token=JSON.stringify([resource,packet.operationId]);
  if(openedRecovery.current.has(token))return;
  openedRecovery.current.add(token);setSaved(false);setReworkSaved(false);setReadError('');
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
 },[actorId,appId,viewId,readRecord,readRework,readDraft]);
 // RecordWorkspace already verified and parsed this runtime. Its lifecycle owns
 // cancellation; this callback only offers a matching immutable unknown write.
 const runtimeReady=useCallback((view:RuntimeView)=>recoverPacket(view),[recoverPacket]);
 const refreshEditor=()=>{
  if(!editor)return;
  if(editor.draft){const id=editor.draft.id;requestSectionLeave(()=>{setDraftSaved(false);void readDraft(id,false);});return;}
  if(editor.record){requestSectionLeave(()=>{setSaved(false);setReworkSaved(false);void refreshRead(editor.identity,editor.record!.id,false);});return;}
  requestSectionLeave(()=>{
   setEditor(null);setPendingRead(null);setReadError('');setSaved(false);setReworkSaved(false);setRecordDirty(false);
   setWorkspaceRefresh(value=>value+1);
  });
 };
 const openFlowAction=(action:()=>void)=>{const current=editor;if(!current)return;requestSectionLeave(()=>{setEditor({...current,mode:'read',reworkInstanceId:undefined});setRecordDirty(false);action();});};
 const openRework=(instanceId:string)=>{if(editor?.record)readRework(instanceId,editor.identity,editor.record.id);};
 const hasPendingFlow=workflowDirty||scopedUnconfirmed(actorId).some(packet=>packet.resource?.kind==='instance-action'&&packet.resource.appId===appId&&packet.resource.viewId===viewId&&packet.resource.id===editor?.record?.id);
 const canRetryRead=!!pendingRead&&!!readError;
 const showingDetail=!!pendingDraftRead||!!pendingRead||!!editor;
 const historyAllowed=!!editor?.record&&!editor.draft&&scopeAllows(editor.view.capabilities.read,actorId,editor.record.createdBy)&&scopeAllows(editor.view.capabilities.history,actorId,editor.record.createdBy);
 const editorContent=<>
   {pendingDraftRead&&!readError&&<p role="status">正在读取草稿…</p>}
   {pendingDraftRead&&readError&&<button type="button" className="admin-button" onClick={()=>void readDraft(pendingDraftRead.id,pendingDraftRead.confirmed,pendingDraftRead.recoveredDraft)}>重试读取草稿</button>}
   {pendingRead&&!readError&&<p role="status">正在读取记录…</p>}
   {readError&&<><p role="alert">{readError}</p>{canRetryRead&&<button type="button" className="admin-button" onClick={()=>{const target=pendingRead!;setReadError('');if(target.view)void readRecord(target.view,target.recordId,target.queryVersion,target.identity,target.mode,target.recoveredDraft);else void refreshRead(target.identity,target.recordId,target.confirmedWrite);}}>重试读取记录</button>}</>}
   <div className="record-detail-grid">{editor&&<RecordForm key={JSON.stringify([editor.identity,editor.mode,editor.view.policyRevision,editor.view.schemaVersion,editor.view.viewVersion,editor.record?.recordVersion??'new',editor.draft?.draftVersion])} actionHost={editor.mode==='create'?createActionHost:null} view={editor.view} identity={editor.identity} record={editor.record} draft={editor.draft} recoveredDraft={editor.recoveredDraft} onDraftConfirmed={onDraftConfirmed} mode={editor.mode} authorityKey={`${editor.view.policyRevision}:${editor.view.schemaVersion}:${editor.view.viewVersion}:${editor.record?.recordVersion??'new'}`}
    reworkInstanceId={editor.reworkInstanceId} queryVersion={editor.queryVersion} loadCandidates={loadCandidates} onConfirmed={onConfirmed} onDirtyChange={setRecordDirty} onRequestDiscard={guardedClose} onDiscard={closeEditor}
    onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} registerLeaveGuard={registerLeaveGuard}
    onRefresh={refreshEditor}/>}
   {editor?.mode==='create'&&<section className="app-surface record-workflow-card" aria-label="关联流程"><h2>关联流程</h2><p>记录保存后可查看关联流程</p></section>}
   {editor?.record&&!editor.draft&&<RecordWorkflows historyAction={historyAllowed?<section className="record-history-entry" aria-label="最近变更"><h2>最近变更</h2><button type="button" className="admin-button" onClick={()=>setHistoryRecord(editor.record!.id)}>查看历史</button></section>:undefined} onRework={openRework} actorId={actorId} appId={appId} viewId={viewId} recordId={editor.record.id} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave} onOpenAction={openFlowAction} onDirtyChange={setWorkflowDirty}/>} </div>
 </>;
 return <section className="record-route" aria-label="记录路由">
  {draftSaved&&<p className="record-saved" role="status">草稿已保存</p>}
  {saved&&<p className="record-saved" role="status">{reworkSaved?"返工修改已保存":"记录已保存"}</p>}
  <div hidden={showingDetail} className="record-list-host"><RecordWorkspace requestSectionLeave={requestSectionLeave} registerLeaveGuard={registerLeaveGuard} onDirtyChange={setPresetDirty} key={`${actorId}:${appId}:${viewId}:${workspaceRefresh}`} actorId={actorId} appId={appId} viewId={viewId}
   onOpenDrafts={setDraftsView} onCreate={openNew} onOpenRecord={openRow} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onRuntimeReady={runtimeReady}/></div>
  {historyAllowed&&historyRecord===editor?.record?.id&&<RecordHistoryPanel actorId={actorId} appId={appId} viewId={viewId} recordId={historyRecord!} onClose={()=>setHistoryRecord(null)} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>}
  {draftsView&&<DraftsPanel actorId={actorId} view={draftsView} onOpen={id=>{setDraftsView(null);setDraftSaved(false);setSaved(false);void readDraft(id,false);}} onClose={()=>setDraftsView(null)} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave} onDirtyChange={setDraftListDirty}/>}
  {showingDetail?<section className="record-detail-page" onKeyDown={event=>{if(editor?.mode==='create'&&event.key==='Escape'&&event.target instanceof Element&&!event.target.closest('dialog')){event.preventDefault();guardedClose();}}} aria-label={editor?.mode==='create'?"新建记录":"记录详情"}>
   <div className="app-page-heading"><h1>{editor?.mode==='create'?"新建记录":"记录详情"}</h1><div className="record-detail-actions">{editor?.mode==='create'&&<div ref={setCreateActionHost}/>}<button type="button" className="admin-button" aria-label="关闭" onClick={guardedClose}>返回</button>{editor?.mode==='read'&&editAllowed(editor)&&<button type="button" className="admin-button primary" disabled={hasPendingFlow} onClick={()=>{setSaved(false);setReworkSaved(false);setEditor({...editor,mode:'edit',queryVersion:editor.queryVersion});}}>编辑记录</button>}</div></div>
   {editorContent}
  </section>:null}
 </section>;
}
