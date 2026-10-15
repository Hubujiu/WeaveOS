import {useEffect,useMemo,useRef,useState,type ReactNode} from 'react';
import {FieldRenderer,type LoadReferenceCandidates,type ReferenceCandidate,type ReferenceDisplay} from '../forms';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {useApplicationOperation,type ApplicationMutationRequest} from '../useApplicationOperation';
import '../forms/forms.css';
import type {Draft,DraftMutationResult,FieldValue,MutationResult,RecordItem,RuntimeField,RuntimeLayoutNode,RuntimeView,UUID,Values} from './contracts';
import {isDraftMutationResult,isMutationResult} from './contracts';
import {confirmRecordIdentity,fieldRendererKey,type RecordEditorIdentity} from './recordState';
import {projectRuntimeFields,requiresRuntimeReview,scopeAllows} from './runtimeModel';

export type RecordFormProps={
 view:RuntimeView;identity:RecordEditorIdentity;record?:RecordItem;draft?:Draft;onDraftConfirmed?:(result:DraftMutationResult)=>void;mode:'create'|'edit'|'read';authorityKey:string;queryVersion?:string;reworkInstanceId?:string;
 loadCandidates?:(field:RuntimeField,request:Parameters<LoadReferenceCandidates>[0],signal:AbortSignal)=>ReturnType<LoadReferenceCandidates>;
 onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;onRefresh:()=>void;
 onConfirmed:(result:MutationResult,identity:RecordEditorIdentity)=>void;onDirtyChange:(dirty:boolean)=>void;
 /** User intent from the footer; must request confirmation before discarding dirty input. */
 onRequestDiscard:()=>void;
 /** Cleanup callback invoked by LeaveController only after a confirmed discard. */
 onDiscard:()=>void;
};
const systemFieldLabels=new Map<string,string>([['id','记录 ID'],['createdBy','创建人'],['createdAt','创建时间'],['updatedAt','更新时间'],['recordVersion','记录版本']]);
function layoutSpan(span?:number):number{return typeof span==='number'&&Number.isInteger(span)&&span>=1&&span<=12?span:12;}
function systemFieldValue(fieldId:string,record:RecordItem):string|undefined{switch(fieldId){case'id':return record.id;case'createdBy':return record.createdBy;case'createdAt':return record.createdAt;case'updatedAt':return record.updatedAt;case'recordVersion':return String(record.recordVersion);default:return undefined;}}
function initialValues(view:RuntimeView,actorId:UUID,mode:RecordFormProps['mode'],record?:RecordItem):Values{return Object.fromEntries(projectRuntimeFields(view,mode,actorId,record).filter(port=>port.editable&&Object.hasOwn(port,'value')).map(port=>[port.field.id,port.value])) as Values;}
const same=(a:unknown,b:unknown)=>JSON.stringify(a)===JSON.stringify(b);

export function RecordForm(props:RecordFormProps){
 const {identity,mode}=props;
 const resourceId=identity.kind==='new'?identity.clientDraftId:identity.kind==='record'?identity.recordId:identity.draftId;
 const scopeKey=JSON.stringify([identity.actorId,identity.appId,identity.viewId,identity.kind,resourceId,mode,props.reworkInstanceId]);
 return <RecordFormScope key={scopeKey} {...props}/>;
}

