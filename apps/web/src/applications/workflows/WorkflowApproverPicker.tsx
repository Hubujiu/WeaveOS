import {useCallback,useEffect,useState} from 'react';
import {FormsDialog} from '../forms/FormsDialog';
import {applicationReadPost,ApplicationError} from '../api';
import {useResourceQuery} from '../../usePersonnelQuery';
import {validWorkflowApproverPage,type WorkflowApproverPage} from './approverContract';

type Parameters={page:number;pageSize:number;q?:string};
export function WorkflowApproverPicker({actorId,appId,viewId,schemaVersion,policyRevision,selectedIds,onConfirm,onClose,onUnauthorized,onIdentityMismatch}:{actorId:string;appId:string;viewId:string;schemaVersion:number;policyRevision:number;selectedIds:readonly string[];onConfirm:(ids:string[])=>void;onClose:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [selected,setSelected]=useState(()=>new Set(selectedIds)),[parameters,setParameters]=useState<Parameters>({page:1,pageSize:20});
 const transport=useCallback(async(body:Parameters&{queryVersion?:string},signal:AbortSignal)=>{
  await Promise.resolve();if(signal.aborted)throw signal.reason??new DOMException('Aborted','AbortError');
  const raw=await applicationReadPost<unknown>(actorId,`applications/${appId}/forms/${viewId}/workflows/approvers/search`,body,signal);
  if(!validWorkflowApproverPage(raw,body.page,body.pageSize,schemaVersion,policyRevision))throw Error('审批人候选与当前表单结构或权限不一致，请刷新后重新选择。');return raw;
 },[actorId,appId,viewId,schemaVersion,policyRevision]);
 const query=useResourceQuery<WorkflowApproverPage,Parameters>(JSON.stringify(['workflow-approvers',actorId,appId,viewId,schemaVersion,policyRevision,parameters.q??'']),parameters,{items:[],total:0,...parameters,queryVersion:'',schemaVersion,policyRevision},transport);
 useEffect(()=>{const error=query.error;if(error instanceof ApplicationError&&error.status===401)onUnauthorized();else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();},[query.error,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>{setParameters(old=>({...old,page:1,pageSize:20}));query.refresh();};
 return <FormsDialog title="选择审批人" closeLabel="关闭" onClose={onClose}><div className="workflow-approver-picker">
  <input className="admin-input workflow-approver-search" aria-label="搜索账号或部门" placeholder="搜索账号或部门" value={parameters.q??''} onChange={e=>{const q=e.target.value;setParameters({page:1,pageSize:20,...(q?{q}:{})});}}/>
  <p className="workflow-publication-caption">仅列出当前有表单读取资格的活跃账号；保存和实际审批时仍会核查权限。</p>
  <div className="workflow-approver-options">{query.loading?<p role="status">正在读取审批人…</p>:query.error?<p role="alert">{query.error.message}</p>:query.data.items.length?query.data.items.map(item=><label key={item.id}><input type="checkbox" checked={selected.has(item.id)} onChange={e=>{const checked=e.target.checked;setSelected(old=>{const next=new Set(old);if(checked)next.add(item.id);else next.delete(item.id);return next;});}}/>{item.label}</label>):<p>暂无符合资格的审批人</p>}</div>
  <div className="workflow-approver-pagination"><button type="button" className="admin-button" disabled={query.loading} onClick={refresh}>刷新</button><button type="button" className="admin-button" disabled={query.loading||!!query.error||parameters.page===1} onClick={()=>setParameters(old=>({...old,page:old.page-1}))}>上一页</button><span>第 {parameters.page} 页</span><button type="button" className="admin-button" disabled={query.loading||!!query.error||parameters.page*parameters.pageSize>=query.data.total} onClick={()=>setParameters(old=>({...old,page:old.page+1}))}>下一页</button></div>
  <p className="workflow-publication-caption">已选择 {selected.size} / 50 人，翻页保留选择；确认后仍需保存流程。</p>
  <div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={query.loading||!!query.error||selected.size<1||selected.size>50} onClick={()=>onConfirm([...selected])}>确认</button></div>
 </div></FormsDialog>;
}
