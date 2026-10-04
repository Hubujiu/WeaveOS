import { useCallback, useEffect, useRef, useState } from 'react';
import { useBlocker, useLocation, useNavigate } from 'react-router';
import { workspaceApi, WorkspaceError } from './workspace-api';
import type { Access, User } from './workspace-types';
import { PersonnelAdmin } from './PersonnelAdmin';
import { Modal } from './Modal';
import { AppShell } from './applications/shell/AppShell';
import './workspace.css';

export function Workspace({user,logout,pending,error}:{user:User;logout:()=>void;pending:boolean;error:string}){
 const location=useLocation();const navigate=useNavigate();const [access,setAccess]=useState<Access|null>(null);
 const [failure,setFailure]=useState('');const [retry,setRetry]=useState(0);const [menu,setMenu]=useState(false);const [dirty,setDirty]=useState(false);
 const dirtyRef=useRef(false);
 const updateDirty=useCallback((value:boolean)=>{dirtyRef.current=value;setDirty(value);},[]);
 const blocker=useBlocker(({currentLocation,nextLocation})=>dirtyRef.current&&currentLocation.pathname!==nextLocation.pathname);
 const admin=location.pathname.startsWith('/app/admin');
 useEffect(()=>{let active=true;setFailure('');workspaceApi<Access>('me/access').then(value=>{if(active)setAccess(value);}).catch(e=>{if(!active)return;if(e instanceof WorkspaceError&&e.status===401)navigate('/login',{replace:true});else setFailure(e instanceof Error?e.message:'服务暂时不可用，请稍后重试');});return()=>{active=false;};},[retry,admin,navigate]);
 useEffect(()=>{if(!dirty)return;const warn=(e:BeforeUnloadEvent)=>{e.preventDefault();};window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);},[dirty]);
 useEffect(()=>{if(!admin)updateDirty(false);},[admin,updateDirty]);
 if(!access)return <div className="workspace"><div className="workspace-loading">{failure?<><p role="alert">{failure}</p><button className="admin-button" onClick={()=>setRetry(v=>v+1)}>重试</button><button className="admin-button" onClick={()=>setMenu(!menu)}>账号</button>{menu&&<><p>{user.account}</p><button className="admin-button" disabled={pending} onClick={logout}>退出登录</button></>}</>:<p role="status">正在加载主页…</p>}</div></div>;
 return <div className="workspace">
  {admin?(access.personnelManage?<PersonnelAdmin access={access} onDirty={updateDirty} onExit={()=>navigate('/app')} onUnauthorized={()=>{updateDirty(false);navigate('/login',{replace:true});}}/>:<main className="workspace-loading"><p role="alert">没有人员管理权限</p><button className="admin-button" onClick={()=>navigate('/app')}>返回主页</button></main>):<AppShell user={user} access={access} logout={logout} pending={pending} error={error} onDirty={updateDirty}/>}
  {blocker.state==='blocked'&&<Modal title="有未保存的修改" onClose={()=>blocker.reset()}><p>离开将丢失未保存的配置。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>blocker.reset()}>继续编辑</button><button className="admin-button primary" onClick={()=>{updateDirty(false);blocker.proceed();}}>放弃修改</button></div></Modal>}
 </div>;
}
