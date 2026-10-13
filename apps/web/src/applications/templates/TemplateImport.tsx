import {useContext,useEffect,useRef,useState} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {FormsDialog} from '../forms/FormsDialog';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {LeavePromptContext} from '../shell/LeavePromptContext';
import {useApplicationOperation} from '../useApplicationOperation';
import {decodeTemplateFile} from './fileInput';
import {templateReferences,type TemplateReferences} from './references';
import {validTemplatePreflight,type TemplatePreflight,type TemplateImportReceipt} from './receipts';
import fileIcon from './assets/file.svg';
import {TemplateMapping,referenceRows,type TemplateSelection,type TemplateReference} from './TemplateMapping';
import type {ReferenceCandidate} from '../forms/FieldRenderer';

export function TemplateImport({actorId,canCreate,onClose,onOpenApplication,onDirty,registerLeaveGuard,onUnauthorized,onIdentityMismatch}:{actorId:string;canCreate:boolean;onClose:()=>void;onOpenApplication:(appId:string)=>void;onDirty:(dirty:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [manifest,setManifest]=useState<Record<string,unknown>|null>(null),[fileName,setFileName]=useState('');
 const [references,setReferences]=useState<TemplateReferences|null>(null),[preflight,setPreflight]=useState<TemplatePreflight|null>(null);
 const [error,setError]=useState(''),[reading,setReading]=useState(false),[checking,setChecking]=useState(false),[closing,setClosing]=useState(false),[result,setResult]=useState<TemplateImportReceipt|null>(null);
 const [selection,setSelection]=useState<TemplateSelection>({});
 const checkedCurrent=useRef(false);
 const bindings=references?referenceRows(references).filter(row=>selection[row.id]).map(row=>({kind:row.kind,sourceId:row.sourceId,targetId:selection[row.id].id})):[];
 const mappingComplete=!!references&&bindings.length===references.userIds.length+references.departmentIds.length;
 const generation=useRef(0),readRequest=useRef<AbortController|null>(null),bypass=useRef(false),finished=useRef(false);
 const leavePrompt=useContext(LeavePromptContext);
 const operation=useApplicationOperation<TemplateImportReceipt>({actorId,scope:JSON.stringify(['template-import',actorId]),resource:{kind:'template-import'},
  confirmed:receipt=>{if(!receipt)return;finished.current=true;setResult(receipt);setManifest(null);setPreflight(null);setError('');onDirty(false);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const guard=useRef({manifest,operation,bindings});guard.current={manifest,operation,bindings};
 const abortRead=()=>{checkedCurrent.current=false;generation.current++;readRequest.current?.abort();readRequest.current=null;};
 useEffect(()=>()=>{generation.current++;readRequest.current?.abort();readRequest.current=null;},[]);
 useEffect(()=>registerLeaveGuard({kind:'template-import',actorId},createLeaveController(()=>{
  const state=guard.current,status=finished.current||bypass.current?'clean':state.operation.getPendingStatus()??(state.manifest?'draft':'clean');
  return {status,fingerprint:JSON.stringify([state.manifest,state.bindings,state.operation.packet?.operationId,status])};
 },decision=>{abortRead();if(decision==='discard'){guard.current.operation.cancelPreflight();setManifest(null);setPreflight(null);setFileName('');}bypass.current=true;onDirty(false);})),[actorId,registerLeaveGuard,onDirty]);
 const busy=operation.phase==='preflight'||operation.phase==='pending';
 const unknown=operation.phase==='unconfirmed'||operation.phase==='accepted';
 useEffect(()=>{onDirty(!result&&(!!manifest||busy||unknown));return()=>onDirty(false);},[manifest,result,busy,unknown,onDirty]);
 const cancel=()=>{abortRead();operation.cancelPreflight();bypass.current=true;onDirty(false);onClose();};
 const beginClose=()=>{abortRead();operation.cancelPreflight();setClosing(true);};
 const selectFile=async(file:File|undefined)=>{
  if(operation.getPendingStatus()||result)return;
  abortRead();const token=generation.current;bypass.current=false;setPreflight(null);setManifest(null);setReferences(null);setSelection({});setError('');setReading(false);setChecking(false);operation.dismissError();setFileName(file?.name??'');
  if(!file)return;
  if(file.size>1048576){setError('模板文件超过1MiB上限。');return;}
  setReading(true);
  try{
   const decoded=decodeTemplateFile(new Uint8Array(await file.arrayBuffer()));
   if(token!==generation.current)return;
   const refs=decoded?templateReferences(decoded):null;
   if(!decoded||!refs)throw Error('模板文件格式无效或结构不完整，请选择完整的UTF-8 JSON模板。');
   setManifest(decoded);setReferences(refs);
  }catch(cause){if(token===generation.current)setError(cause instanceof Error?cause.message:'无法读取模板文件。');}
  finally{if(token===generation.current)setReading(false);}
 };
 const selectTarget=(reference:TemplateReference,candidate:ReferenceCandidate)=>{
  if(operation.getPendingStatus()||closing||!canCreate)return;
  abortRead();setChecking(false);setPreflight(null);setError('');setSelection(old=>({...old,[reference.id]:candidate}));
 };
 const check=async()=>{
  if(!manifest||readRequest.current||operation.getPendingStatus()||!canCreate||closing||!mappingComplete)return;
  const controller=new AbortController(),token=generation.current;readRequest.current=controller;checkedCurrent.current=false;setChecking(true);setError('');setPreflight(null);
  try{
   const response=await applicationReadPost<unknown>(actorId,'application-templates/preflight',{manifest,bindings},controller.signal);
   if(controller.signal.aborted||readRequest.current!==controller||token!==generation.current)return;
   if(!validTemplatePreflight(response))throw Error('模板检查结果无效，不能确认导入。');
   checkedCurrent.current=true;setPreflight(response);
  }catch(cause){
   if(controller.signal.aborted||readRequest.current!==controller||token!==generation.current)return;
   if(cause instanceof ApplicationError&&cause.status===401)onUnauthorized();
   else if(cause instanceof ApplicationError&&cause.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
   else setError(cause instanceof Error?cause.message:'模板检查失败。');
  }finally{if(readRequest.current===controller){readRequest.current=null;setChecking(false);}}
 };
 const submit=()=>{
  if(!manifest||!preflight||!checkedCurrent.current||!mappingComplete||!canCreate||operation.getPendingStatus()||closing||result)return;
  try{void operation.start({resource:{kind:'template-import'},path:'application-templates/import',method:'POST',expectedStatus:201,body:{manifest,bindings}});}
  catch(cause){setError(cause instanceof Error?cause.message:'导入请求无效。');}
 };
 if(!canCreate&&!operation.packet&&!result)return <section className="app-state"><p role="alert">没有应用创建权限</p><button type="button" className="admin-button" onClick={onClose}>返回应用中心</button></section>;
 if(leavePrompt)return null;
 const needsMapping=!!references&&(references.userIds.length+references.departmentIds.length)>0;
 return <div onKeyDownCapture={event=>{if(event.key==='Escape')beginClose();}} onClickCapture={event=>{if((event.target as HTMLElement).closest('[data-forms-close],.forms-icon-button'))beginClose();}}>
  <FormsDialog title="导入应用模板" onClose={cancel}><div className={'template-import-panel'+(preflight||needsMapping?' template-import-confirm':'')}>
   {!result&&<><label className={needsMapping?"template-file-change":"template-file-drop"}>{!needsMapping&&<span className="template-file-icon"><img src={fileIcon} alt=""/></span>}<span>{needsMapping?'更换模板文件':'选择模板文件'}</span>{!needsMapping&&<span className="workflow-publication-caption">.json · UTF-8 · 最多1MiB</span>}<input type="file" aria-label="选择模板文件" accept=".json,application/json" disabled={busy||unknown||closing||!canCreate} onChange={event=>void selectFile(event.target.files?.[0])}/></label>
    <p>{fileName||((busy||unknown)?'已保留原导入请求':'尚未选择文件')}</p>
    {reading&&<p role="status">正在读取模板文件…</p>}
    {needsMapping&&<TemplateMapping actorId={actorId} references={references!} selection={selection} disabled={busy||unknown||closing||!canCreate} onSelect={selectTarget} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>}

   </>}
   {preflight&&!result&&<><p role="status">模板检查通过</p><p>{preflight.counts.directories} 个目录 · {preflight.counts.tables} 张表 · {preflight.counts.fields} 个字段 · {preflight.counts.forms} 个表单 · {preflight.counts.workflows} 个流程 · {preflight.counts.permissionGroups} 个权限组</p><p className="workflow-publication-caption">导入后流程保持关闭</p></>}
   {(error||operation.message)&&<p role="alert">{error||operation.message}</p>}
   {result?<><p role="status">模板已导入</p><p className="workflow-publication-caption">已创建全新应用；流程保持关闭，尚未发布或启用。</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>返回应用中心</button><button type="button" className="admin-button primary" onClick={()=>onOpenApplication(result.appId)}>打开新应用</button></div></>
   :unknown?<div className="dialog-actions template-import-recovery"><button type="button" className="admin-button" data-forms-close>关闭并保留待核查操作</button><button type="button" className="admin-button" onClick={operation.query}>核查导入结果</button><button type="button" className="admin-button primary" onClick={operation.retry}>重试原导入</button></div>
   :<div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>{busy?'关闭并保留待核查操作':'取消'}</button>{preflight?<button type="button" className="admin-button primary" disabled={busy||closing||!canCreate} onClick={submit}>{busy?'导入中…':'确认导入'}</button>:<button type="button" className="admin-button primary" aria-label="检查模板" disabled={!manifest||reading||checking||busy||!mappingComplete||closing||!canCreate} onClick={()=>void check()}>{checking?'检查中…':'检查模板'}</button>}</div>}
  </div></FormsDialog>
 </div>;
}
