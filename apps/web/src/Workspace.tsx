import { useCallback, useEffect, useRef, useState } from 'react';
import { useBlocker, useLocation, useNavigate } from 'react-router';
import { workspaceApi, WorkspaceError } from './workspace-api';
import type { Access, User } from './workspace-types';
import { PersonnelAdmin } from './PersonnelAdmin';
import { Modal } from './Modal';
import { AppShell } from './applications/shell/AppShell';
import { cancelUnsentPreflights, clearRecovery, clearScopedDrafts, getRecovery, scopedUnconfirmed } from './applications/recovery';
import { LeaveGuards, type ActiveLeaveGuard, type RegisterLeaveGuard } from './applications/shell/leaveGuards';
import './workspace.css';

export function Workspace({user,logout,pending,error}:{user:User;logout:()=>void;pending:boolean;error:string}){
 const location=useLocation();const navigate=useNavigate();const [access,setAccess]=useState<Access|null>(null);
 const [failure,setFailure]=useState('');const [retry,setRetry]=useState(0);const [menu,setMenu]=useState(false);const [dirty,setDirty]=useState(false);
 const [identityMismatch,setIdentityMismatch]=useState(false);
 const [dirtySource,setDirtySource]=useState<'application'|'personnel'|'permissions'|'forms'|null>(null);
 const [dirtySummary,setDirtySummary]=useState('');
 const [leaveSnapshot,setLeaveSnapshot]=useState<ActiveLeaveGuard[]>([]);
 const [leaveError,setLeaveError]=useState('');
 const [internalPrompt,setInternalPrompt]=useState(false);
 const internalAction=useRef<(()=>void)|null>(null);
 const dirtyRef=useRef(false);
 const dirtySources=useRef(new Map<'application'|'personnel'|'permissions'|'forms',string>());
 const leaveGuards=useRef(new LeaveGuards());
 const authExit=useRef(false);
 const registerLeaveGuard=useCallback<RegisterLeaveGuard>((scope,controller)=>leaveGuards.current.register(scope,controller),[]);
 const updateDirty=useCallback((value:boolean,source:'application'|'personnel'|'permissions'|'forms'|null=null,summary='')=>{if(!source)dirtySources.current.clear();else if(value)dirtySources.current.set(source,summary);else dirtySources.current.delete(source);dirtyRef.current=dirtySources.current.size>0;setDirty(dirtyRef.current);setDirtySource(dirtySources.current.keys().next().value??null);setDirtySummary([...dirtySources.current.values()].filter(Boolean).join('、'));},[]);
 const applicationDirty=useCallback((value:boolean)=>updateDirty(value,'application'),[updateDirty]);
 const personnelDirty=useCallback((value:boolean)=>updateDirty(value,'personnel'),[updateDirty]);
 const permissionDirty=useCallback((value:boolean,summary:string)=>updateDirty(value,'permissions',summary),[updateDirty]);
 const formsDirty=useCallback((value:boolean)=>updateDirty(value,'forms'),[updateDirty]);
 const requestSectionLeave=useCallback((action:()=>void)=>{
  const snapshot=leaveGuards.current.snapshot(user.id);
  if(snapshot.every(entry=>entry.status==='clean')&&!dirtySources.current.has('forms')){action();return;}
  internalAction.current=action;setLeaveSnapshot(snapshot);setLeaveError('');setInternalPrompt(true);
 },[user.id]);
 const onIdentityMismatch=useCallback(()=>{setIdentityMismatch(true);updateDirty(false);},[updateDirty]);
 const blocker=useBlocker(({currentLocation,nextLocation})=>{
  if (authExit.current && nextLocation.pathname === '/login') return false;
  if (currentLocation.pathname === nextLocation.pathname && currentLocation.search === nextLocation.search) return false;
  const active=leaveGuards.current.snapshot(user.id);
  const otherDirty=[...dirtySources.current.keys()].some(source=>source!=='forms');
  return otherDirty || active.some(entry=>entry.status!=='clean') || (dirtySources.current.has('forms') && !active.length);
 });
 useEffect(()=>{if(blocker.state==='blocked'){setLeaveSnapshot(leaveGuards.current.snapshot(user.id));setLeaveError('');}},[blocker.state,user.id]);
 const admin=location.pathname.startsWith('/app/admin');
 useEffect(()=>{authExit.current=false;},[user.id]);
 const forceLogin=useCallback(()=>{authExit.current=true;updateDirty(false);navigate('/login',{replace:true});},[navigate,updateDirty]);
 const confirmLeave=()=>{
  if(blocker.state!=='blocked'&&!internalPrompt)return;
  if(dirtySources.current.has('forms')&&!leaveSnapshot.length){setLeaveError('表单离开保护尚未就绪，请继续编辑后重试。');return;}
  const result=leaveGuards.current.prepare(leaveSnapshot);
  if(!result.ok){setLeaveSnapshot(result.current);setLeaveError('离开期间状态已变化，请核对当前后果并再次确认。');return;}
  if(internalPrompt){const action=internalAction.current;internalAction.current=null;setInternalPrompt(false);updateDirty(false,'forms');action?.();return;}
  if(blocker.state!=='blocked')return;
  if(dirtySources.current.has('application')&&!getRecovery(user.id)?.unknown){cancelUnsentPreflights(user.id);clearRecovery(user.id);}
  if(dirtySources.current.has('permissions')){
   cancelUnsentPreflights(user.id);
   const appId=location.pathname.match(/^\/app\/applications\/([^/]+)/)?.[1];
   if(appId)clearScopedDrafts(user.id,decodeURIComponent(appId)+'/group/');
  }
  updateDirty(false);blocker.proceed();
 };
 const consequences=[
  dirtySources.current.has('personnel')?'未保存的人员更改将丢失。':'',
  dirtySources.current.has('application')?(getRecovery(user.id)?.unknown?'应用创建结果尚未确认，原操作保留供核查。':'未保存的应用名称将丢失；尚未发送的创建请求会取消。'):'',
  getRecovery(user.id)?.unknown&&!dirtySources.current.has('application')?'应用创建结果仍未确认，原操作保留供核查。':'',
  dirtySources.current.has('permissions')?'未保存的权限组更改将丢失（'+dirtySummary+'）；尚未发送的请求会取消。'+(scopedUnconfirmed(user.id).length?' 已发送的权限组操作保留供核查。':''):'',
  ...leaveSnapshot.filter(entry=>entry.status!=='clean').map(entry=>(entry.scope.kind==='structure'?'应用目录':'表单设计器')+(entry.status==='draft'?'草稿将丢失。':entry.status==='preflight'?'尚未发送的预检将取消，草稿将丢失。':'已发送的操作会保留原请求供核查。')),
 ].filter(Boolean).join(' ');
 useEffect(()=>{let active=true;setFailure('');workspaceApi<Access>('me/access').then(value=>{if(!active)return;setAccess(value);setIdentityMismatch(value.user?.id!==user.id);if(value.user?.id!==user.id)updateDirty(false);}).catch(e=>{if(!active)return;if(e instanceof WorkspaceError&&e.status===401)forceLogin();else setFailure(e instanceof Error?e.message:'服务暂时不可用，请稍后重试');});return()=>{active=false;};},[retry,admin,user.id,updateDirty,forceLogin]);
 useEffect(()=>{if(!dirty)return;const warn=(e:BeforeUnloadEvent)=>{e.preventDefault();};window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);},[dirty]);
 useEffect(()=>{if(!admin)updateDirty(false);},[admin,updateDirty]);
 if(!access)return <div className="workspace"><div className="workspace-loading">{failure?<><p role="alert">{failure}</p><button className="admin-button" onClick={()=>setRetry(v=>v+1)}>重试</button><button className="admin-button" onClick={()=>setMenu(!menu)}>账号</button>{menu&&<><p>{user.account}</p><button className="admin-button" disabled={pending} onClick={logout}>退出登录</button></>}</>:<p role="status">正在加载主页…</p>}</div></div>;
 const suspended=identityMismatch||access.user?.id!==user.id;
 return <div className="workspace monochrome-workspace">
  {<AppShell key={user.id} user={user} access={access} logout={logout} pending={pending} error={error} onDirty={applicationDirty} onPermissionDirty={permissionDirty} onFormsDirty={formsDirty} registerLeaveGuard={registerLeaveGuard} requestSectionLeave={requestSectionLeave} onAuthLost={forceLogin} onIdentityMismatch={onIdentityMismatch} suspended={suspended} adminContent={admin ? (access.personnelManage ? <PersonnelAdmin access={access} onDirty={personnelDirty} onExit={()=>navigate('/app')} onUnauthorized={forceLogin}/> : <section className="app-state"><p role="alert">没有人员管理权限</p><button className="admin-button" onClick={()=>navigate('/app')}>返回主页</button></section>) : undefined}/> }
  {suspended?<main className="workspace-loading"><p role="alert">登录身份已变化，原账号的未确认操作仍保留在此标签页。</p>{failure&&<p role="alert">{failure}</p>}<button className="admin-button" onClick={()=>setRetry(v=>v+1)}>重新核对身份</button><button className="admin-button" onClick={()=>{updateDirty(false);navigate('/login',{replace:true});}}>载入当前账号</button></main>:null}
  {(blocker.state==='blocked'||internalPrompt)&&<Modal title="有未保存的修改" onClose={()=>{if(blocker.state==='blocked')blocker.reset();internalAction.current=null;setInternalPrompt(false);}}><p>{consequences||'当前操作状态需重新确认。'}</p>{leaveError&&<p role="alert">{leaveError}</p>}<div className="dialog-actions"><button className="admin-button" onClick={()=>{if(blocker.state==='blocked')blocker.reset();internalAction.current=null;setInternalPrompt(false);}}>继续编辑</button><button className="admin-button primary" onClick={confirmLeave}>{dirtySource==='personnel'&&getRecovery(user.id)?.unknown?'放弃人员更改并保留待核查操作':dirtySource==='permissions'?'放弃权限组更改并保留待核查操作':getRecovery(user.id)?.unknown?'离开并保留待核查操作':'放弃修改'}</button></div></Modal>}
 </div>;
}
