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
import '../applications.css';

const uuid = /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i;
const validCreatedApplication = (app: Application, actorId: string) => uuid.test(app?.id) && typeof app.name === 'string' && app.name.trim().length > 0 && app.ownerUserId === actorId && app.policyRevision === 1;

export function AppShell({ user, access, logout, pending, error, onDirty, onPermissionDirty, onFormsDirty, registerLeaveGuard, requestSectionLeave, onAuthLost, onIdentityMismatch, suspended, adminContent }: { user: User; access: Access; logout: () => void; pending: boolean; error: string; onDirty: (dirty: boolean) => void; onPermissionDirty: (dirty: boolean, summary: string) => void; onFormsDirty: (dirty: boolean) => void; registerLeaveGuard: RegisterLeaveGuard; requestSectionLeave: (action: () => void) => void; onAuthLost: () => void; onIdentityMismatch: () => void; suspended: boolean; adminContent?: ReactNode }) {
 const location = useLocation(); const navigate = useNavigate();
 const catalog = location.pathname === '/app/applications' || location.pathname === '/app/applications/';
 const appRoute = location.pathname.match(/^\/app\/applications\/([^/]+)(?:\/forms\/([^/]+)(?:\/(design))?)?\/?$/);
 const appId = appRoute?.[1]&&uuid.test(appRoute[1])?appRoute[1]:undefined;
 const viewId = appId&&appRoute?.[2]&&uuid.test(appRoute[2])?appRoute[2]:undefined;
 const configurationRoute = !!viewId&&appRoute?.[3] === 'design';
 const [tabs, setTabs] = useState<ApplicationTab[]>([]);
 const handledClose = useRef(new Set<string>());
 const [account, setAccount] = useState(false);
 const [create, setCreate] = useState(false); const [success, setSuccess] = useState('');
 useEffect(() => { if (suspended) { setTabs([]); setAccount(false); setCreate(false); setSuccess(''); onDirty(false); } }, [suspended, onDirty]);
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
 return <WorkspaceShell user={user} access={access} catalog={catalog} appId={appId} admin={!!adminContent}
  accountOpen={account} onAccount={()=>setAccount(value=>!value)} onHome={()=>navigate('/app')}
  onCatalog={()=>navigate('/app/applications')} onAdmin={()=>navigate('/app/admin')}
  tabs={
   <nav className="app-tabs" aria-label="全局应用标签">
    <button className="app-global-tab app-home-tab" aria-current={!adminContent && !catalog && !appId ? 'page' : undefined} onClick={() => navigate('/app')}>首页</button>
    {tabs.map(tab => <div className="app-tab-group" key={tab.application.id}><button className="app-global-tab" aria-current={appId === tab.application.id ? 'page' : undefined} onClick={() => open(tab.application)}>{tab.application.name}</button>{!tab.pinned && <button className="app-close-tab" aria-label={'关闭应用：' + tab.application.name} onClick={() => closeTab(tab.application.id)}>×</button>}</div>)}
    <button className="app-global-tab" aria-current={catalog ? 'page' : undefined} onClick={() => navigate('/app/applications')}>全部应用</button>
   </nav>
  }>
  <main className="app-content" aria-label={adminContent ? '人员管理' : catalog ? '应用中心' : appId ? '应用工作台' : '主页'}>
   {error && <p role="alert">{error}</p>}{catalog && success && <p className="app-success" role="status">{success}</p>}
   {adminContent ?? (appId ? <ApplicationWorkspace actorId={user.id} appId={appId} viewId={viewId} configurationRoute={configurationRoute} bootstrapAdmin={access.bootstrapAdmin} opened={opened} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} onPermissionDirty={onPermissionDirty} onFormsDirty={onFormsDirty} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave} openForm={id=>navigate(runtimeFormPath(appId,id))} configureForm={id=>openConfiguration(id,location.pathname)} backFromConfiguration={()=>navigate(configurationReturn())} backToStructure={()=>navigate('/app/applications/'+encodeURIComponent(appId))} />
    : catalog ? <ApplicationCatalog applications={apps.items} loading={apps.loading} error={apps.error} retry={apps.reload} canCreate={canCreate} create={trigger => { createTrigger.current = trigger; setSuccess(''); setCreate(true); }} open={open} />
    : <><div className="app-page-heading"><h1>主页</h1></div><section className="app-surface app-home-section"><h2>我的应用</h2>{apps.loading ? <p role="status">正在加载应用…</p> : apps.error ? <div className="app-state"><p role="alert">{apps.error}</p><button className="admin-button" onClick={apps.reload}>重试</button></div> : apps.items.length ? <ApplicationCards applications={apps.items} open={open} /> : <div className="home-empty"><h3>暂无可用应用</h3><p>获得应用访问权限后，将在这里显示。</p></div>}</section><div><button className="admin-button" onClick={() => navigate('/app/applications')}>打开应用中心</button></div></>)}
  </main>
  {account && <section className="account-menu" aria-label="账号信息"><strong>{user.account}</strong><p>{access.bootstrapAdmin ? 'Bootstrap Admin' : access.identities.length ? access.identities.map(v => v.name).join('、') : '尚未分配身份'}</p>{access.permissions.map(p => <div key={p.code}><span>{p.name}</span>{p.sources?.map((s, i) => <small key={i}>{s.identityName}{s.templateName ? ' · ' + s.templateName : ' · 直接权限'}</small>)}</div>)}<button className="admin-button" disabled={pending} onClick={logout}>退出登录</button></section>}
  {create && <CreateApplication actorId={user.id} close={closeCreate} onDirty={onDirty} operation={operation} />}
 </WorkspaceShell>;
}
