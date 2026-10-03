import {useEffect,useMemo,useRef,useState,type ReactNode} from 'react';
import {FieldRenderer,type LoadReferenceCandidates,type ReferenceCandidate,type ReferenceDisplay} from '../forms';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {useApplicationOperation,type ApplicationMutationRequest} from '../useApplicationOperation';
import '../forms/forms.css';
import type {FieldValue,MutationResult,RecordItem,RuntimeField,RuntimeLayoutNode,RuntimeView,UUID,Values} from './contracts';
import {isMutationResult} from './contracts';
import {confirmRecordIdentity,fieldRendererKey,type RecordEditorIdentity} from './recordState';
import {projectRuntimeFields,requiresRuntimeReview,scopeAllows} from './runtimeModel';

export type RecordFormProps={view:RuntimeView;identity:RecordEditorIdentity;record?:RecordItem;mode:'create'|'edit'|'read';authorityKey:string;queryVersion?:string;loadCandidates?:(field:RuntimeField,request:Parameters<LoadReferenceCandidates>[0],signal:AbortSignal)=>ReturnType<LoadReferenceCandidates>;onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;onRefresh:()=>void;onConfirmed:(result:MutationResult,identity:RecordEditorIdentity)=>void;onDirtyChange:(dirty:boolean)=>void;onDiscard:()=>void};
const systemFieldLabels=new Map<string,string>([['id','记录 ID'],['createdBy','创建人'],['createdAt','创建时间'],['updatedAt','更新时间'],['recordVersion','记录版本']]);
function layoutSpan(span?:number):number{return typeof span==='number'&&Number.isInteger(span)&&span>=1&&span<=12?span:12;}
function systemFieldValue(fieldId:string,record:RecordItem):string|undefined{switch(fieldId){case'id':return record.id;case'createdBy':return record.createdBy;case'createdAt':return record.createdAt;case'updatedAt':return record.updatedAt;case'recordVersion':return String(record.recordVersion);default:return undefined;}}
function initialValues(view:RuntimeView,actorId:UUID,mode:RecordFormProps['mode'],record?:RecordItem):Values{return Object.fromEntries(projectRuntimeFields(view,mode,actorId,record).filter(port=>port.editable&&Object.hasOwn(port,'value')).map(port=>[port.field.id,port.value])) as Values;}
const same=(a:unknown,b:unknown)=>JSON.stringify(a)===JSON.stringify(b);

export function RecordForm(props:RecordFormProps){
 const {identity,mode}=props;
 const resourceId=identity.kind==='new'?identity.clientDraftId:identity.kind==='record'?identity.recordId:identity.draftId;
 const scopeKey=JSON.stringify([identity.actorId,identity.appId,identity.viewId,identity.kind,resourceId,mode]);
 return <RecordFormScope key={scopeKey} {...props}/>;
}

