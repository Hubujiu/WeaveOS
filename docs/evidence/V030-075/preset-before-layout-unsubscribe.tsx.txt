import {useCallback,useEffect,useLayoutEffect,useRef,useState,type MutableRefObject} from 'react';
import {TablePresetManager,type ResourcePreset,type ResourcePresetRepository,type PresetEditorState} from '../../TablePresetManager';
import type {ResourceFilterField} from '../../QueryFilterState';
import {applicationApi,ApplicationError} from '../api';
import {useApplicationOperation,type ApplicationMutationRequest} from '../useApplicationOperation';
import {validApplicationPreset,validApplicationPresets,type ApplicationPreset,type ApplicationPresetValid,type PresetMutationReceipt} from './presetContract';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import type {RuntimeView,RecordSort} from './contracts';

type Layout={sort:RecordSort;hiddenColumnIds:string[];columnOrder:string[];columnWidths:Record<string,number>};
type Props={registerLeaveGuard:RegisterLeaveGuard;onDirtyChange:(dirty:boolean)=>void;actorId:string;view:RuntimeView;layout:Layout;active:ApplicationPresetValid|null;onApply:(preset:ApplicationPresetValid|null)=>Promise<boolean>;refreshRuntime:(signal:AbortSignal)=>Promise<RuntimeView>;onUnauthorized:()=>void;onIdentityMismatch:()=>void};
type Job={key:string;request:ApplicationMutationRequest;resolve:(value:PresetMutationReceipt|undefined)=>void;reject:(error:Error)=>void};
const descriptors=(view:RuntimeView)=>({
 fields:view.fields.filter(f=>f.access.read!=='none'&&f.query.operators.length>0).map(f=>({id:f.id,label:f.name,kind:f.kind,operators:f.query.operators,optionIds:f.input.options?.map(o=>o.id),choices:f.input.options?.map(o=>({value:o.id,label:o.label}))})) as ResourceFilterField[],
 columns:view.fields.filter(f=>f.access.read!=='none').map(f=>({id:f.id,name:f.name})),
});

