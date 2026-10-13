import type {WorkflowDeletionResult} from './deletionContract';
import {validWorkflowDefinitionRequest} from './definitionSaveContract';
import {insertWorkflowNode,removeWorkflowNode} from './graphEditing';
import publicationSuccessIcon from './assets/publication-success.svg';
import {WorkflowTriggerEditor} from './WorkflowTriggerEditor';
import {WorkflowConditionEditor} from './WorkflowConditionEditor';
import {validateResourceFilter,type ResourceFilterField,type ResourceFilterGroup} from '../../QueryFilterState';
import type {WorkflowManagementAction,WorkflowManagementReceipt} from './managementContract';
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
import {validWorkflowDefinition,type WorkflowDefinition,type WorkflowNode,type WorkflowGraph} from './definitionContract';
import {workflowLayers} from './graphLayout';

const labels={start:'开始',end:'结束',approval:'审批节点',condition:'条件节点'};
function Diagram({definition,fallback,select}:{definition:WorkflowDefinition;fallback:WorkflowDefinition;select:(node:WorkflowNode)=>void}){
 const validLayers=workflowLayers(definition.graph);
 const lastValid=useRef(workflowLayers(fallback.graph)!);
 if(validLayers)lastValid.current=validLayers;
 // Invalid condition input does not change topology. Retain the latest unsaved
 // topology, not the older persisted graph, while rendering current node values.
 const currentNodes=new Map(definition.graph.nodes.map(node=>[node.id,node]));
 const layers=(validLayers??lastValid.current).map(layer=>layer.map(node=>currentNodes.get(node.id)??node));
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
export function WorkflowDefinitionPage({actorId,appId,viewId,flowId,createMode=false,onCreated,onBack,onUnauthorized,onIdentityMismatch,registerLeaveGuard,requestSectionLeave,onDirtyChange}:{actorId:string;appId:string;viewId:string;flowId:string;createMode?:boolean;onCreated?:()=>void;onBack:()=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void;onDirtyChange:(dirty:boolean)=>void}){
 const publishButton=useRef<HTMLButtonElement|null>(null);
 const [insertion,setInsertion]=useState<{graph:WorkflowGraph;edge:WorkflowGraph['edges'][number]}|null>(null);
 const [removal,setRemoval]=useState<{graph:WorkflowGraph;nodeId:string}|null>(null);
 const [conditionInsertion,setConditionInsertion]=useState<{graph:WorkflowGraph;edge:WorkflowGraph['edges'][number];condition:ResourceFilterGroup}|null>(null);
 const [pickerNodeId,setPickerNodeId]=useState<string|null>(null);
 const [runtime,setRuntime]=useState<RuntimeView|null>(null),[runtimeError,setRuntimeError]=useState('');
 const [definition,setDefinition]=useState<WorkflowDefinition|null>(null),[draft,setDraft]=useState<WorkflowDefinition|null>(null);
 const [error,setError]=useState(''),[loading,setLoading]=useState(true),[revision,setRevision]=useState(0),[selectedId,setSelectedId]=useState<string|null>(null),[tab,setTab]=useState<'design'|'settings'>('design'),[saved,setSaved]=useState(false);
 const scope=JSON.stringify(['workflow-definition',actorId,appId,viewId,flowId]);
 const path=`applications/${appId}/forms/${viewId}/workflows/${flowId}/definition`;
 const resource={kind:'workflow-definition' as const,appId,viewId,id:flowId};
 const operation=useApplicationOperation<WorkflowDefinitionReceipt>({actorId,scope,resource,
  confirmed:()=>{setSaved(true);setDefinition(null);setDraft(null);setSelectedId(null);if(createMode){guard.current.definition=null;guard.current.draft=null;onDirtyChange(false);onCreated?.();}else setRevision(v=>v+1);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const [publicationResult,setPublicationResult]=useState<WorkflowPublicationResult|null>(null),[publicationDialog,setPublicationDialog]=useState<'confirm'|'progress'|'success'|null>(null);
 const publication=useApplicationOperation<WorkflowPublicationResult>({actorId,scope:JSON.stringify(['workflow-publication',actorId,appId,viewId,flowId]),resource:{kind:'workflow-publication',appId,viewId,id:flowId,version:Math.max(1,definition?.candidateVersion??1)},
  confirmed:result=>{if(!result)return;setPublicationResult(result);setPublicationDialog(result.status==='confirmed'?'success':null);if(result.status==='confirmed'){setSaved(false);setRevision(v=>v+1);}},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const publicationBusy=publication.phase==='preflight'||publication.phase==='pending';
 const publicationUnknown=publication.phase==='accepted'||publication.phase==='unconfirmed';
 const publicationRefresh=publicationResult?.status==='blocked'||publicationResult?.status==='superseded'||['WORKFLOW_CONFLICT','WORKFLOW_CLOSING','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(publication.errorCode??'');
 const [managementResult,setManagementResult]=useState<WorkflowManagementReceipt|null>(null),[closeConfirmation,setCloseConfirmation]=useState(false);
 const managementResource=(action:WorkflowManagementAction)=>({kind:'workflow-management' as const,appId,viewId,id:flowId,action});
 const managementOptions={actorId,scope:JSON.stringify(['workflow-management',actorId,appId,viewId,flowId]),confirmed:(result:WorkflowManagementReceipt|undefined)=>{if(!result)return;setManagementResult(result);setSaved(false);setRevision(v=>v+1);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch};
 // Both fixed action scopes stay mounted so a newer server state cannot hide
 // an earlier close/enable operation that still needs confirmation.
 const closeOperation=useApplicationOperation<WorkflowManagementReceipt>({...managementOptions,resource:managementResource('close')});
 const enableOperation=useApplicationOperation<WorkflowManagementReceipt>({...managementOptions,resource:managementResource('enable')});
 const managementBusy=[closeOperation,enableOperation].some(op=>op.phase==='preflight'||op.phase==='pending');
 const pendingManagement=closeOperation.phase==='unconfirmed'?closeOperation:enableOperation.phase==='unconfirmed'?enableOperation:null;
 const managementUnknown=!!pendingManagement;
 const managementRefresh=[closeOperation,enableOperation].some(op=>['WORKFLOW_CONFLICT','WORKFLOW_CLOSING','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(op.errorCode??''));
 const [deletionConfirmation,setDeletionConfirmation]=useState(false),[deleted,setDeleted]=useState(false);
 const deletionResource={kind:'workflow-deletion' as const,appId,viewId,id:flowId};
 // Remains mounted when definition GET returns 404: that is not a deletion receipt.
 const deletion=useApplicationOperation<WorkflowDeletionResult>({actorId,scope:JSON.stringify(['workflow-deletion',actorId,appId,viewId,flowId]),resource:deletionResource,
  confirmed:result=>{if(result?.status!=='deleted')return;setDeleted(true);setDefinition(null);setDraft(null);setSelectedId(null);setSaved(false);setManagementResult(null);setPublicationResult(null);setError('');guard.current.definition=null;guard.current.draft=null;onDirtyChange(false);},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const deletionBusy=deletion.phase==='preflight'||deletion.phase==='pending';
 const deletionUnknown=deletion.phase==='accepted'||deletion.phase==='unconfirmed';
 const deletionRefresh=['WORKFLOW_CONFLICT','WORKFLOW_CLOSING','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(deletion.errorCode??'');
 const dirty=!!definition&&!!draft&&JSON.stringify(definition)!==JSON.stringify(draft);
 const busy=operation.phase==='preflight'||operation.phase==='pending',unknown=operation.phase==='unconfirmed';
 const needsRefresh=['WORKFLOW_CONFLICT','WORKFLOW_CLOSING','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_POLICY_CONFLICT'].includes(operation.errorCode??'');
 const conditionFields:ResourceFilterField[]=runtime?.fields.map(field=>({id:field.id,label:field.name,kind:field.kind,operators:field.query.operators,optionIds:field.input.options?.map(option=>option.id),choices:field.input.options?.map(option=>({value:option.id,label:option.label}))}))??[];
 const conditionIssues=runtime&&draft?[...draft.graph.nodes.flatMap(node=>node.kind==='condition'?validateResourceFilter(conditionFields,node.condition).issues:[]),...draft.triggers.flatMap(trigger=>validateResourceFilter(conditionFields,trigger.condition).issues)]:[];
 const invalidCondition=conditionIssues.length>0;
 const invalidName=!!draft&&(!draft.name.trim()||[...draft.name].length>100);
 const operationBlocked=!runtime||busy||unknown||needsRefresh||publicationBusy||publicationUnknown||publicationRefresh||managementBusy||managementUnknown||managementRefresh||deletionBusy||deletionUnknown||deletionRefresh||deleted;
 const blocked=operationBlocked||draft?.state==='closing';
 const guard=useRef({definition,draft,operation,publication,closeOperation,enableOperation,deletion});guard.current={definition,draft,operation,publication,closeOperation,enableOperation,deletion};
 useEffect(()=>{onDirtyChange(dirty||busy||unknown||publicationBusy||publicationUnknown||managementBusy||managementUnknown||deletionBusy||deletionUnknown);return()=>onDirtyChange(false);},[dirty,busy,unknown,publicationBusy,publicationUnknown,managementBusy,managementUnknown,deletionBusy,deletionUnknown,onDirtyChange]);
 useEffect(()=>registerLeaveGuard({kind:'workflow-definition',actorId,appId,viewId,flowId},createLeaveController(()=>{
  const latest=guard.current,pending=latest.operation.getPendingStatus()??latest.publication.getPendingStatus()??latest.closeOperation.getPendingStatus()??latest.enableOperation.getPendingStatus()??latest.deletion.getPendingStatus();
  const status=pending??(latest.definition&&latest.draft&&JSON.stringify(latest.definition)!==JSON.stringify(latest.draft)?'draft':'clean');
  return {status,fingerprint:JSON.stringify([latest.draft,latest.operation.packet?.operationId,latest.publication.packet?.operationId,latest.closeOperation.packet?.operationId,latest.enableOperation.packet?.operationId,latest.deletion.packet?.operationId,status])};
 },decision=>{if(decision==='discard'){guard.current.operation.cancelPreflight();guard.current.publication.cancelPreflight();guard.current.closeOperation.cancelPreflight();guard.current.enableOperation.cancelPreflight();guard.current.deletion.cancelPreflight();setDraft(guard.current.definition);}})),[scope,registerLeaveGuard]);
 useEffect(()=>{const controller=new AbortController();let current=true;setDefinition(null);setDraft(null);setSelectedId(null);setError('');setLoading(true);
  if(createMode){
   const start=crypto.randomUUID(),end=crypto.randomUUID();
   // Explicit local initial state, never parsed or presented as a server version.
   const initial:WorkflowDefinition={id:flowId,name:'',revision:0,currentVersion:0,candidateVersion:0,state:'disabled',schemaVersion:0,allowWithdraw:false,triggers:[],graph:{version:1,nodes:[{id:start,kind:'start'},{id:end,kind:'end'}],edges:[{from:start,to:end}]}};
   const packet=operation.packet,body=packet?.body;
   let recovered=initial;
   if(packet&&body){const {operationId:_,...input}=body;
    if(input.expectedRevision===0&&validWorkflowDefinitionRequest(resource,{...packet,body:input}))recovered={...initial,name:input.name as string,graph:input.graph as WorkflowGraph,triggers:input.triggers as WorkflowDefinition['triggers'],allowWithdraw:input.allowWithdraw as boolean};
   }
   setDefinition(initial);setDraft(recovered);setLoading(false);
   return()=>{current=false;controller.abort();};
  }
  void(async()=>{await Promise.resolve();if(!current)return;try{
   const raw=await applicationApi<unknown>(actorId,path,'GET',undefined,controller.signal);
   if(!current)return;if(!validWorkflowDefinition(raw,flowId))throw Error('流程定义响应格式不正确，请重新读取。');setDefinition(raw);setDraft(raw);
  }catch(e){if(!current)return;setDefinition(null);setDraft(null);setSelectedId(null);if(e instanceof ApplicationError&&e.status===401)onUnauthorized();else if(e instanceof ApplicationError&&e.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();setError(e instanceof Error?e.message:'读取流程失败');
  }finally{if(current)setLoading(false);}})();return()=>{current=false;controller.abort();};
 },[actorId,path,flowId,createMode,revision,onUnauthorized,onIdentityMismatch]);
 useEffect(()=>{const controller=new AbortController();let current=true;setRuntime(null);setRuntimeError('');
  void(async()=>{await Promise.resolve();if(!current)return;try{
   const raw=await applicationApi<unknown>(actorId,`applications/${appId}/forms/${viewId}/runtime`,'GET',undefined,controller.signal);
   if(current)setRuntime(parseRuntimeView(raw,{appId,viewId}));
  }catch(e){if(!current)return;if(e instanceof ApplicationError&&e.status===401)onUnauthorized();else if(e instanceof ApplicationError&&e.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();setRuntimeError(e instanceof Error?e.message:'读取当前表单结构失败');}
  })();return()=>{current=false;controller.abort();};
 },[actorId,appId,viewId,revision,onUnauthorized,onIdentityMismatch]);
 const refresh=()=>requestSectionLeave(()=>{operation.dismissError();publication.dismissError();closeOperation.dismissError();enableOperation.dismissError();deletion.dismissError();setPublicationResult(null);setSaved(false);setRevision(v=>v+1);});
 const pendingStatus=()=>operation.getPendingStatus()??publication.getPendingStatus()??closeOperation.getPendingStatus()??enableOperation.getPendingStatus()??deletion.getPendingStatus();
 const manage=(action:WorkflowManagementAction)=>{
  if(createMode||!definition||dirty||operationBlocked||pendingStatus()||action==='enable'&&(definition.currentVersion===0||definition.currentVersion!==definition.candidateVersion))return;
  setCloseConfirmation(false);setManagementResult(null);closeOperation.dismissError();enableOperation.dismissError();
  (action==='close'?closeOperation:enableOperation).start({resource:managementResource(action),path:path.replace(/\/definition$/,'/'+action),method:'POST',expectedStatus:200,body:{expectedRevision:definition.revision}});
 };
 const deleteFlow=()=>{
  if(createMode||!definition||dirty||operationBlocked||pendingStatus())return;
  setDeletionConfirmation(false);setManagementResult(null);setPublicationResult(null);setSaved(false);
  deletion.start({resource:deletionResource,path:path.replace(/\/definition$/,'/delete'),method:'POST',expectedStatus:202,body:{expectedRevision:definition.revision}});
 };
 const save=()=>{
  if(!definition||!draft||!runtime||!dirty||blocked||invalidCondition||invalidName||pendingStatus())return;
  setSaved(false);operation.start({resource,path,method:'PUT',expectedStatus:createMode?201:200,body:{name:draft.name,expectedRevision:definition.revision,expectedSchemaVersion:runtime.schemaVersion,graph:draft.graph,allowWithdraw:draft.allowWithdraw,triggers:draft.triggers}});
 };
 const publish=()=>{
  if(createMode||!definition||!runtime||dirty||blocked||pendingStatus())return;
  setPublicationResult(null);setPublicationDialog('progress');publication.start({resource:{kind:'workflow-publication',appId,viewId,id:flowId,version:definition.candidateVersion},path:path.replace(/\/definition$/,'/publish'),method:'POST',expectedStatus:202,body:{expectedRevision:definition.revision,expectedSchemaVersion:runtime.schemaVersion}});
 };
 const publicationStatus=publicationResult?({confirmed:'发布已确认',blocked:'发布未生效',superseded:'候选版本已被替代',pending:'发布处理中',unknown:'发布结果待确认'}[publicationResult.status]):publication.phase==='accepted'?(publication.message==='流程发布处理中'?'发布处理中':'发布结果待确认'):null;
 const publicationFeedback=<>{publicationStatus&&<p role="status">{publicationStatus}</p>}{publication.message&&publication.phase!=='accepted'&&<p role="alert">{publication.message}</p>}{publicationUnknown&&<div className="workflow-definition-actions"><button type="button" className="admin-button" onClick={publication.query}>核查发布结果</button><button type="button" className="admin-button" onClick={publication.retry}>重试原发布</button></div>}</>;
 const managementAccessLost=[closeOperation,enableOperation].some(op=>op.errorCode==='APPLICATION_FORBIDDEN'||op.errorCode==='APPLICATION_NOT_FOUND');
 const accessLost=deletion.errorCode==='APPLICATION_FORBIDDEN'||deletion.errorCode==='APPLICATION_NOT_FOUND'||managementAccessLost||publication.errorCode==='APPLICATION_FORBIDDEN'||publication.errorCode==='APPLICATION_NOT_FOUND'||operation.errorCode==='APPLICATION_FORBIDDEN'||operation.errorCode==='APPLICATION_NOT_FOUND';
 const current=accessLost||deleted?null:draft;
 const selected=current?.graph.nodes.find(n=>n.id===selectedId)??null;
 const removable=!!current&&!!selected&&removeWorkflowNode(current.graph,selected.id)!==null;
 const removeNode=()=>{
  if(!removal||!draft||draft.graph!==removal.graph||blocked||invalidCondition||pendingStatus()){setRemoval(null);return;}
  const graph=removeWorkflowNode(draft.graph,removal.nodeId);
  if(graph){setDraft({...draft,graph});setSelectedId(null);}
  setRemoval(null);
 };
 const beginApprovalInsertion=(branch?:'true'|'false')=>{
  if(!draft||!selected||blocked||invalidCondition||pendingStatus()||draft.graph.nodes.length>=100)return;
  const edges=draft.graph.edges.filter(edge=>edge.from===selected.id&&(branch===undefined||edge.branch===branch));
  if(edges.length===1)setInsertion({graph:draft.graph,edge:edges[0]});
 };
 const insertApproval=(ids:string[])=>{
  if(!insertion||!draft||draft.graph!==insertion.graph||blocked||invalidCondition||pendingStatus()){setInsertion(null);return;}
  const node:WorkflowNode={id:crypto.randomUUID(),kind:'approval',approval:{mode:'all',assigneeIds:ids,editableFieldIds:[]}};
  const graph=insertWorkflowNode(draft.graph,insertion.edge,node);
  if(graph){setDraft({...draft,graph});setSelectedId(node.id);}
  setInsertion(null);
 };
 const beginConditionInsertion=()=>{
  if(!draft||!selected||blocked||invalidCondition||pendingStatus()||draft.graph.nodes.length>=100)return;
  const edges=draft.graph.edges.filter(edge=>edge.from===selected.id);
  if(edges.length===1)setConditionInsertion({graph:draft.graph,edge:edges[0],condition:{operator:'and',children:[{fieldId:'',operator:'eq',value:''}]}});
 };
 const insertCondition=()=>{
  if(!conditionInsertion||!draft||draft.graph!==conditionInsertion.graph||blocked||invalidCondition||pendingStatus()||validateResourceFilter(conditionFields,conditionInsertion.condition).issues.length)return;
  const node:WorkflowNode={id:crypto.randomUUID(),kind:'condition',condition:conditionInsertion.condition};
  const graph=insertWorkflowNode(draft.graph,conditionInsertion.edge,node);
  if(graph){setDraft({...draft,graph});setSelectedId(node.id);}
  setConditionInsertion(null);
 };
 const changeMode=(mode:'all'|'any')=>{if(blocked||!selected||selected.kind!=='approval')return;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===selected.id&&node.kind==='approval'?{...node,approval:{...node.approval,mode}}:node)}}:previous);};
 return <section className="workflow-definition-page">
  <div className="app-page-heading"><div><p className="workflow-definition-context">当前表单 / 流程</p><h1>{createMode?'新建流程':definition?.name??'流程设计'}</h1></div><div className="workflow-definition-actions">{current&&<button type="button" className="admin-button primary workflow-definition-save" disabled={!dirty||blocked||invalidCondition||invalidName} onClick={save}>保存</button>}{current&&<button type="button" ref={publishButton} className="admin-button workflow-definition-publish" disabled={createMode||dirty||blocked||loading} onClick={()=>setPublicationDialog('confirm')}>发布</button>}<button type="button" className="admin-button" onClick={onBack}>返回流程列表</button></div></div>
  {deleted&&<p role="status">流程已删除</p>}
  {deletion.message&&<p role={deletion.phase==='accepted'?'status':'alert'}>{deletion.message}</p>}
  {deletionUnknown&&<div className="workflow-definition-actions"><button type="button" className="admin-button" onClick={deletion.query}>核查删除结果</button><button type="button" className="admin-button" onClick={deletion.retry}>重试原删除</button></div>}
  {publicationDialog!=='progress'&&publicationFeedback}
  {managementResult&&<p role="status">{{enabled:'已启用',disabled:'已关闭',closing:'关闭中'}[managementResult.state]}</p>}
  {closeOperation.message&&<p role="alert">{closeOperation.message}</p>}{enableOperation.message&&<p role="alert">{enableOperation.message}</p>}
  {pendingManagement&&<div className="workflow-definition-actions"><button type="button" className="admin-button" onClick={pendingManagement.query}>核查管理结果</button><button type="button" className="admin-button" onClick={pendingManagement.retry}>重试原管理操作</button></div>}
  {runtimeError&&definition&&<p role="alert">{runtimeError}</p>}
  {!createMode&&runtime&&definition&&runtime.schemaVersion!==definition.schemaVersion&&<p role="status">候选结构版本 {definition.schemaVersion}，当前结构版本 {runtime.schemaVersion}；请核对配置后明确保存。</p>}
  {invalidName&&dirty&&<p role="alert">流程名称不能为空且不能超过100个字符。</p>}
  {conditionIssues.length>0&&<div role="alert">{conditionIssues.map((issue,index)=><p key={index}>{issue.message}</p>)}</div>}
  {saved&&<p role="status">定义已保存</p>}
  {operation.message&&<p role="alert">{operation.message}</p>}
  {unknown&&<div className="workflow-definition-actions"><button type="button" className="admin-button" onClick={operation.query}>核查保存结果</button><button type="button" className="admin-button" onClick={operation.retry}>重试原保存</button></div>}
  {(needsRefresh||publicationRefresh||managementRefresh||deletionRefresh)&&<button type="button" className="admin-button" onClick={refresh}>刷新定义</button>}
  {deleted?null:loading?<section className="app-surface app-state" role="status">正在读取流程定义…</section>:error?<section className="app-surface app-state"><p role="alert">{error}</p><button type="button" className="admin-button" onClick={refresh}>重新读取</button></section>:current&&<>
   <div className="workflow-definition-version">{createMode?<span>尚未保存</span>:<><span>候选版本 {current.candidateVersion}</span><span>{current.currentVersion?'已发布版本 '+current.currentVersion:'尚未发布'}</span></>}</div>
   <div role="tablist" aria-label="流程配置" className="workflow-definition-tabs"><button type="button" role="tab" aria-selected={tab==='design'} onClick={()=>setTab('design')}>流程设计</button><button type="button" role="tab" aria-selected={tab==='settings'} onClick={()=>setTab('settings')}>触发与设置</button></div>
   {tab==='design'?<div className="workflow-definition-workspace"><Diagram definition={current} fallback={definition!} select={node=>setSelectedId(node.id)}/><section className="app-surface workflow-node-settings" aria-label="节点设置"><h2>{selected?labels[selected.kind]:'节点设置'}</h2>{selected?<><p className="workflow-node-id">{selected.id}</p>{(selected.kind==='start'||selected.kind==='approval')&&<button type="button" className="admin-button" disabled={blocked||invalidCondition||current.graph.nodes.length>=100} onClick={()=>beginApprovalInsertion()}>在此节点后添加审批节点</button>}{(selected.kind==='start'||selected.kind==='approval')&&<button type="button" className="admin-button" disabled={blocked||invalidCondition||current.graph.nodes.length>=100} onClick={beginConditionInsertion}>在此节点后添加条件节点</button>}{selected.kind==='condition'&&(['true','false'] as const).map(branch=><button type="button" className="admin-button" disabled={blocked||invalidCondition||current.graph.nodes.length>=100} key={branch} onClick={()=>beginApprovalInsertion(branch)}>{branch==='true'?'在条件成立分支添加审批节点':'在条件不成立分支添加审批节点'}</button>)}{(selected.kind==='approval'||selected.kind==='condition')&&<><button type="button" className="admin-button" disabled={blocked||invalidCondition||!removable} onClick={()=>setRemoval({graph:current.graph,nodeId:selected.id})}>移除节点</button>{selected.kind==='condition'&&new Set(current.graph.edges.filter(edge=>edge.from===selected.id).map(edge=>edge.to)).size>1&&<p>两个分支的后继不同，请先明确调整分支。</p>}</>}{selected.kind==='approval'?<><h3>审批人</h3><button type="button" className="admin-button" disabled={blocked} onClick={()=>setPickerNodeId(selected.id)}>选择审批人</button><ul>{selected.approval.assigneeIds.map(id=><li key={id}>{id}</li>)}</ul><label className="forms-field-label" htmlFor="workflow-approval-mode">通过规则</label><select id="workflow-approval-mode" className="admin-input" value={selected.approval.mode} disabled={blocked} onChange={e=>changeMode(e.target.value as 'all'|'any')}><option value="all">全部同意</option><option value="any">任一同意</option></select><h3>可编辑字段</h3>{runtime?<div className="workflow-editable-fields">{runtime.fields.map(field=><label key={field.id}><input type="checkbox" aria-label={field.name} disabled={blocked} checked={selected.approval.editableFieldIds.includes(field.id)} onChange={e=>{const checked=e.target.checked;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===selected.id&&node.kind==='approval'?{...node,approval:{...node.approval,editableFieldIds:checked?[...node.approval.editableFieldIds,field.id]:node.approval.editableFieldIds.filter(id=>id!==field.id)}}:node)}}:previous);}}/>{field.name}<small>{field.id}</small></label>)}{selected.approval.editableFieldIds.filter(id=>!runtime.fields.some(field=>field.id===id)).map(id=><p key={id}>字段已不可用：{id}</p>)}</div>:selected.approval.editableFieldIds.length?<ul>{selected.approval.editableFieldIds.map(id=><li key={id}>{id}</li>)}</ul>:<p>无可编辑字段</p>}</>:selected.kind==='condition'?<><h3>判断条件</h3>{runtime?<WorkflowConditionEditor value={selected.condition as ResourceFilterGroup} fields={conditionFields} disabled={blocked} onChange={condition=>{if(blocked)return;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===selected.id&&node.kind==='condition'?{...node,condition}:node)}}:previous);}}/>:<pre>{JSON.stringify(selected.condition,null,2)}</pre>}</>:<p>{selected.kind==='start'?'流程从此节点开始':'流程在此节点结束'}</p>}</>:<p>选择节点查看当前配置。</p>}</section></div>:<section className="workflow-definition-settings-grid" aria-label="触发与设置"><WorkflowTriggerEditor value={current.triggers} fields={conditionFields} disabled={blocked} onChange={triggers=>{if(blocked)return;setDraft(previous=>previous?{...previous,triggers}:previous);}}/><section className="app-surface workflow-definition-settings"><label className="forms-field-label" htmlFor="workflow-name">流程名称</label><input id="workflow-name" className="admin-input workflow-name-input" value={current.name} disabled={blocked} onChange={e=>{if(blocked)return;const name=e.target.value;setDraft(previous=>previous?{...previous,name}:previous);}}/><h2>撤回设置</h2><label className="workflow-definition-withdraw"><input type="checkbox" checked={current.allowWithdraw} disabled={blocked} onChange={e=>setDraft(previous=>previous?{...previous,allowWithdraw:e.target.checked}:previous)}/>允许发起人撤回</label><p>{current.allowWithdraw?'允许撤回':'不允许撤回'}</p><h2>流程状态</h2><p>当前状态：{createMode?'尚未创建':{enabled:'已启用',disabled:'已关闭',closing:'关闭中'}[current.state]}</p>{current.state==='enabled'?<button type="button" className="admin-button" disabled={dirty||blocked} onClick={()=>setCloseConfirmation(true)}>关闭流程</button>:<>{current.state==='closing'&&<p>正在等待现有审批结束，停止新的发起。</p>}<button type="button" className="admin-button" disabled={createMode||dirty||operationBlocked||current.currentVersion===0||current.currentVersion!==current.candidateVersion} onClick={()=>manage('enable')}>启用流程</button></>}{!createMode&&<><h2>删除流程</h2><p>删除流程配置不会删除记录、日志和审批依据。</p><button type="button" className="admin-button" disabled={dirty||operationBlocked} onClick={()=>setDeletionConfirmation(true)}>删除流程</button></>}</section></section>}
  </>}
  {removal&&<FormsDialog title="移除节点" onClose={()=>setRemoval(null)}><p>移除此节点并连接其原后继，仅改变当前草稿，保存后才生效。</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={blocked||invalidCondition} onClick={removeNode}>确认移除</button></div></FormsDialog>}
  {conditionInsertion&&runtime&&<FormsDialog title="添加条件节点" onClose={()=>setConditionInsertion(null)}><div className="workflow-new-condition"><WorkflowConditionEditor value={conditionInsertion.condition} fields={conditionFields} disabled={blocked} onChange={condition=>setConditionInsertion(previous=>previous?{...previous,condition}:previous)}/><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={blocked||validateResourceFilter(conditionFields,conditionInsertion.condition).issues.length>0} onClick={insertCondition}>添加</button></div></div></FormsDialog>}
  {insertion&&runtime&&<WorkflowApproverPicker actorId={actorId} appId={appId} viewId={viewId} schemaVersion={runtime.schemaVersion} policyRevision={runtime.policyRevision} selectedIds={[]} onClose={()=>setInsertion(null)} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onConfirm={insertApproval}/>}
  {pickerNodeId&&runtime&&selected?.kind==='approval'&&selected.id===pickerNodeId&&<WorkflowApproverPicker key={pickerNodeId} actorId={actorId} appId={appId} viewId={viewId} schemaVersion={runtime.schemaVersion} policyRevision={runtime.policyRevision} selectedIds={selected.approval.assigneeIds} onClose={()=>setPickerNodeId(null)} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onConfirm={ids=>{if(blocked)return;setDraft(previous=>previous?{...previous,graph:{...previous.graph,nodes:previous.graph.nodes.map(node=>node.id===pickerNodeId&&node.kind==='approval'?{...node,approval:{...node.approval,assigneeIds:ids}}:node)}}:previous);setPickerNodeId(null);}}/>}
  {deletionConfirmation&&definition&&<FormsDialog title="删除流程" closeLabel="关闭" onClose={()=>setDeletionConfirmation(false)}><div className="workflow-close-confirmation"><h3>停止新发起，等待已接受工作结束后删除</h3><p>{definition.name}</p><p className="workflow-publication-caption">流程配置删除后不可恢复；记录、日志和审批依据会保留。已受理的删除不能撤销，关闭窗口不会取消删除。</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={dirty||operationBlocked} onClick={deleteFlow}>确认删除</button></div></div></FormsDialog>}
  {closeConfirmation&&definition&&<FormsDialog title="关闭流程" closeLabel="关闭" onClose={()=>setCloseConfirmation(false)}><div className="workflow-close-confirmation"><h3>停止新的发起，等待现有审批结束</h3><p>{definition.name}</p><p className="workflow-publication-caption">已有审批和历史会保留；只有服务确认关闭完成后才显示已关闭。</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={dirty||blocked} onClick={()=>manage('close')}>开始关闭</button></div></div></FormsDialog>}
  {publicationDialog==='confirm'&&definition&&<FormsDialog title="发布流程" closeLabel="关闭" onClose={()=>setPublicationDialog(null)}><div className="workflow-publication-confirm"><h3>{definition.name}</h3><p>候选版本 {definition.candidateVersion}</p><p className="workflow-publication-caption">{definition.currentVersion?'当前已发布版本 '+definition.currentVersion:'尚未发布'}</p><p className="workflow-publication-caption">发布确认后，新发起使用候选版本，在途流程保持原结构。流程启用状态不会自动改变。</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>取消</button><button type="button" className="admin-button primary" disabled={dirty||blocked} onClick={publish}>确认发布</button></div></div></FormsDialog>}
  {publicationDialog==='success'&&publicationResult?.status==='confirmed'&&<FormsDialog title="流程已发布" closeLabel="关闭" onClose={()=>{setPublicationDialog(null);requestAnimationFrame(()=>publishButton.current?.focus());}}><div className="workflow-publication-success"><span className="workflow-publication-success-icon"><img src={publicationSuccessIcon} alt=""/></span><h3>{definition?.name??'流程'} · 版本 {publicationResult.version}</h3><p className="workflow-publication-caption">当前状态：{definition?{enabled:'已启用',disabled:'已关闭',closing:'关闭中'}[definition.state]:error?'暂无法读取':'读取中'}</p><div className="dialog-actions"><button type="button" className="admin-button" data-forms-close>继续编辑</button><button type="button" className="admin-button primary" onClick={onBack}>返回流程</button></div></div></FormsDialog>}
  {publicationDialog==='progress'&&<FormsDialog title="正在发布流程" closeLabel="关闭" onClose={()=>setPublicationDialog(null)}><div className="workflow-publication-progress"><p>正在部署并核对流程版本…</p><div className="workflow-publication-indeterminate" role="progressbar" aria-label="发布结果尚未确认"><span/></div>{publicationFeedback}<p className="workflow-publication-caption">关闭此窗口不会取消已发送的发布，仍可核查原操作。</p></div></FormsDialog>}
 </section>;
}