function RecordFormScope(props:RecordFormProps){
 const {view,identity,record,draft,onDraftConfirmed,mode,authorityKey,queryVersion,reworkInstanceId,loadCandidates,onUnauthorized,onIdentityMismatch,registerLeaveGuard,onRefresh,onConfirmed,onDirtyChange,onRequestDiscard,onDiscard}=props;
 const recordId=identity.kind==='record'?identity.recordId:draft?.targetRecordId??undefined;
 const saved=!!recordId;
 const invalidDraftIdentity=identity.kind==='draft'&&(!draft||draft.id!==identity.draftId||draft.viewId!==identity.viewId||draft.tableId!==view.tableId);
 const creationNonce=identity.kind==='new'?identity.clientDraftId:identity.kind==='draft'?identity.draftId:undefined;
 const resource=useMemo(()=>reworkInstanceId&&recordId?{kind:'round-record' as const,appId:identity.appId,viewId:identity.viewId,id:recordId,instanceId:reworkInstanceId}:recordId?{kind:'record' as const,appId:identity.appId,viewId:identity.viewId,id:recordId}:creationNonce?{kind:'record' as const,appId:identity.appId,viewId:identity.viewId,creationNonce}:null,[identity.appId,identity.viewId,recordId,creationNonce,reworkInstanceId]);
 const operationScope=reworkInstanceId&&recordId?`round-record:${identity.appId}:${identity.viewId}:${recordId}:${reworkInstanceId}`:recordId?`record-editor:${identity.appId}:${identity.viewId}:record:${recordId}`:`record-editor:${identity.appId}:${identity.viewId}:new:${creationNonce}`;
 const [draftJob,setDraftJob]=useState<DraftJob|null>(null),[draftPhase,setDraftPhase]=useState('idle'),[draftError,setDraftError]=useState(''),[draftConflict,setDraftConflict]=useState(false);
 const draftOperation=useRef<DraftOperation|null>(null),pendingDraft=useRef<DraftJob|null>(null);
 const draftBasisConflict=!!draft&&(draft.hasConflicts||draft.schemaVersion!==view.schemaVersion||!!draft.targetRecordId&&draft.baseRecordVersion!==record?.recordVersion);
 const confirmedRef=useRef(onConfirmed);confirmedRef.current=onConfirmed;
 const unauthorizedRef=useRef(onUnauthorized);unauthorizedRef.current=onUnauthorized;
 const identityMismatchRef=useRef(onIdentityMismatch);identityMismatchRef.current=onIdentityMismatch;
 const confirmationSync=useRef(false);
 const [confirmed,setConfirmed]=useState(false);
 const hookResource=resource??{kind:'record' as const,appId:identity.appId,viewId:identity.viewId,id:identity.kind==='draft'?identity.draftId:identity.kind==='record'?identity.recordId:''};
 const operation=useApplicationOperation<MutationResult>({actorId:identity.actorId,scope:operationScope,resource:hookResource,confirmed:(result)=>{if(!result)return;const next=identity.kind==='new'?confirmRecordIdentity(identity,result,result.operationId):identity.kind==='draft'?{kind:'record' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,recordId:result.id}:identity;if(!next)return;confirmationSync.current=true;setConfirmed(true);readGuard.current={...readGuard.current,confirmed:true,packetId:null,phase:'idle',dirty:false};onDirtyChange(false);confirmedRef.current(result,next);},unauthorized:()=>unauthorizedRef.current(),identityMismatch:()=>identityMismatchRef.current(),valid:(result,packet)=>!!packet.operationId&&isMutationResult(result,packet.operationId,recordId)});
 const startView=useRef(view),startValues=useRef(draft?Object.fromEntries(Object.entries(draft.values).filter(([id])=>projectRuntimeFields(view,mode,identity.actorId,record).some(port=>port.editable&&port.field.id===id))):initialValues(view,identity.actorId,mode,record));
 const [values,setValues]=useState<Values>(startValues.current),[pendingDisplays,setPendingDisplays]=useState<Record<UUID,ReferenceDisplay>>({});
 const fields=projectRuntimeFields(view,mode,identity.actorId,record);
 const fieldById=new Map(fields.map(port=>[port.field.id,port] as const));
 const currentEditable=new Set(fields.filter(port=>port.editable).map(port=>port.field.id));
 const packet=resource&&operation.packet?.resource&&same(operation.packet.resource,resource)?operation.packet:null;
 const phase=packet?operation.phase:'idle';
 const unknown=phase==='unconfirmed';
 const draftUnknown=!!draftJob&&draftPhase==='unconfirmed';
 const busy=phase==='preflight'||phase==='pending'||!!draftJob&&!draftUnknown;
 const dirty=!same(values,startValues.current);
 const needsReview=requiresRuntimeReview(startView.current,view,dirty||!!packet);
 const visibleRecord=record&&record.appId===view.appId&&record.tableId===view.tableId&&record.viewId===view.viewId&&scopeAllows(view.capabilities.read,identity.actorId,record.createdBy)?record:undefined;
 const canEditRecord=saved&&!!visibleRecord&&scopeAllows(view.capabilities.edit,identity.actorId,visibleRecord.createdBy);
 const retainedUnknown=unknown&&!!packet?.operationId&&operation.getPendingStatus()==='unknown';

 // Recovery packets are immutable. Reconstruct only present values/changes the
 // current runtime still permits, over this mount's projected baseline.
 const hydrated=useRef<string|null>(null);
 useEffect(()=>{if(!packet?.operationId||hydrated.current===packet.operationId)return;hydrated.current=packet.operationId;const frozen=packet.body?.values??packet.body?.changes;if(!frozen||typeof frozen!=='object'||Array.isArray(frozen))return;setValues(old=>{const next={...old};for(const [id,value] of Object.entries(frozen))if(currentEditable.has(id))next[id]=value as FieldValue;return next;});},[packet]);
 useEffect(()=>{setPendingDisplays({});},[authorityKey]);
 useEffect(()=>{onDirtyChange((dirty||!!draftJob)&&!confirmed);},[dirty,draftJob,confirmed,onDirtyChange]);

 const readGuard=useRef({values,packetId:packet?.operationId??null,phase,needsReview,dirty,confirmed:confirmationSync.current});
 readGuard.current={values,packetId:packet?.operationId??null,phase,needsReview,dirty,confirmed:confirmationSync.current};
 const discardRef=useRef(onDiscard);discardRef.current=onDiscard;
 const cancelRef=useRef(()=>{});cancelRef.current=()=>{operation.cancelPreflight();draftOperation.current?.cancelPreflight();};
 const pendingStatusRef=useRef(operation.getPendingStatus);pendingStatusRef.current=()=>draftOperation.current?.getPendingStatus()??(pendingDraft.current?'preflight':operation.getPendingStatus());
 useEffect(()=>{
  const scope=identity.kind==='new'?{kind:'record' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,clientDraftId:identity.clientDraftId}:identity.kind==='record'?{kind:'record' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,recordId:identity.recordId}:{kind:'draft' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,draftId:identity.draftId};
  const controller=createLeaveController(()=>{const s=readGuard.current;const pending=pendingStatusRef.current();const status=s.confirmed?'clean':pending==='unknown'?'unknown':pending==='write_in_flight'?'write_in_flight':pending==='preflight'?'preflight':s.dirty?'draft':'clean';return {status,fingerprint:JSON.stringify([s.values,s.packetId,s.phase,s.needsReview,s.confirmed,status])};},decision=>{if(decision==='discard'&&!readGuard.current.confirmed&&(readGuard.current.dirty||pendingStatusRef.current()==='preflight')){cancelRef.current();discardRef.current();}});
  return registerLeaveGuard(scope,controller);
 },[identity,registerLeaveGuard]);

 const save=()=>{
  if(!resource||invalidDraftIdentity||pendingDraft.current||draftConflict||draftBasisConflict||busy||unknown||confirmed||needsReview||mode==='read'||saved&&!dirty&&!draft||!view.capabilities.create&&!saved||saved&&!canEditRecord)return;
  const recordPath='applications/'+encodeURIComponent(identity.appId)+'/forms/'+encodeURIComponent(identity.viewId)+'/records'+(saved?'/'+encodeURIComponent(recordId!):'');
  const path=reworkInstanceId?recordPath+'/workflows/'+encodeURIComponent(reworkInstanceId)+'/rework':recordPath;
  const payload:Values=Object.fromEntries(Object.entries(values).filter(([id,value])=>currentEditable.has(id)&&(mode==='create'||!same(record?.values[id],value))));
  if(saved&&!Object.keys(payload).length&&!draft)return;
  const draftRef=draft?{draftRef:{id:draft.id,draftVersion:draft.draftVersion}}:{};
  const body=saved?{expectedSchemaVersion:draft?.schemaVersion??startView.current.schemaVersion,expectedRecordVersion:draft?.baseRecordVersion??record?.recordVersion,changes:payload,...draftRef,...(queryVersion&&!reworkInstanceId?{queryVersion}:{})}:{expectedSchemaVersion:draft?.schemaVersion??startView.current.schemaVersion,values:payload,...draftRef,...(queryVersion&&!reworkInstanceId?{queryVersion}:{})};
  const request:ApplicationMutationRequest={path,method:saved&&!reworkInstanceId?'PATCH':'POST',expectedStatus:saved?200:201,resource,body};
  operation.start(request);
 };
 const saveDraft=()=>{
  if(!onDraftConfirmed||invalidDraftIdentity||pendingDraft.current||operation.getPendingStatus()||busy||unknown||confirmed||needsReview||draftConflict||draftBasisConflict||reworkInstanceId||mode==='read'||(saved?!view.capabilities.draftEdit:!view.capabilities.draftCreate))return;
  const draftResource={kind:'draft' as const,appId:identity.appId,viewId:identity.viewId,...(draft?{id:draft.id}:{creationNonce:crypto.randomUUID()})};
  const payload=Object.fromEntries(Object.entries(values).filter(([id,value])=>currentEditable.has(id)&&(!draft||!same(startValues.current[id],value))));
  const request:ApplicationMutationRequest={resource:draftResource,path:`applications/${identity.appId}/forms/${identity.viewId}/drafts`+(draft?'/'+draft.id:''),method:draft?'PATCH':'POST',expectedStatus:draft?200:201,body:draft?{expectedDraftVersion:draft.draftVersion,changes:payload,removeFieldIds:[]}:{targetRecordId:recordId??null,schemaVersion:startView.current.schemaVersion,baseRecordVersion:record?.recordVersion??null,values:payload}};
  const job={key:crypto.randomUUID(),request};pendingDraft.current=job;setDraftError('');setDraftPhase('preflight');setDraftJob(job);
 };
 const settleDraft=(result?:DraftMutationResult,message='',code?:string)=>{
  if(result){confirmationSync.current=true;setConfirmed(true);readGuard.current={...readGuard.current,confirmed:true,packetId:null,phase:'idle',dirty:false};onDirtyChange(false);onDraftConfirmed?.(result);return;}
  pendingDraft.current=null;setDraftJob(null);setDraftPhase('idle');setDraftError(message);setDraftConflict(['APPLICATION_DRAFT_CONFLICT','APPLICATION_DRAFT_BASE_CONFLICT','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(code??''));
 };
 const canEdit=!invalidDraftIdentity&&!busy&&!unknown&&!draftJob&&!confirmed&&!needsReview&&!draftBasisConflict&&mode!=='read';
 const renderField=(node:Extract<RuntimeLayoutNode,{kind:'field'}>,port:ReturnType<typeof projectRuntimeFields>[number]):ReactNode=>{
  const id=port.field.id,instanceKey=fieldRendererKey(identity,id),readOnly=!canEdit||!port.editable;
  const value=(port.editable?values[id]:port.value)??null,selectedId=typeof value==='string'?value:null;
  const authoritative=selectedId?record?.referenceDisplays[id]?.[selectedId]:undefined;
  const referenceDisplay=authoritative??(selectedId&&pendingDisplays[id]?.id===selectedId?pendingDisplays[id]:undefined);
  const candidateLoader=!readOnly&&loadCandidates&&(port.field.kind==='member'||port.field.kind==='department')?(request:Parameters<LoadReferenceCandidates>[0],signal:AbortSignal)=>loadCandidates(port.field,request,signal):undefined;
  const change=(next:FieldValue,candidate?:ReferenceCandidate)=>{setValues(old=>({...old,[id]:next}));setPendingDisplays(old=>{const copy={...old};delete copy[id];if(typeof next==='string'&&candidate?.id===next&&candidate.status==='active')copy[id]={id:next,label:candidate.label,deleted:false};return copy;});};
  const span=layoutSpan(node.span);
  return <div key={node.id} style={{gridColumn:`span ${span} / span ${span}`}}><FieldRenderer field={port.field} value={value} readOnly={readOnly} onChange={readOnly?undefined:change} referenceDisplay={referenceDisplay} loadReferenceCandidates={candidateLoader} referenceScopeKey={JSON.stringify([instanceKey,authorityKey])} idPrefix="record"/></div>;
 };
 const renderLayoutNode=(node:RuntimeLayoutNode):ReactNode=>{
  if(node.kind==='field'){const port=fieldById.get(node.fieldId);return port?renderField(node,port):null;}
  if(node.kind==='group'){const span=layoutSpan(node.span);return <fieldset key={node.id} className="forms-preview-group" style={{gridColumn:`span ${span} / span ${span}`}}><legend>{node.title}</legend><div className="forms-preview-grid">{node.children.map(renderLayoutNode)}</div></fieldset>;}
  if(node.kind==='divider')return <hr key={node.id} className="forms-preview-divider" style={{gridColumn:'span 12 / span 12'}}/>;
  if(node.kind==='description')return <p key={node.id} className="record-form-description" style={{gridColumn:'span 12 / span 12'}}>{node.text}</p>;
  if(node.kind==='system_field'){const label=systemFieldLabels.get(node.fieldId);if(!label)return null;const value=mode==='create'?'由系统填写':visibleRecord?systemFieldValue(node.fieldId,visibleRecord):undefined;if(value===undefined)return null;const span=layoutSpan(node.span),inputId=`record-system-${node.id}`;return <div key={node.id} style={{gridColumn:`span ${span} / span ${span}`}} className="forms-rendered-field"><label htmlFor={inputId}>{label}</label><input id={inputId} aria-label={label} readOnly value={value}/></div>;}
  return null;
 };
 const conflict=operation.phase==='error'&&['APPLICATION_RECORD_CONFLICT','APPLICATION_SCHEMA_CONFLICT','APPLICATION_QUERY_CHANGED','APPLICATION_QUERY_CONTEXT_EXPIRED','APPLICATION_POLICY_CONFLICT',...(reworkInstanceId?['WORKFLOW_CONFLICT','WORKFLOW_NOT_READY']:[])].includes(operation.errorCode??'');
 const statusMessage=unknown?'保存结果待确认':busy?'正在确认并保存…':'';
 return <section aria-label={reworkInstanceId?"返工记录":"记录填写"} className="surface record-form">
  {invalidDraftIdentity?<p role="alert">草稿身份不能保存为普通记录</p>:null}
  {draftBasisConflict?<p role="alert">草稿结构、基础记录或字段权限已变化，请刷新核对；原草稿不会自动覆盖或重新绑定。</p>:null}
  {draftError?<p role="alert">{draftConflict?'草稿版本或权限已变化，请刷新草稿并核对当前输入。':''}{draftError}</p>:null}
  {draftJob?<RecordDraftMutation job={draftJob} actorId={identity.actorId} operationRef={draftOperation} onPhase={setDraftPhase} settle={settleDraft} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>:null}
  {needsReview?<p role="alert">结构或权限已变化，请核对当前输入</p>:null}
  {conflict?<p role="alert">记录、结构、筛选或权限已变化，请刷新记录并核对当前输入。{operation.message}</p>:null}
  {operation.phase==='error'&&!conflict?<p role="alert">{operation.message}</p>:null}
  {unknown?<p role="status">{statusMessage}</p>:busy?<p role="status">{statusMessage}</p>:null}
  {unknown&&operation.message?<p role="alert">{operation.message}</p>:null}
  {unknown?<button type="button" className="admin-button" onClick={operation.query}>恢复保存结果</button>:null}
  {retainedUnknown?<button type="button" className="admin-button" onClick={operation.retry} disabled={busy}>使用同一操作重试</button>:null}
  {conflict?<button type="button" className="admin-button" onClick={onRefresh}>刷新记录</button>:null}
  {needsReview&&!conflict?<button type="button" className="admin-button" onClick={onRefresh}>刷新记录</button>:null}
  <div className="record-form-fields forms-preview-grid">{view.layout.map(renderLayoutNode)}</div>
  {mode!=='read'&&<footer><button type="button" className="admin-button" onClick={onRequestDiscard} disabled={busy||unknown||!!draftJob}>放弃填写</button>{onDraftConfirmed&&!reworkInstanceId&&(saved?view.capabilities.draftEdit:view.capabilities.draftCreate)&&<button type="button" className="admin-button" onClick={saveDraft} disabled={!canEdit||draftConflict||conflict}>保存草稿</button>}<button type="button" className="admin-button" onClick={save} disabled={!canEdit||conflict||draftConflict||saved&&!dirty&&!draft||!saved&&!view.capabilities.create||saved&&!canEditRecord}>{reworkInstanceId?"保存返工":"保存记录"}</button></footer>}
 </section>;
}


type DraftJob={key:string;request:ApplicationMutationRequest};
type DraftOperation=ReturnType<typeof useApplicationOperation<DraftMutationResult>>;
function RecordDraftMutation({job,actorId,operationRef,onPhase,settle,onUnauthorized,onIdentityMismatch}:{job:DraftJob;actorId:string;operationRef:{current:DraftOperation|null};onPhase:(phase:string)=>void;settle:(result?:DraftMutationResult,message?:string,code?:string)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const resource=job.request.resource;
 const key=resource.id??resource.creationNonce;
 const operation=useApplicationOperation<DraftMutationResult>({actorId,scope:`draft-editor:${resource.appId}:${resource.viewId}:${key}`,resource,confirmed:result=>settle(result),unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch,valid:(result,packet)=>isDraftMutationResult(result,packet.operationId??'',resource.id)});
 const current=useRef(operation);current.current=operation;operationRef.current=operation;
 useEffect(()=>()=>{operationRef.current=null;},[]);
 useEffect(()=>{let active=true;void Promise.resolve().then(()=>{if(active)current.current.start(job.request);});return()=>{active=false;};},[job]);
 useEffect(()=>{onPhase(operation.phase);if(operation.phase==='error')settle(undefined,operation.message,operation.errorCode);},[operation.phase,operation.message,operation.errorCode]);
 return operation.phase==='unconfirmed'?<div><p role="status">草稿保存结果待确认</p><p role="alert">{operation.message}</p><button type="button" className="admin-button" onClick={operation.query}>核查草稿结果</button><button type="button" className="admin-button" onClick={operation.retry}>重试原草稿操作</button></div>:<p role="status">正在保存草稿…</p>;
}
