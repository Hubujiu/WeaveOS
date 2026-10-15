import {StructureCopyDialog,type StructureCopySelection} from '../templates/StructureCopyDialog';
import {StructureDeletionDialog,type StructureDeletionSelection} from '../forms/StructureDeletionDialog';
import {TemplateImport} from '../templates/TemplateImport';
import {scopedUnconfirmed} from '../recovery';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useLocation, useNavigate } from 'react-router';
import type { Access, User } from '../../workspace-types';
import { WorkspaceShell } from '../../WorkspaceShell';
import type { Application, ApplicationTab } from '../types';
import type { RegisterLeaveGuard } from './leaveGuards';
import { ApplicationWorkspace } from '../ApplicationWorkspace';
import { ApplicationCards, ApplicationCatalog } from '../catalog/ApplicationCatalog';
import { CreateApplication } from '../catalog/CreateApplication';
import { useApplications } from '../useApplications';
import { useApplicationOperation } from '../useApplicationOperation';
import { validCreatedApplication } from '../creationReceipt';
import '../applications.css';
import {WorkflowRecordHistory} from '../workflows/WorkflowRecordHistory';
import {WorkflowStarted} from '../workflows/WorkflowStarted';
import {WorkflowApprovedDetail} from '../workflows/WorkflowApprovedDetail';
import {WorkflowApprovals} from '../workflows/WorkflowApprovals';
import {WorkflowInbox} from '../workflows/WorkflowInbox';
import {WorkflowTaskPage} from '../workflows/WorkflowTaskPage';

const uuid = /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i;