// Business adapter for the approved shared editor. No second query or mutation
// state machine: live reads use the scoped API and writes use the shared hook.
type PresetOperation=ReturnType<typeof useApplicationOperation<PresetMutationReceipt>>;
export function RecordPresets({registerLeaveGuard,onDirtyChange,actorId,view,layout,active,onApply,refreshRuntime,onUnauthorized,onIdentityMismatch}:Props){
 const {appId,viewId}=view,scope=JSON.stringify(['presets',actorId,appId,viewId]);
 const base=`applications/${appId}/forms/${viewId}/table-presets`;
 const [job,setJob]=useState<Job|null>(null),[dirty,setDirty]=useState(false),[operating,setOperating]=useState(false);
 const [confirmedRead,setConfirmedRead]=useState<{id:string;resolve:(preset:ApplicationPreset)=>void;message:string}|null>(null),[readingConfirmed,setReadingConfirmed]=useState(false);
 const editor=useRef<PresetEditorState>({dirty:false,fingerprint:'',discard:()=>{}}),operationRef=useRef<PresetOperation|null>(null);
 const editorState=useCallback((state:PresetEditorState)=>{editor.current=state;},[]);
 useEffect(()=>{onDirtyChange(dirty||operating);return()=>onDirtyChange(false);},[dirty,operating,onDirtyChange]);
 useEffect(()=>registerLeaveGuard({kind:'preset',actorId,appId,viewId},createLeaveController(()=>{
  const operation=operationRef.current,pendingStatus=operation?.getPendingStatus();
  return {status:pendingStatus??(editor.current.dirty?'draft':'clean'),fingerprint:JSON.stringify([pendingStatus,operation?.packet?.operationId,editor.current.fingerprint])};
 },decision=>{
  if(decision!=='discard')return;
  const operation=operationRef.current;
  if(operation?.getPendingStatus()==='preflight'){
   operation.cancelPreflight();const current=pending.current;pending.current=null;setJob(null);setOperating(false);current?.reject(new Error('未发送的方案操作已取消'));
  }
  if(editor.current.dirty)editor.current.discard();
 })),[scope,registerLeaveGuard]);
 const pending=useRef<Job|null>(null),known=useRef(new Map<string,ApplicationPreset>());
 const life=useRef<AbortController|null>(null);
 useEffect(()=>{const controller=new AbortController();life.current=controller;return()=>controller.abort();},[scope]);
 const report=(error:unknown)=>{if(error instanceof ApplicationError&&error.status===401)onUnauthorized();else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();};
 async function read(id:string):Promise<ApplicationPreset>{
  try{const raw=await applicationApi<unknown>(actorId,base+'/'+encodeURIComponent(id),'GET',undefined,life.current?.signal,200);
   if(life.current?.signal.aborted)throw new Error('读取已取消');
   if(!validApplicationPreset(raw,{appId,viewId},id))throw new Error('方案响应格式不正确');known.current.set(id,raw);return raw;
  }catch(error){report(error);throw error;}
 }
 function mutate(request:ApplicationMutationRequest):Promise<PresetMutationReceipt|undefined>{
  // Two synchronous clicks cannot enqueue two operations before React commits.
  if(pending.current)return Promise.reject(new Error('已有方案操作正在处理，请核查原操作'));
  return new Promise((resolve,reject)=>{const next={key:crypto.randomUUID(),request,resolve,reject};pending.current=next;setJob(next);});
 }
 const repository:ResourcePresetRepository={
  list:async()=>{try{
   const raw=await applicationApi<unknown>(actorId,base,'GET',undefined,life.current?.signal,200);
   if(life.current?.signal.aborted)throw new Error('读取已取消');
   if(!validApplicationPresets(raw,{appId,viewId}))throw new Error('方案列表响应格式不正确');
   for(const item of raw.items)known.current.set(item.id,item);return raw;
  }catch(error){report(error);throw error;}},
  read,
  save:async(input,existing)=>{
   const old=existing?known.current.get(existing.id):null;
   const savedLayout=old&&!old.invalid?old:layout;
   const resource={kind:'preset' as const,appId,viewId,...(existing?{id:existing.id}:{creationNonce:crypto.randomUUID()})};
   const receipt=await mutate({resource,path:base+(existing?'/'+existing.id:''),method:existing?'PUT':'POST',expectedStatus:existing?200:201,
    body:{...input,sort:savedLayout.sort,columnOrder:[...savedLayout.columnOrder],columnWidths:{...savedLayout.columnWidths},expectedSchemaVersion:view.schemaVersion,...(existing?{expectedVersion:existing.version}:{})}});
   if(!receipt)throw new Error('保存回执不完整');
   // Confirmation authorizes no historical configuration read. Re-read the
   // current projection; failure cannot turn a confirmed write into a retry.
   try{return await read(receipt.id);}catch(error){
    return new Promise<ApplicationPreset>(resolve=>setConfirmedRead({id:receipt.id,resolve,message:'方案已保存，但最新配置无法读取。'+(error instanceof Error?error.message:'')}));
   }
  },
  remove:async(id,version)=>{await mutate({resource:{kind:'preset',appId,viewId,id},path:base+'/'+id,method:'DELETE',expectedStatus:204,expectedVersion:version});known.current.delete(id);},
 };
 function settle(current:Job,result:PresetMutationReceipt|undefined,error?:Error){
  if(pending.current!==current)return;pending.current=null;setJob(null);
  if(error)current.reject(error);else current.resolve(result);
 }
 const apply=async(preset:ResourcePreset|null)=>{
  if(preset&&!validApplicationPreset(preset,{appId,viewId}))throw new Error('方案响应格式不正确');
  if(preset&&preset.invalid)throw new Error('方案已失效，请明确重新配置');
  return onApply(preset as ApplicationPresetValid|null);
 };
 async function retryConfirmedRead(){
  if(!confirmedRead||readingConfirmed)return;
  const current=confirmedRead;setReadingConfirmed(true);
  try{const preset=await read(current.id);if(life.current?.signal.aborted)return;setConfirmedRead(null);current.resolve(preset);}
  catch(error){if(!life.current?.signal.aborted)setConfirmedRead({...current,message:'方案已保存，但最新配置无法读取。'+(error instanceof Error?error.message:'')});}
  finally{setReadingConfirmed(false);}
 }
 return <TablePresetManager view="resource" active={active} hiddenColumnIds={layout.hiddenColumnIds} onDirty={setDirty} onUnauthorized={onUnauthorized}
  resource={{scopeKey:scope,onEditorState:editorState,...descriptors(view),repository,onApply:apply,
   refreshDescriptors:async()=>descriptors(await refreshRuntime(life.current!.signal)),
   operationStatus:confirmedRead?<div><p role="alert">{confirmedRead.message}</p><button type="button" disabled={readingConfirmed} onClick={()=>void retryConfirmedRead()}>重试读取已保存方案</button></div>:job?<PresetMutation key={job.key} actorId={actorId} scope={scope} job={job} operationRef={operationRef} onActivity={setOperating} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} settle={settle}/>:null}}/>;
}

function PresetMutation({actorId,scope,job,operationRef,onActivity,onUnauthorized,onIdentityMismatch,settle}:{actorId:string;scope:string;job:Job;operationRef:MutableRefObject<PresetOperation|null>;onActivity:(active:boolean)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;settle:(job:Job,result:PresetMutationReceipt|undefined,error?:Error)=>void}){
 const operation=useApplicationOperation<PresetMutationReceipt>({actorId,scope,resource:job.request.resource,confirmed:result=>settle(job,result),unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const latest=useRef(operation);latest.current=operation;
 useLayoutEffect(()=>{operationRef.current=operation;});
 useEffect(()=>()=>{operationRef.current=null;onActivity(false);},[]);
 useEffect(()=>{onActivity(['preflight','pending','unconfirmed'].includes(operation.phase));},[operation.phase,onActivity]);
 useEffect(()=>{let current=true;void Promise.resolve().then(()=>{if(current)latest.current.start(job.request);});return()=>{current=false;};},[job]);
 useEffect(()=>{if(operation.phase==='error')settle(job,undefined,new Error(operation.errorCode==='APPLICATION_PRESET_CONFLICT'?'方案已被更新，请重新加载后核对；当前输入已保留':operation.message));},[operation.phase,operation.message,operation.errorCode]);
 if(operation.phase==='unconfirmed')return <div><p role="alert">方案操作结果尚未确认。{operation.message}</p><button type="button" onClick={operation.query}>核查方案操作</button><button type="button" onClick={operation.retry}>重试原方案操作</button></div>;
 return <p role="status">正在核对方案操作…</p>;
}
