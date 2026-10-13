import {WorkflowApproverPicker} from './WorkflowApproverPicker';
import {useEffect,useRef,useState} from 'react';
import {FormsDialog} from '../forms/FormsDialog';
import type {WorkflowPublicationResult} from './publicationContract';
import {parseRuntimeView} from '../recordReadContracts';
import type {RuntimeView} from '../records/contracts';
import {useApplicationOperation} from '../useApplicationOperation';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import type {WorkflowDefinitionReceipt} from './definitionSaveContract';
import {applicationApi,ApplicationError} from '../api';
import {validWorkflowDefinition,type WorkflowDefinition,type WorkflowNode} from './definitionContract';
import {workflowLayers} from './graphLayout';

const labels={start:'开始',end:'结束',approval:'审批节点',condition:'条件节点'};
function Diagram({definition,select}:{definition:WorkflowDefinition;select:(node:WorkflowNode)=>void}){
 const layers=workflowLayers(definition.graph)!;
 const width=Math.max(692,...layers.map(layer=>layer.length*300-20)),positions=new Map<string,{x:number;y:number;width:number;height:number;node:WorkflowNode}>();let y=24;
 for(const layer of layers){const height=layer.some(n=>n.kind==='approval'||n.kind==='condition')?80:44;layer.forEach((node,i)=>{const w=node.kind==='start'||node.kind==='end'?160:280;positions.set(node.id,{x:width*(i+.5)/layer.length-w/2,y,width:w,height:node.kind==='start'||node.kind==='end'?44:80,node});});y+=height+54;}
 const height=Math.max(654,y-30);
 return <section className="workflow-definition-canvas app-surface" aria-label="流程图"><div className="workflow-graph-inner" style={{width,height}}>
  <svg className="workflow-graph-edges" width={width} height={height} role="img" aria-label="流程节点的实际连线">
   {definition.graph.edges.map(edge=>{const a=positions.get(edge.from)!,b=positions.get(edge.to)!,x1=a.x+a.width/2,y1=a.y+a.height,x2=b.x+b.width/2,y2=b.y,middle=(y1+y2)/2;return <g key={JSON.stringify(edge)}><path d={`M ${x1} ${y1} V ${middle} H ${x2} V ${y2}`} fill="none" stroke="var(--mono-line)" strokeWidth="1"/>{edge.branch&&<text x={(x1+x2)/2+6} y={middle-6}>{edge.branch==='true'?'条件成立':'条件不成立'}</text>}</g>;})}
  </svg>
  {[...positions.values()].map(({x,y,width,height,node})=><div className={'workflow-graph-node workflow-graph-node-'+node.kind} style={{left:x,top:y,width,height}} key={node.id}>
   <button type="button" title={node.id} onClick={()=>select(node)}>{labels[node.kind]}</button>
   {node.kind==='approval'&&<span className="workflow-graph-caption">{node.approval.mode==='all'?'全部同意':'任一同意'} · {node.approval.assigneeIds.length} 人</span>}
  </div>)}
 </div></section>;
}
export function WorkflowDefinitionPage({actorId,appId,viewId,flowId,onBack,onUnauthorized,onIdentityMismatch,registerLeaveGuard,requestSectionLeave,onDirtyChange}:{actorId:string;appId:string;viewId:string;flowId:string;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;onDirtyChange:(dirty:boolean)=>void}){
 const [pickerNodeId,setPickerNodeId]=useState<string|null>(null);
 const [runtime,setRuntime]=useState<RuntimeView|null>(null),[runtimeError,setRuntimeError]=useState('');
 const [definition,setDefinition]=useState<WorkflowDefinition|null>(null),[draft,setDraft]=useState<WorkflowDefinition|null>(null);
 const [error,setError]=useState(''),[loading,setLoading]=useState(true),[revision,setRevision]=useState(0),[selectedId,setSelectedId]=useState<string|null>(null),[tab,setTab]=useState<'design'|'settings'>('design'),[saved,setSaved]=useState(false);
 const scope=JSON.stringify(['workflow-definition',actorId,appId,viewId,flowId]);
 const path=`applications/${appId}/forms/${viewId}/workflows/${flowId}/definition`;
 const resource={kind:'workflow-definition' as const,appId,viewId,id:flowId};
 const operation=useApplicationOperation<WorkflowDefinitionReceipt>({actorId,scope,resource,
  confirmed:()=>{setSaved(true);setDefinition(null);setDraft(null);setSelectedId(null);setRevision(v=>v+1);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const [publicationResult,setPublicationResult]=useState<WorkflowPublicationResult|null>(null),[publicationDialog,setPublicationDialog]=useState<'confirm'|'progress'|null>(null);
 const publication=useApplicationOperation<WorkflowPublicationResult>({actorId,scope:JSON.stringify(['workflow-publication',actorId,appId,viewId,flowId]),resource:{kind:'workflow-publication',appId,viewId,id:flowId,version:definition?.candidateVersion??1},
  confirmed:result=>{if(!result)return;setPublicationResult(result);setPublicationDialog(null);if(result.status==='confirmed'){setSaved(false);setRevision(v=>v+1);}},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const publicationBusy=publication.phase==='preflight'||publication.phase==='pending';
 const publicationUnknown=publication.phase==='accepted'||publication.phase==='unconfirmed';
 const publicationRefresh=publicationResult?.status==='blocked'||publicationResult?.status==='superseded'||['WORKFLOW_CONFLICT','WORKFLOW_CLOSING','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(publication.errorCode??'');
 const dirty=!!definition&&!!draft&&JSON.stringify(definition)!==JSON.stringify(draft);
 const busy=operation.phase==='preflight'||operation.phase==='pending',unknown=operation.phase==='unconfirmed';
 const needsRefresh=['WORKFLOW_CONFLICT','WORKFLOW_CLOSING','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(operation.errorCode??'');
 const blocked=!runtime||busy||unknown||needsRefresh||publicationBusy||publicationUnknown||publicationRefresh||draft?.state==='closing';
 const guard=useRef({definition,draft,operation,publication});guard.current={definition,draft,operation,publication};
 useEffect(()=>{onDirtyChange(dirty||busy||unknown||publicationBusy||publicationUnknown);return()=>onDirtyChange(false);},[dirty,busy,unknown,publicationBusy,publicationUnknown,onDirtyChange]);
 useEffect(()=>registerLeaveGuard({kind:'workflow-definition',actorId,appId,viewId,flowId},createLeaveController(()=>{
  const latest=guard.current,pending=latest.operation.getPendingStatus()??latest.publication.getPendingStatus();
  const status=pending??(latest.definition&&latest.draft&&JSON.stringify(latest.definition)!==JSON.stringify(latest.draft)?'draft':'clean');
  return {status,fingerprint:JSON.stringify([latest.draft,latest.operation.packet?.operationId,latest.publication.packet?.operationId,status])};
 },decision=>{if(decision==='discard'){guard.current.operation.cancelPreflight();guard.current.publication.cancelPreflight();setDraft(guard.current.definition);}})),[scope,registerLeaveGuard]);
 useEffect(()=>{const controller=new AbortController();let current=true;setDefinition(null);setDraft(null);setSelectedId(null);setError('');setLoading(true);
  void(async()=>{await Promise.resolve();if(!current)return;try{
   const raw=await applicationApi<unknown>(actorId,path,'GET',undefined,controller.signal);
   if(!current)return;if(!validWorkflowDefinition(raw,flowId))throw Error('流程定义响应格式不正确，请重新读取。');setDefinition(raw);setDraft(raw);
  }catch(e){if(!current)return;setDefinition(null);setDraft(null);setSelectedId(null);if(e instanceof ApplicationError&&e.status===401)onUnauthorized();else if(e instanceof ApplicationError&&e.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();setError(e instanceof Error?e.message:'读取流程失败');
  }finally{if(current)setLoading(false);}})();return()=>{current=false;controller.abort();};
 },[actorId,path,flowId,revision,onUnauthorized,onIdentityMismatch]);
 useEffect(()=>{const controller=new AbortController();let current=true;setRuntime(null);setRuntimeError('');
  void(async()=>{await Promise.resolve();if(!current)return;try{
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal);
   if(current)setRuntime(parseRuntimeView(raw,{appId,viewId}));
  }catch(e){if(!current)return;if(e instanceof ApplicationError&&e.status===401)onUnauthorized();else if(e instanceof ApplicationError&&e.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();setRuntimeError(e instanceof Error?e.message:'读取当前表单结构失败');}
  })();return()=>{current=false;controller.abort();};
 },[actorId,appId,viewId,revision,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>requestSectionLeave(()=>{operation.dismissError();publication.dismissError();setPublicationResult(null);setSaved(false);setRevision(v=>v+1);});
 const save=()=>{
  if(!definition||!draft||!runtime||!dirty||blocked||operation.getPendingStatus())return;
  setSaved(false);operation.start({resource,path,method:'PUT',expectedStatus:200,body:{name:draft.name,expectedRevision:definition.revision,expectedSchemaVersion:runtime.schemaVersion,graph:draft.graph,allowWithdraw:draft.allowWithdraw,triggers:draft.triggers}});
 };
 const publish=()=>{
  if(!definition||!runtime||dirty||blocked||operation.getPendingStatus()||publication.getPendingStatus())return;
  setPublicationResult(null);setPublicationDialog('progress');publication.start({resource:{kind:'workflow-publication',appId,viewId,id:flowId,version:definition.candidateVersion},path:path.replace(/\/definition$/,'/publish'),method:'POST',expectedStatus:202,body:{expectedRevision:definition.revision,expectedSchemaVersion:runtime.schemaVersion}});
 };
 const publicationStatus=publicationResult?({confirmed:'发布已确认',blocked:'发布未生效',superseded:'候选版本已被替代',pending:'发布处理中',unknown:'发布结果待确认'}[publicationResult.status]):publication.phase==='accepted'?(publication.message==='流程发布处理中'?'发布处理中':'发布结果待确认'):null;
 const publicationFeedback=<>{publicationStatus&&<p role="status">{publicationStatus}</p>}{publication.message&&publication.phase!=='accepted'&&<p role="alert">{publication.message}</p>}{publicationUnknown&&<div className="workflow-definition-actions"><button type="button" className="admin-button" onClick={publication.query}>核查发布结果</button><button type="button" className="admin-button" onClick={publication.retry}>重试原发布</button></div>}</>;
 const accessLost=publication.errorCode==='APPLICATION_FORBIDDEN'||publication.errorCode==='APPLICATION_NOT_FOUND'||operation.errorCode==='APPLICATION_FORBIDDEN'||operation.errorCode==='APPLICATION_NOT_FOUND';
 const current=accessLost?null:draft;
 const selected=current?.graph.nodes.find(n=>n.id===selectedId)??null;
 const changeMode=(mode:'all'|'any')=>{if(blocked||!selected||selected.kind!=='approval')return;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===selected.id&&node.kind==='approval'?{...node,approval:{...node.approval,mode}}:node)}}:previous);};
 return <section className="workflow-definition-page">
  <div className="app-page-heading"><div><p className="workflow-definition-context">当前表单 / 流程</p><h1>{definition?.name??'流程设计'}</h1></div><div className="workflow-definition-actions">{current&&<button type="button" className="admin-button primary workflow-definition-save" disabled={!dirty||blocked} onClick={save}>保存</button>}{current&&<button type="button" className="admin-button workflow-definition-publish" disabled={dirty||blocked||loading} onClick={()=>setPublicationDialog('confirm')}>发布</button>}<button type="button" className="admin-button" onClick={onBack}>返回流程列表</button></div></div>
  {publicationDialog!=='progress'&&publicationFeedback}
  {runtimeError&&definition&&<p role="alert">{runtimeError}</p>}
  {runtime&&definition&&runtime.schemaVersion!==definition.schemaVersion&&<p role="status">候选结构版本 {definition.schemaVersion}，当前结构版本 {runtime.schemaVersion}；请核对配置后明确保存。</p>}
  {saved&&<p role="status">定义已保存</p>}
  {operation.message&&<p role="alert">{operation.message}</p>}
  {unknown&&<div className="workflow-definition-actions"><button type="button" className="admin-button" onClick={operation.query}>核查保存结果</button><button type="button" className="admin-button" onClick={operation.retry}>重试原保存</button></div>}
  {(needsRefresh||publicationRefresh)&&<button type="button" className="admin-button" onClick={refresh}>刷新定义</button>}
  {loading?<section className="app-surface app-state" role="status">正在读取流程定义…</section>:error?<section className="app-surface app-state"><p role="alert">{error}</p><button type="button" className="admin-button" onClick={refresh}>重新读取</button></section>:current&&<>
   <div className="workflow-definition-version"><span>候选版本 {current.candidateVersion}</span><span>{current.currentVersion?'已发布版本 '+current.currentVersion:'尚未发布'}</span></div>
   <div role="tablist" aria-label="流程配置" className="workflow-definition-tabs"><button type="button" role="tab" aria-selected={tab==='design'} onClick={()=>setTab('design')}>流程设计</button><button type="button" role="tab" aria-selected={tab==='settings'} onClick={()=>setTab('settings')}>触发与设置</button></div>
   {tab==='design'?<div className="workflow-definition-workspace"><Diagram definition={current} select={node=>setSelectedId(node.id)}/><section className="app-surface workflow-node-settings" aria-label="节点设置"><h2>{selected?labels[selected.kind]:'节点设置'}</h2>{selected?<><p className="workflow-node-id">{selected.id}</p>{selected.kind==='approval'?<><h3>审批人</h3><button type="button" className="admin-button" disabled={blocked} onClick={()=>setPickerNodeId(selected.id)}>选择审批人</button><ul>{selected.approval.assigneeIds.map(id=><li key={id}>{id}</li>)}</ul><label className="forms-field-label" htmlFor="workflow-approval-mode">通过规则</label><select id="workflow-approval-mode" className="admin-input" value={selected.approval.mode} disabled={blocked} onChange={e=>changeMode(e.target.value as 'all'|'any')}><option value="all">全部同意</option><option value="any">任一同意</option></select><h3>可编辑字段</h3>{runtime?<div className="workflow-editable-fields">{runtime.fields.map(field=><label key={field.id}><input type="checkbox" aria-label={field.name} disabled={blocked} checked={selected.approval.editableFieldIds.includes(field.id)} onChange={e=>{const checked=e.target.checked;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===selected.id&&node.kind==='approval'?{...node,approval:{...node.approval,editableFieldIds:checked?[...node.approval.editableFieldIds,field.id]:node.approval.editableFieldIds.filter(id=>id!==field.id)}}:node)}}:previous);}}/>{field.name}<small>{field.id}</small></label>)}{selected.approval.editableFieldIds.filter(id=>!runtime.fields.some(field=>field.id===id)).map(id=><p key={id}>字段已不可用：{id}</p>)}</div>:selected.approval.editableFieldIds.length?<ul>{selected.approval.editableFieldIds.map(id=><li key={id}>{id}</li>)}</ul>:<p>无可编辑字段</p>}</>:selected.kind==='condition'?<><h3>判断条件</h3><pre>{JSON.stringify(selected.condition,null,2)}</pre></>:<p>{selected.kind==='start'?'流程从此节点开始':'流程在此节点结束'}</p>}</>:<p>选择节点查看当前配置。</p>}</section></div>:<section className="app-surface workflow-definition-settings" aria-label="触发与设置"><h2>触发方式</h2>{current.triggers.length?<ul>{current.triggers.map(t=><li key={t.event}><strong>{t.event==='manual'?'手动':t.event==='record.created'?'记录新增':'记录修改'}</strong>{t.condition===null?<p>无附加条件</p>:<pre>{JSON.stringify(t.condition,null,2)}</pre>}</li>)}</ul>:<p>未配置触发方式</p>}<h2>撤回设置</h2><label className="workflow-definition-withdraw"><input type="checkbox" checked={current.allowWithdraw} disabled={blocked} onChange={e=>setDraft(previous=>previous?{...previous,allowWithdraw:e.target.checked}:previous)}/>允许发起人撤回</label><p>{current.allowWithdraw?'允许撤回':'不允许撤回'}</p></section>}
  </>}
  {pickerNodeId&&runtime&&selected?.kind==='approval'&&selected.id===pickerNodeId&&<WorkflowApproverPicker key={pickerNodeId} actorId={actorId} appId={appId} viewId={viewId} schemaVersion={runtime.schemaVersion} policyRevision={runtime.policyRevision} selectedIds={selected.approval.assigneeIds} onClose={()=>setPickerNodeId(null)} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onConfirm={ids=>{if(blocked)return;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===pickerNodeId&&node.kind==='approval'?{...node,approval:{...node.approval,assigneeIds:ids}}:node)}}:previous);setPickerNodeId(null);}}/>}
  {publicationDialog==='confirm'&&definition&&<FormsDialog title="发布流程" closeLabel="关闭" onClose={()=>setPublicationDialog(null)}><div className="workflow-publication-confirm"><h3>{definition.name}</h3><p>候选版本 {definition.candidateVersion}</p><p className="workflow-publication-caption">{definition.currentVersion?'当前已发布版本 '+definition.currentVersion:'尚未发布'}</p><p className="workflow-publication-caption">发布确认后，新发起使用候选版本，在途流程保持原结构。流程启用状态不会自动改变。</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={dirty||blocked} onClick={publish}>确认发布</button></div></div></FormsDialog>}
  {publicationDialog==='progress'&&<FormsDialog title="正在发布流程" closeLabel="关闭" onClose={()=>setPublicationDialog(null)}><div className="workflow-publication-progress"><p>正在部署并核对流程版本…</p><div className="workflow-publication-indeterminate" role="progressbar" aria-label="发布结果尚未确认"><span/></div>{publicationFeedback}<p className="workflow-publication-caption">关闭此窗口不会取消已发送的发布，仍可核查原操作。</p></div></FormsDialog>}
 </section>;
}