export function AppShell({ user, access, logout, pending, error, onDirty, onPermissionDirty, onFormsDirty, registerLeaveGuard, requestSectionLeave, onAuthLost, onIdentityMismatch, suspended, adminContent }: { user: User; access: Access; logout: () => void; pending: boolean; error: string; onDirty: (dirty: boolean) => void; onPermissionDirty: (dirty: boolean, summary: string) => void; onFormsDirty: (dirty: boolean) => void; registerLeaveGuard: RegisterLeaveGuard; requestSectionLeave: (action: () => void) => void; onAuthLost: () => void; onIdentityMismatch: () => void; suspended: boolean; adminContent?: ReactNode }) {
 const location = useLocation(); const navigate = useNavigate();
 const taskParts=location.pathname.match(/^\/app\/approvals\/([^/]+)\/([^/]+)\/([^/]+)\/([^/]+)\/([^/]+)\/?$/);
 const taskLocation=taskParts&&taskParts.slice(1).every(id=>uuid.test(id))?{appId:taskParts[1],viewId:taskParts[2],recordId:taskParts[3],instanceId:taskParts[4],taskId:taskParts[5]}:null;
 const approvedHistory=/^\/app\/approvals\/completed\/?$/.test(location.pathname);
 const startedHistory=/^\/app\/approvals\/started\/?$/.test(location.pathname);
 const eventParts=location.pathname.match(/^\/app\/approvals\/completed\/([^/]+)\/([^/]+)\/([^/]+)\/([^/]+)\/?$/);
 const eventLocation=eventParts&&eventParts.slice(1).every(id=>uuid.test(id)&&id===id.toLowerCase()&&id!=='00000000-0000-0000-0000-000000000000')?{appId:eventParts[1],viewId:eventParts[2],recordId:eventParts[3],eventId:eventParts[4]}:null;
 const historyParts=location.pathname.match(/^\/app\/approvals\/started\/([^/]+)\/([^/]+)\/([^/]+)\/?$/);
 const historyLocation=historyParts&&historyParts.slice(1).every(id=>uuid.test(id)&&id===id.toLowerCase()&&id!=='00000000-0000-0000-0000-000000000000')?{appId:historyParts[1],viewId:historyParts[2],recordId:historyParts[3]}:null;
 const approvals = !!historyLocation||!!eventLocation||startedHistory||approvedHistory||!!taskLocation||/^\/app\/approvals\/?$/.test(location.pathname);
 const templateImport=location.pathname==='/app/application-templates/import';
 const catalog = location.pathname === '/app/applications' || location.pathname === '/app/applications/';
 const parsedAppRoute = location.pathname.match(/^\/app\/applications\/([^/]+)(?:\/forms\/([^/]+)(?:\/(design|workflows)(?:\/([^/]+)\/(design|new))?)?)?\/?$/);
 const appRoute=parsedAppRoute?.[4]&&(parsedAppRoute[3]!=='workflows'||!uuid.test(parsedAppRoute[4]))?null:parsedAppRoute;
 const appId = appRoute?.[1]&&uuid.test(appRoute[1])?appRoute[1]:undefined;
 const viewId = appId&&appRoute?.[2]&&uuid.test(appRoute[2])?appRoute[2]:undefined;
 const configurationRoute = !!viewId&&appRoute?.[3] === 'design';
 const workflowManagementRoute = !!viewId&&appRoute?.[3] === 'workflows';
 const workflowFlowId=workflowManagementRoute&&appRoute?.[4]&&uuid.test(appRoute[4])?appRoute[4]:undefined;
 const [tabs, setTabs] = useState<ApplicationTab[]>([]);
 const [contentDirty,setContentDirty]=useState(false),[recoveryDirty,setRecoveryDirty]=useState(false);
 const [copyDirty,setCopyDirty]=useState(false);
 const [copy,setCopy]=useState<{actorId:string;selection:StructureCopySelection;operationId?:string}|null>(null);
 const copyOrigin=useRef<HTMLButtonElement|null>(null);
 const currentCopy=copy?.actorId===user.id&&!suspended?copy:null;
 const copyRecoveries=suspended?[]:scopedUnconfirmed(user.id).flatMap(packet=>{const r=packet.resource;if(r?.kind!=='structure-copy')return [];return [{actorId:user.id,operationId:packet.operationId,selection:{appId:r.sourceAppId,kind:r.sourceKind,id:r.sourceId,name:r.sourceId}}];});
 const closeCopy=()=>{const origin=copyOrigin.current;setCopy(null);setCopyDirty(false);requestAnimationFrame(()=>{if(origin?.isConnected&&!origin.disabled)origin.focus();});};
 const beginCopy=(selection:StructureCopySelection,origin:HTMLButtonElement)=>{requestSectionLeave(()=>{copyOrigin.current=origin;setCopy({actorId:user.id,selection});});};
 const [structureRefresh,setStructureRefresh]=useState<{appId:string;version:number}|null>(null);
 const [deletionRecovery,setDeletionRecovery]=useState<{actorId:string;appId:string;operationId:string;selection:StructureDeletionSelection}|null>(null);
 const recoveryOrigin=useRef<HTMLButtonElement|null>(null);
 const structuralRecoveries=suspended?[]:scopedUnconfirmed(user.id).flatMap(packet=>{
  const resource=packet.resource,body=packet.body;
  if(resource?.kind!=='structure-deletion'||typeof packet.operationId!=='string'||!Number.isSafeInteger(body?.expectedStructureVersion)||!Number.isSafeInteger(body?.expectedResourceVersion))return [];
  return [{actorId:user.id,appId:resource.appId,operationId:packet.operationId,selection:{kind:resource.resourceKind,id:resource.id,structureVersion:body!.expectedStructureVersion as number,resourceVersion:body!.expectedResourceVersion as number}}];
 });
 const currentDeletionRecovery=deletionRecovery?.actorId===user.id&&!suspended?deletionRecovery:null;
 useEffect(()=>{onFormsDirty(!suspended&&(contentDirty||recoveryDirty||copyDirty));return()=>onFormsDirty(false);},[contentDirty,recoveryDirty,copyDirty,suspended,onFormsDirty]);
 useEffect(()=>{
  const warn=(event:BeforeUnloadEvent)=>{if(!suspended&&scopedUnconfirmed(user.id).some(packet=>packet.resource?.kind==='structure-deletion'||packet.resource?.kind==='structure-copy')){event.preventDefault();event.returnValue='';}};
  window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);
 },[user.id,suspended]);
 const closeDeletionRecovery=()=>{const origin=recoveryOrigin.current;setDeletionRecovery(null);requestAnimationFrame(()=>{if(origin?.isConnected&&!origin.disabled)origin.focus();});};

 const handledClose = useRef(new Set<string>());
 const [account, setAccount] = useState(false);
 const [create, setCreate] = useState(false); const [success, setSuccess] = useState('');
 useEffect(() => { if (suspended) { setTabs([]); setAccount(false); setCreate(false); setSuccess('');setDeletionRecovery(null);setCopy(null);setCopyDirty(false);setContentDirty(false);setRecoveryDirty(false); onDirty(false); } }, [suspended, onDirty]);
 const importTrigger=useRef<HTMLButtonElement|null>(null),restoreImportFocus=useRef(false);
 useEffect(()=>{if(catalog&&restoreImportFocus.current){restoreImportFocus.current=false;queueMicrotask(()=>importTrigger.current?.focus());}},[catalog,location.pathname]);
 const closeImport=()=>{restoreImportFocus.current=true;navigate('/app/applications');};
 const createTrigger = useRef<HTMLButtonElement | null>(null);
 const closeCreate = useCallback(() => { setCreate(false); queueMicrotask(() => { if (createTrigger.current?.isConnected) createTrigger.current.focus(); }); }, []);
 const previousPath = useRef(location.pathname);
 useEffect(() => { if (previousPath.current !== location.pathname && create) closeCreate(); previousPath.current = location.pathname; }, [location.pathname, create, closeCreate]);
 useEffect(() => {
  const close = (location.state as { closeApplication?: { id: string; event: string } } | null)?.closeApplication;
  if (!close || handledClose.current.has(close.event)) return;
  handledClose.current.add(close.event);
  setTabs(items => items.filter(value => value.application.id !== close.id));
 }, [location.state]);
 const onUnauthorized = useCallback(() => { onDirty(false); onPermissionDirty(false, ''); onAuthLost(); }, [onDirty, onPermissionDirty, onAuthLost]);
 const apps = useApplications(user.id, !suspended, onUnauthorized, onIdentityMismatch);
 const confirmed = useCallback((app: Application) => { onDirty(false); closeCreate(); setSuccess('应用已创建：' + app.name); apps.reload(); }, [onDirty, closeCreate, apps.reload]);
 const operation = useApplicationOperation(user.id, confirmed, onUnauthorized, onIdentityMismatch, app => validCreatedApplication(app, user.id));
 const opened = useCallback((app: Application) => setTabs(items => items.some(v => v.application.id === app.id) ? items.map(v => v.application.id === app.id ? { ...v, application: app } : v) : [...items, { application: app, pinned: false }]), []);
 const open = (app: Application) => navigate('/app/applications/' + encodeURIComponent(app.id));
 const configurationPath=(id:string,formId:string)=>'/app/applications/'+encodeURIComponent(id)+'/forms/'+encodeURIComponent(formId)+'/design';
 const runtimeFormPath=(id:string,formId:string)=>'/app/applications/'+encodeURIComponent(id)+'/forms/'+encodeURIComponent(formId);
 const openConfiguration=(formId:string,returnPath:string)=>{
  if(!appId)return;
  navigate(configurationPath(appId,formId),{state:{formConfigurationReturn:{appId,path:returnPath}}});
 };
 const configurationReturn=()=>{
  if(!appId||!viewId)return '/app/applications';
  const state=location.state as {formConfigurationReturn?:{appId?:unknown;path?:unknown}}|null;
  const candidate=state?.formConfigurationReturn;
  const directory='/app/applications/'+appId;
  const runtime=runtimeFormPath(appId,viewId);
  return candidate?.appId===appId&&([directory,directory+'/',runtime,runtime+'/'].includes(candidate.path as string))?candidate.path as string:runtime;
 };
 const closeTab = (id: string) => {
  if (appId === id) navigate('/app/applications', { state: { closeApplication: { id, event: crypto.randomUUID() } } });
  else setTabs(items => items.filter(value => value.application.id !== id));
 };
 const canCreate = access.bootstrapAdmin || access.permissions.some(p => p.code === 'applications.create');
 if (suspended) return null;
 return <WorkspaceShell user={user} access={access} catalog={catalog||templateImport} appId={appId} admin={!!adminContent} approvals={approvals} onApprovals={()=>navigate('/app/approvals')}
  accountOpen={account} onAccount={()=>setAccount(value=>!value)} onHome={()=>navigate('/app')}
  onCatalog={()=>navigate('/app/applications')} onAdmin={()=>navigate('/app/admin')}
  tabs={
   <nav className="app-tabs" aria-label="全局应用标签">
    <button className="app-global-tab app-home-tab" aria-current={!adminContent && !catalog && !appId && !approvals && !templateImport ? 'page' : undefined} onClick={() => navigate('/app')}>首页</button>
    {tabs.map(tab => <div className="app-tab-group" key={tab.application.id}><button className="app-global-tab" aria-current={appId === tab.application.id ? 'page' : undefined} onClick={() => open(tab.application)}>{tab.application.name}</button>{!tab.pinned && <button className="app-close-tab" aria-label={'关闭应用：' + tab.application.name} onClick={() => closeTab(tab.application.id)}>×</button>}</div>)}
    <button className="app-global-tab" aria-current={catalog ? 'page' : undefined} onClick={() => navigate('/app/applications')}>全部应用</button>
   </nav>
  }>
  <main className="app-content" aria-label={templateImport?'模板导入':adminContent ? '人员管理' : approvals ? '我的待办' : catalog ? '应用中心' : appId ? '应用工作台' : '主页'}>
   {!currentCopy&&copyRecoveries.map(item=><div className="workflow-manager-actions" key={item.operationId}><span>待核查复制：{item.selection.id}</span><button type="button" className="admin-button" disabled={contentDirty||recoveryDirty} onClick={event=>{if(contentDirty||recoveryDirty)return;copyOrigin.current=event.currentTarget;setCopy(item);}}>核查结构复制</button></div>)}
   {currentCopy&&<StructureCopyDialog key={JSON.stringify([user.id,currentCopy.selection.appId,currentCopy.selection.kind,currentCopy.selection.id,currentCopy.operationId])} actorId={user.id} selection={currentCopy.selection} recover={!!currentCopy.operationId} applications={apps.items} canCreate={canCreate} bootstrapAdmin={access.bootstrapAdmin} onClose={closeCopy} onConfirmed={value=>{setCopyDirty(false);apps.reload();setStructureRefresh(previous=>({appId:value.appId,version:(previous?.version??0)+1}));}} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onDirtyChange={setCopyDirty} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave}/>}
   {!currentDeletionRecovery&&structuralRecoveries.map(item=><div className="workflow-manager-actions" key={item.operationId}><span>待核查删除：{item.selection.id}</span><button type="button" className="admin-button" disabled={contentDirty||copyDirty} onClick={event=>{if(contentDirty||copyDirty)return;recoveryOrigin.current=event.currentTarget;setDeletionRecovery(item);}}>核查结构删除</button></div>)}
   {structuralRecoveries.length>0&&contentDirty&&!currentDeletionRecovery&&<p className="workflow-history-meta">请先保存或明确放弃当前修改，再核查原操作。</p>}
   {currentDeletionRecovery&&<StructureDeletionDialog key={currentDeletionRecovery.operationId} actorId={user.id} appId={currentDeletionRecovery.appId} selection={currentDeletionRecovery.selection} onClose={closeDeletionRecovery} onConfirmed={result=>{setDeletionRecovery(null);setRecoveryDirty(false);apps.reload();if(result.resourceKind==='application'){if(appId===result.appId)closeTab(result.appId);else setTabs(items=>items.filter(item=>item.application.id!==result.appId));}else setStructureRefresh(previous=>({appId:result.appId,version:(previous?.version??0)+1}));}} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onForbidden={closeDeletionRecovery} onDirtyChange={setRecoveryDirty} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave}/>}
   {error && <p role="alert">{error}</p>}{catalog && success && <p className="app-success" role="status">{success}</p>}
   {adminContent ?? (templateImport ? <TemplateImport key={user.id} actorId={user.id} canCreate={canCreate} onClose={closeImport} onOpenApplication={id=>{apps.reload();navigate('/app/applications/'+encodeURIComponent(id));}} onDirty={setContentDirty} registerLeaveGuard={registerLeaveGuard} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/> : appId ? <ApplicationWorkspace canCopyApplication={canCreate} onCopy={beginCopy} structureRefresh={structureRefresh?.appId===appId?structureRefresh.version:0} onApplicationDeleted={()=>{apps.reload();closeTab(appId);}} actorId={user.id} appId={appId} viewId={viewId} configurationRoute={configurationRoute} workflowManagementRoute={workflowManagementRoute} workflowFlowId={workflowFlowId} workflowCreateMode={appRoute?.[5]==='new'} openWorkflow={(id,flowId,create)=>navigate(runtimeFormPath(appId,id)+'/workflows/'+encodeURIComponent(flowId)+(create?'/new':'/design'))} manageWorkflows={id=>navigate(runtimeFormPath(appId,id)+'/workflows')} bootstrapAdmin={access.bootstrapAdmin} opened={opened} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onPermissionDirty={onPermissionDirty} onFormsDirty={setContentDirty} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave} openForm={id=>navigate(runtimeFormPath(appId,id))} configureForm={id=>openConfiguration(id,location.pathname)} backFromConfiguration={()=>navigate(configurationReturn())} backToStructure={()=>navigate('/app/applications/'+encodeURIComponent(appId))} />
    : taskLocation ? <WorkflowTaskPage key={JSON.stringify([user.id,taskLocation])} actorId={user.id} location={taskLocation} onDirtyChange={setContentDirty} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave} onBack={()=>navigate('/app/approvals')} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
    : eventLocation ? <WorkflowApprovedDetail key={JSON.stringify([user.id,eventLocation])} actorId={user.id} location={eventLocation} onBack={()=>navigate('/app/approvals/completed')} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
    : historyLocation ? <WorkflowRecordHistory key={JSON.stringify([user.id,historyLocation])} actorId={user.id} location={historyLocation} onBack={()=>navigate('/app/approvals/started')} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
    : startedHistory ? <WorkflowStarted key={user.id} actorId={user.id} onOpenHistory={item=>navigate('/app/approvals/started/'+[item.appId,item.viewId,item.recordId].join('/'))} onOpenInbox={()=>navigate('/app/approvals')} onOpenApproved={()=>navigate('/app/approvals/completed')} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
    : approvedHistory ? <WorkflowApprovals key={user.id} actorId={user.id} onOpenStarted={()=>navigate('/app/approvals/started')} onOpenEvent={item=>navigate('/app/approvals/completed/'+[item.appId,item.viewId,item.recordId,item.event.id].join('/'))} onOpenInbox={()=>navigate('/app/approvals')} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
    : approvals ? <WorkflowInbox key={user.id} actorId={user.id} onOpenStarted={()=>navigate('/app/approvals/started')} onOpenApproved={()=>navigate('/app/approvals/completed')} onOpenTask={task=>navigate('/app/approvals/'+[task.appId,task.viewId,task.recordId,task.instanceId,task.id].map(encodeURIComponent).join('/'))} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch}/>
    : catalog ? <ApplicationCatalog canImport={canCreate||scopedUnconfirmed(user.id).some(packet=>packet.resource?.kind==='template-import')} importTrigger={importTrigger} importTemplate={()=>navigate('/app/application-templates/import')} applications={apps.items} loading={apps.loading} error={apps.error} retry={apps.reload} canCreate={canCreate} create={trigger => { createTrigger.current = trigger; setSuccess(''); setCreate(true); }} open={open} />
    : <><div className="app-page-heading"><h1>主页</h1></div><section className="app-surface app-home-section"><h2>我的应用</h2>{apps.loading ? <p role="status">正在加载应用…</p> : apps.error ? <div className="app-state"><p role="alert">{apps.error}</p><button className="admin-button" onClick={apps.reload}>重试</button></div> : apps.items.length ? <ApplicationCards applications={apps.items} open={open} /> : <div className="home-empty"><h3>暂无可用应用</h3><p>获得应用访问权限后，将在这里显示。</p></div>}</section><div><button className="admin-button" onClick={() => navigate('/app/applications')}>打开应用中心</button></div></>)}
  </main>
  {account && <section className="account-menu" aria-label="账号信息"><strong>{user.account}</strong><p>{access.bootstrapAdmin ? 'Bootstrap Admin' : access.identities.length ? access.identities.map(v => v.name).join('、') : '尚未分配身份'}</p>{access.permissions.map(p => <div key={p.code}><span>{p.name}</span>{p.sources?.map((s, i) => <small key={i}>{s.identityName}{s.templateName ? ' · ' + s.templateName : ' · 直接权限'}</small>)}</div>)}<button className="admin-button" disabled={pending} onClick={logout}>退出登录</button></section>}
  {create && <CreateApplication actorId={user.id} close={closeCreate} onDirty={onDirty} operation={operation} />}
 </WorkspaceShell>;
}
