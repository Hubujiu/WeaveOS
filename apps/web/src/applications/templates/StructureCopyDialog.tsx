import {useEffect,useRef,useState,type ReactNode} from 'react';
import {applicationReadPost,ApplicationError} from '../api';
import {formApi,FormApiError} from '../forms/api';
import type {Directory} from '../forms/contracts';
import {FormsDialog} from '../forms/FormsDialog';
import {createLeaveController,type RegisterLeaveGuard} from '../forms/leaveGuard';
import {useApplicationOperation} from '../useApplicationOperation';
import type {Application} from '../types';
import {validStructureCopyPlanRequest,validStructureCopyPreflightRequest,type StructureCopyPlanRequest,type StructureCopyPreflightRequest} from './copyRequest';
import {validStructureCopyPlan,validStructureCopyPreflight,type StructureCopyPlan,type StructureCopyPreflight} from './copyPreview';
import type {StructureCopyKind,StructureCopyReceipt} from './copyReceipt';
import {TemplateMapping,referenceRows,type TemplateSelection} from './TemplateMapping';
export type StructureCopySelection={appId:string;kind:StructureCopyKind;id:string;name:string};
type Props={actorId:string;selection:StructureCopySelection;applications:Application[];canCreate:boolean;bootstrapAdmin:boolean;recover?:boolean;onClose:()=>void;onConfirmed:(value:StructureCopyReceipt)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void;onDirtyChange:(dirty:boolean)=>void;registerLeaveGuard:RegisterLeaveGuard;requestSectionLeave:(action:()=>void)=>void};
const names={application:'应用',directory:'目录',table:'数据表',form:'表单结构'};
const labels:Record<string,string>={format:'格式',version:'格式版本',application:'应用',directories:'目录',tables:'数据表',forms:'表单视图',workflows:'流程（副本保持停用）',permissionGroups:'独立新权限组',id:'标识',name:'名称',parentId:'父目录',directoryId:'目录',position:'位置',fields:'字段',kind:'类型',required:'必填',default:'默认值',config:'字段配置',presentation:'展示配置',helpText:'帮助文字',displayTimeZone:'显示时区',maxLength:'最大长度',precision:'精度',scale:'小数位数',roundingMode:'舍入规则',roundingPlaces:'处理位数',options:'选项',label:'标签',tableId:'数据表标识',viewId:'表单标识',layout:'布局',fieldId:'字段标识',span:'宽度',text:'文字',title:'标题',children:'子项',graph:'流程图',nodes:'节点',edges:'连线',approval:'审批配置',mode:'通过规则',assigneeIds:'审批人',editableFieldIds:'可编辑字段',condition:'条件',operator:'条件运算',value:'条件值',from:'起点',to:'终点',branch:'分支',allowWithdraw:'允许撤回',triggers:'触发条件',event:'触发事件',enabled:'启用',memberIds:'成员',grants:'授权',resourceKind:'资源类型',resourceId:'资源标识',action:'动作',rowScope:'记录范围'};
// A read-only rendering of every server-supplied configuration member. It does
// not rebuild copy closure, normalize values, allocate identities or infer grants.
function ConfigurationValue({value}:{value:unknown}):ReactNode{
 if(value===null)return <span>空</span>;
 if(Array.isArray(value))return value.length?<ol>{value.map((item,index)=><li key={index}><ConfigurationValue value={item}/></li>)}</ol>:<span>无</span>;
 if(typeof value==='object'&&Object.keys(value as Record<string,unknown>).length===0)return <span>无</span>;
 if(typeof value==='object')return <dl>{Object.entries(value as Record<string,unknown>).map(([key,item])=><div key={key}><dt>{labels[key]??key}</dt><dd><ConfigurationValue value={item}/></dd></div>)}</dl>;
 return <span>{typeof value==='boolean'?(value?'是':'否'):String(value)===''?'空字符串':String(value)}</span>;
}
export function StructureCopyDialog({actorId,selection,applications,canCreate,bootstrapAdmin,recover=false,onClose,onConfirmed,onUnauthorized,onIdentityMismatch,onDirtyChange,registerLeaveGuard,requestSectionLeave}:Props){
 const initialName=selection.name+'副本';
 const [name,setName]=useState(initialName),[targetApp,setTargetApp]=useState(''),[targetDir,setTargetDir]=useState('');
 const [directories,setDirectories]=useState<Directory[]>([]),[targetReady,setTargetReady]=useState(selection.kind==='application');
 const [plan,setPlan]=useState<StructureCopyPlan|null>(null),[checked,setChecked]=useState<StructureCopyPreflight|null>(null),[selectionMap,setSelectionMap]=useState<TemplateSelection>({}),[groups,setGroups]=useState<Record<string,string>>({});
 const [error,setError]=useState(''),[reading,setReading]=useState(false),[denied,setDenied]=useState(false),[result,setResult]=useState<StructureCopyReceipt|null>(null);
 const request=useRef<AbortController|null>(null),generation=useRef(0),checkedBody=useRef<StructureCopyPreflightRequest|null>(null),finished=useRef(false),bypass=useRef(false);
 const source={appId:selection.appId,kind:selection.kind,id:selection.id};
 const resource={kind:'structure-copy' as const,sourceAppId:selection.appId,sourceKind:selection.kind,sourceId:selection.id};
 const operation=useApplicationOperation<StructureCopyReceipt>({actorId,scope:JSON.stringify(['structure-copy',actorId,selection.appId,selection.kind,selection.id]),resource,confirmed:value=>{if(value){finished.current=true;setResult(value);setPlan(null);setChecked(null);setSelectionMap({});setGroups({});checkedBody.current=null;setError('');onDirtyChange(false);onConfirmed(value);}},unauthorized:onUnauthorized,identityMismatch:onIdentityMismatch});
 const busy=operation.phase==='preflight'||operation.phase==='pending',unknown=operation.phase==='unconfirmed'||operation.phase==='accepted';
 const blocked=busy||unknown||!!result;
 const target={appId:selection.kind==='application'?null:targetApp||null,directoryId:selection.kind==='application'?null:targetDir||null,name};
 const input:StructureCopyPlanRequest={source,target};
 const bindings=plan?referenceRows(plan.externalReferences).filter(row=>selectionMap[row.id]).map(row=>({kind:row.kind,sourceId:row.sourceId,targetId:selectionMap[row.id].id})):[];
 const planGroups=(plan?.manifest.permissionGroups??[]) as {id:string;name:string}[];
 const body:StructureCopyPreflightRequest|null=plan?{...input,expectedSourceFingerprint:plan.sourceFingerprint,expectedTargetStructureVersion:plan.targetStructureVersion,expectedTargetPolicyRevision:plan.targetPolicyRevision,bindings,groupMappings:planGroups.map(g=>({sourceGroupId:g.id,name:groups[g.id]??''}))}:null;
 const mappingComplete=!!plan&&bindings.length===plan.externalReferences.userIds.length+plan.externalReferences.departmentIds.length&&!!body&&validStructureCopyPreflightRequest(body);
 const draft=!!plan||name!==initialName||!!targetApp||!!targetDir;
 const current=useRef({operation,draft,name,targetApp,targetDir,selectionMap,groups});current.current={operation,draft,name,targetApp,targetDir,selectionMap,groups};
 const abort=()=>{generation.current++;request.current?.abort();request.current=null;checkedBody.current=null;setReading(false);};
 const clearPreview=()=>{abort();setChecked(null);setError('');operation.dismissError();};
 const resetPlan=()=>{clearPreview();setPlan(null);setSelectionMap({});setGroups({});};
 const fail=(cause:unknown)=>{
  if((cause instanceof ApplicationError||cause instanceof FormApiError)&&cause.status===401)onUnauthorized();
  else if((cause instanceof ApplicationError||cause instanceof FormApiError)&&cause.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();
  else {setError(cause instanceof Error?cause.message:'复制读取失败。');if((cause instanceof ApplicationError||cause instanceof FormApiError)&&[403,404].includes(cause.status)){setDenied(true);setPlan(null);setChecked(null);setSelectionMap({});setGroups({});setDirectories([]);checkedBody.current=null;}}
 };
 useEffect(()=>()=>{generation.current++;request.current?.abort();},[]);
 useEffect(()=>{if(operation.errorCode==='APPLICATION_FORBIDDEN')fail(new ApplicationError(403,operation.errorCode));},[operation.errorCode]);
 useEffect(()=>registerLeaveGuard({kind:'structure-copy',actorId,appId:selection.appId,resourceKind:selection.kind,resourceId:selection.id},createLeaveController(()=>{const s=current.current,status=finished.current||bypass.current?'clean':s.operation.getPendingStatus()??(s.draft?'draft':'clean');return {status,fingerprint:JSON.stringify([status,s.name,s.targetApp,s.targetDir,s.selectionMap,s.groups,s.operation.packet?.operationId])};},decision=>{abort();if(decision==='discard')current.current.operation.cancelPreflight();bypass.current=true;onDirtyChange(false);})),[actorId,selection.appId,selection.kind,selection.id,registerLeaveGuard,onDirtyChange]);
 useEffect(()=>{onDirtyChange(!result&&(draft||busy||unknown));return()=>onDirtyChange(false);},[draft,busy,unknown,result,onDirtyChange]);
 useEffect(()=>{
  if(selection.kind==='application'||recover||!targetApp)return;
  let live=true;const controller=new AbortController();setTargetReady(false);setDirectories([]);
  void formApi.structure(targetApp,actorId,controller.signal).then(value=>{if(!live)return;if(!value.capabilities.canManageDefinition)throw new ApplicationError(403,'APPLICATION_FORBIDDEN');setDirectories(value.directories);setTargetReady(true);}).catch(cause=>{if(live&&!controller.signal.aborted)fail(cause);});
  return()=>{live=false;controller.abort();};
 },[actorId,targetApp,selection.kind,recover]);
 const runRead=async(stage:'plan'|'preflight')=>{
  if(blocked||denied||request.current||recover||!targetReady||selection.kind==='application'&&!canCreate)return;
  const payload=stage==='plan'?input:body;if(!payload||stage==='plan'&&!validStructureCopyPlanRequest(payload)||stage==='preflight'&&!mappingComplete)return;
  abort();const controller=new AbortController(),token=generation.current;request.current=controller;setReading(true);setError('');setChecked(null);
  try{const value=await applicationReadPost<unknown>(actorId,'structure-copies/'+stage,payload,controller.signal);if(controller.signal.aborted||generation.current!==token||request.current!==controller)return;
   if(stage==='plan'){if(!validStructureCopyPlan(value,input))throw Error('复制计划不完整或作用域不匹配。');setPlan(value);setSelectionMap({});setGroups({});}
   else {if(!validStructureCopyPreflight(value,input))throw Error('复制预检响应不完整，不能确认。');checkedBody.current=structuredClone(payload as StructureCopyPreflightRequest);setChecked(value);}
  }catch(cause){if(!controller.signal.aborted&&generation.current===token&&request.current===controller)fail(cause);}
  finally{if(request.current===controller){request.current=null;setReading(false);}}
 };
 const submit=()=>{if(blocked||denied||!checked||!checkedBody.current||!body||JSON.stringify(body)!==JSON.stringify(checkedBody.current))return;operation.start({resource,path:'structure-copies/execute',method:'POST',expectedStatus:201,body:checkedBody.current});};
 const close=()=>{abort();onClose();};
 const phase=result?'result':unknown||recover?'recovery':checked?'preview-stage':plan?'mapping':selection.kind==='application'?'application':'target';
 return <FormsDialog title={checked?'复制结构：核对完整配置':plan?'复制结构：明确映射':'复制'+names[selection.kind]} onClose={close} busy={busy} requestClose={requestSectionLeave}>
  <div className={'structure-copy-panel structure-copy-'+phase}>
   {(error||operation.message)&&<p role="alert">{error||operation.message}</p>}
   {result?<><p role="status">结构已复制</p><p>新对象已确认创建；新表为空，流程保持停用。</p><p>新对象编号：{result.id}</p></>:unknown||recover?<><p>已保留原复制操作，不再读取来源配置。</p><div className="workflow-instance-buttons"><button className="admin-button" type="button" disabled={busy} onClick={operation.query}>核查原操作</button><button className="admin-button" type="button" disabled={busy} onClick={operation.retry}>重试原操作</button></div></>:!denied&&<>
    {!plan&&<><p>来源：{selection.name}</p><p className="workflow-publication-caption">只复制结构和明确依赖；新表为空，流程保持停用。</p>
     <label>{selection.kind==='application'?'新应用名称':'新根名称'}<input aria-label={selection.kind==='application'?'新应用名称':'新根名称'} value={name} disabled={blocked} onChange={e=>{resetPlan();setName(e.target.value);}}/></label>
     {selection.kind==='application'?<label>目标位置（固定）<input readOnly value="新建独立应用"/></label>:<><label>目标应用<select aria-label="目标应用" value={targetApp} disabled={blocked} onChange={e=>{resetPlan();setTargetApp(e.target.value);setTargetDir('');setDirectories([]);setTargetReady(false);}}><option value="">请选择有管理权限的应用</option>{applications.filter(app=>bootstrapAdmin||app.ownerUserId===actorId).map(app=><option key={app.id} value={app.id}>{app.name}</option>)}</select></label><label>目标目录<select aria-label="目标目录" value={targetDir} disabled={blocked||!targetReady} onChange={e=>{resetPlan();setTargetDir(e.target.value);}}><option value="">根目录</option>{directories.map(dir=><option key={dir.id} value={dir.id}>{dir.name}</option>)}</select></label></>}
    </>}
    {plan&&!checked&&<><TemplateMapping actorId={actorId} sourceAppId={selection.appId} groups={planGroups.map(group=>({...group,value:groups[group.id]??''}))} onGroupName={(id,name)=>{clearPreview();setGroups(old=>({...old,[id]:name}));}} references={plan.externalReferences} selection={selectionMap} disabled={blocked} onSelect={(row,candidate)=>{clearPreview();setSelectionMap(old=>({...old,[row.id]:candidate}));}} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onForbidden={()=>fail(new ApplicationError(403,'APPLICATION_FORBIDDEN'))}/>
     <p className="workflow-publication-caption">每项显式确认；同一人员也须映射。创建独立新组，不修改目标已有组。</p></>}
    {checked&&<section className="structure-copy-preview" aria-label="完整配置预览"><p>预检通过；确认复制前不会创建任何对象。</p><p>目标：{name}</p><h3>所选表单与必要依赖</h3><ConfigurationValue value={{application:checked.mappedManifest.application,directories:checked.mappedManifest.directories,tables:checked.mappedManifest.tables,forms:checked.mappedManifest.forms}}/><h3>流程配置（副本停用）</h3><ConfigurationValue value={checked.mappedManifest.workflows}/><h3>独立新权限组与授权</h3><ConfigurationValue value={checked.mappedManifest.permissionGroups}/></section>}
   </>}
   {reading&&<p role="status">正在读取复制{plan?'预检':'计划'}…</p>}
   <div className="forms-dialog-actions"><button type="button" data-forms-close disabled={busy}>{result?'关闭':unknown?'关闭并保留待核查操作':'取消'}</button>{!result&&!unknown&&!recover&&!denied&&(checked?<button className="forms-primary" type="button" disabled={busy} onClick={submit}>确认复制</button>:plan?<button className="forms-primary" type="button" disabled={blocked||reading||!mappingComplete} onClick={()=>void runRead('preflight')}>预检复制</button>:<button className="forms-primary" type="button" disabled={blocked||reading||!targetReady||!validStructureCopyPlanRequest(input)||selection.kind==='application'&&!canCreate} onClick={()=>void runRead('plan')}>读取复制计划</button>)}</div>
  </div>
 </FormsDialog>;
}