function RecordFormScope(props:RecordFormProps){
 const {view,identity,record,mode,authorityKey,queryVersion,loadCandidates,onUnauthorized,onIdentityMismatch,registerLeaveGuard,onRefresh,onConfirmed,onDirtyChange,onDiscard}=props;
 const saved=identity.kind==='record';
 const resource=useMemo(()=>identity.kind==='new'?{kind:'record' as const,appId:identity.appId,viewId:identity.viewId,creationNonce:identity.clientDraftId}:identity.kind==='record'?{kind:'record' as const,appId:identity.appId,viewId:identity.viewId,id:identity.recordId}:null,[identity]);
 const operationScope=identity.kind==='new'?`record-editor:${identity.appId}:${identity.viewId}:new:${identity.clientDraftId}`:identity.kind==='record'?`record-editor:${identity.appId}:${identity.viewId}:record:${identity.recordId}`:`record-editor:${identity.appId}:${identity.viewId}:draft:${identity.draftId}`;
 const confirmedRef=useRef(onConfirmed);confirmedRef.current=onConfirmed;
 const unauthorizedRef=useRef(onUnauthorized);unauthorizedRef.current=onUnauthorized;
 const identityMismatchRef=useRef(onIdentityMismatch);identityMismatchRef.current=onIdentityMismatch;
 const confirmationSync=useRef(false);
 const [confirmed,setConfirmed]=useState(false);
 const hookResource=resource??{kind:'record' as const,appId:identity.appId,viewId:identity.viewId,id:identity.kind==='draft'?identity.draftId:identity.kind==='record'?identity.recordId:''};
 const operation=useApplicationOperation<MutationResult>({actorId:identity.actorId,scope:operationScope,resource:hookResource,confirmed:(result)=>{if(!result)return;const next=identity.kind==='new'?confirmRecordIdentity(identity,result,result.operationId):identity;if(!next)return;confirmationSync.current=true;setConfirmed(true);readGuard.current={...readGuard.current,confirmed:true,packetId:null,phase:'idle',dirty:false};onDirtyChange(false);confirmedRef.current(result,next);},unauthorized:()=>unauthorizedRef.current(),identityMismatch:()=>identityMismatchRef.current(),valid:(result,packet)=>!!packet.operationId&&isMutationResult(result,packet.operationId,identity.kind==='record'?identity.recordId:undefined)});
 const startView=useRef(view),startValues=useRef(initialValues(view,identity.actorId,mode,record));
 const [values,setValues]=useState<Values>(startValues.current),[pendingDisplays,setPendingDisplays]=useState<Record<UUID,ReferenceDisplay>>({});
 const fields=projectRuntimeFields(view,mode,identity.actorId,record);
 const fieldById=new Map(fields.map(port=>[port.field.id,port] as const));
 const currentEditable=new Set(fields.filter(port=>port.editable).map(port=>port.field.id));
 const packet=resource&&operation.packet?.resource&&same(operation.packet.resource,resource)?operation.packet:null;
 const phase=packet?operation.phase:'idle';
 const unknown=phase==='unconfirmed';
 const busy=phase==='preflight'||phase==='pending';
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
 useEffect(()=>{onDirtyChange(dirty&&!confirmed);},[dirty,confirmed,onDirtyChange]);

 const readGuard=useRef({values,packetId:packet?.operationId??null,phase,needsReview,dirty,confirmed:confirmationSync.current});
 readGuard.current={values,packetId:packet?.operationId??null,phase,needsReview,dirty,confirmed:confirmationSync.current};
 const discardRef=useRef(onDiscard);discardRef.current=onDiscard;
 const cancelRef=useRef(operation.cancelPreflight);cancelRef.current=operation.cancelPreflight;
 const pendingStatusRef=useRef(operation.getPendingStatus);pendingStatusRef.current=operation.getPendingStatus;
 useEffect(()=>{
  const scope=identity.kind==='new'?{kind:'record' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,clientDraftId:identity.clientDraftId}:identity.kind==='record'?{kind:'record' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,recordId:identity.recordId}:{kind:'record' as const,actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,clientDraftId:identity.draftId};
  const controller=createLeaveController(()=>{const s=readGuard.current;const pending=pendingStatusRef.current();const status=s.confirmed?'clean':pending==='unknown'?'unknown':pending==='write_in_flight'?'write_in_flight':pending==='preflight'?'preflight':s.dirty?'draft':'clean';return {status,fingerprint:JSON.stringify([s.values,s.packetId,s.phase,s.needsReview,s.confirmed,status])};},decision=>{if(decision==='discard'){cancelRef.current();discardRef.current();}});
  return registerLeaveGuard(scope,controller);
 },[identity,registerLeaveGuard]);

 const save=()=>{
  if(!resource||identity.kind==='draft'||busy||unknown||confirmed||needsReview||mode==='read'||saved&&!dirty||!view.capabilities.create&&!saved||saved&&!canEditRecord)return;
  const path='applications/'+encodeURIComponent(identity.appId)+'/forms/'+encodeURIComponent(identity.viewId)+'/records'+(saved?'/'+encodeURIComponent(identity.recordId):'');
  const payload:Values=Object.fromEntries(Object.entries(values).filter(([id,value])=>currentEditable.has(id)&&(mode==='create'||!same(startValues.current[id],value))));
  if(saved&&!Object.keys(payload).length)return;
  const body=saved?{expectedSchemaVersion:startView.current.schemaVersion,expectedRecordVersion:record?.recordVersion,changes:payload,...(queryVersion?{queryVersion}:{})}:{expectedSchemaVersion:startView.current.schemaVersion,values:payload,...(queryVersion?{queryVersion}:{})};
  const request:ApplicationMutationRequest={path,method:saved?'PATCH':'POST',expectedStatus:saved?200:201,resource,body};
  operation.start(request);
 };
 const canEdit=!busy&&!unknown&&!confirmed&&!needsReview&&mode!=='read'&&identity.kind!=='draft';
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
 const conflict=operation.phase==='error'&&['APPLICATION_RECORD_CONFLICT','APPLICATION_SCHEMA_CONFLICT','APPLICATION_QUERY_CHANGED','APPLICATION_QUERY_CONTEXT_EXPIRED','APPLICATION_POLICY_CONFLICT'].includes(operation.errorCode??'');
 const statusMessage=unknown?'保存结果待确认':busy?'正在确认并保存…':'';
 return <section aria-label="记录填写" className="surface record-form">
  {identity.kind==='draft'?<p role="alert">草稿身份不能保存为普通记录</p>:null}
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
  <footer><button type="button" className="admin-button" onClick={onDiscard} disabled={busy||unknown}>放弃填写</button><button type="button" className="admin-button" onClick={save} disabled={!canEdit||conflict||saved&&!dirty||!saved&&!view.capabilities.create||saved&&!canEditRecord}>保存记录</button></footer>
 </section>;
}
