import {useEffect,useRef} from 'react';
import {FormsDialog} from './FormsDialog';
import {createLeaveController,type RegisterLeaveGuard} from './leaveGuard';
import {useApplicationOperation} from '../useApplicationOperation';
import {validStructureDeletionDependencies,type StructureDeletionKind,type StructureDeletionReceipt} from './structureDeletion';
import './forms.css';
export type StructureDeletionSelection={kind:StructureDeletionKind;id:string;structureVersion:number;resourceVersion:number};
type Props={actorId:string;appId:string;selection:StructureDeletionSelection;onClose:()=>void;onConfirmed:(value:StructureDeletionReceipt)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onForbidden:()=>void;onDirtyChange:(value:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave?:(action:()=>void)=>void};
const titles={application:'删除应用？',directory:'删除目录？',table:'删除数据表？',form:'删除表单视图？'};
const descriptions={application:['将移除所选应用入口。应用仍有目录、表、表单、权限组或流程时，删除会被拒绝。','不会级联删除子资源或已有历史。'],directory:['将移除所选目录。仍有子目录、表、表单或授权依赖时，删除会被拒绝。','不会移动或级联删除其中的资源。'],table:['将移除所选数据表。仍有表单、流程、草稿、未确认命令或记录时，删除会被拒绝。','已删除但保留的记录也属于表内数据。'],form:['只移除所选表单入口，共享表、业务记录及已有历史保持。','仍有流程、草稿、未确认命令或授权依赖时，删除会被拒绝。']};
const dependencyNames:Record<string,string>={directories:'目录',tables:'数据表',forms:'表单视图',permission_groups:'权限组',workflows:'流程',drafts:'草稿',pending_commands:'未确认命令',grants:'授权',records:'记录'};
export function StructureDeletionDialog({actorId,appId,selection,onClose,onConfirmed,onUnauthorized,onIdentityMismatch,onForbidden,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const resource={kind:'structure-deletion' as const,appId,resourceKind:selection.kind,id:selection.id};
 const scope=JSON.stringify(['structure-deletion',actorId,appId,selection.kind,selection.id]);
 const operation=useApplicationOperation<StructureDeletionReceipt>({actorId,scope,resource,confirmed:value=>{if(value)onConfirmed(value);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const busy=operation.phase==='preflight'||operation.phase==='pending',unknown=operation.phase==='unconfirmed';
 const latest=useRef(operation),forbidden=useRef(onForbidden);latest.current=operation;forbidden.current=onForbidden;
 useEffect(()=>registerLeaveGuard({kind:'structure-deletion',actorId,appId,resourceKind:selection.kind,resourceId:selection.id},createLeaveController(()=>{const status=latest.current.getPendingStatus()??'clean';return{status,fingerprint:JSON.stringify([status,latest.current.packet?.operationId])};},decision=>{if(decision==='discard')latest.current.cancelPreflight();})),[actorId,appId,selection.kind,selection.id,registerLeaveGuard]);
 useEffect(()=>{onDirtyChange(busy||unknown);return()=>onDirtyChange(false);},[busy,unknown,onDirtyChange]);
 useEffect(()=>{if(operation.errorCode==='APPLICATION_FORBIDDEN')forbidden.current();},[operation.errorCode]);
 const dependencies=validStructureDeletionDependencies(operation.errorDetails,selection.kind)?operation.errorDetails.dependencies:null;
 const submit=()=>{if(busy||unknown||operation.phase==='error'||operation.getPendingStatus())return;const base=`applications/${appId}`,path=selection.kind==='application'?base+'/deletion':base+'/'+{directory:'directories',table:'tables',form:'forms'}[selection.kind]+'/'+selection.id+'/deletion';operation.start({resource,path,method:'POST',expectedStatus:200,body:{expectedStructureVersion:selection.structureVersion,expectedResourceVersion:selection.resourceVersion}});};
 return <FormsDialog title={titles[selection.kind]} onClose={onClose} busy={busy} requestClose={close=>{if(requestSectionLeave)requestSectionLeave(close);else if(!unknown)close();}}>
  <div className="record-lifecycle-dialog structure-deletion-dialog">
   {descriptions[selection.kind].map(text=><p key={text}>{text}</p>)}
   {operation.errorCode==='APPLICATION_STRUCTURE_NOT_EMPTY'?<p role="alert">{dependencies?'仍有依赖：'+dependencies.map(kind=>dependencyNames[kind]).join('、')+'。请先通过对应功能明确处理。':'依赖响应格式不正确，请重新核对。'}</p>:operation.message?<p role="alert">{operation.message}</p>:null}
   {busy&&<p role="status">正在核对删除结果…</p>}
   {unknown&&<div className="workflow-instance-buttons"><button type="button" className="admin-button" onClick={operation.query}>核查原操作</button><button type="button" className="admin-button" onClick={operation.retry}>重试原操作</button></div>}
   <div className="forms-dialog-actions"><button type="button" data-forms-close disabled={busy}>取消</button><button type="button" className="forms-primary" disabled={busy||unknown||operation.phase==='error'} onClick={submit}>确认删除</button></div>
  </div>
 </FormsDialog>;
}
