import { useCallback, useEffect, useState } from 'react';
import { applicationApi, ApplicationError } from './api';
import type { Application, ApplicationAccess, ApplicationFormSlots } from './types';
import { FormSlots } from './shell/FormSlots';
import { ApplicationStructurePanel, FormDesigner, type RegisterLeaveGuard } from './forms';
import { PermissionManager } from './permissions/PermissionManager';
import { Modal } from '../Modal';
import { cancelUnsentPreflights, clearScopedDrafts, scopedUnconfirmed } from './recovery';

export function ApplicationWorkspace({ actorId, appId, viewId, bootstrapAdmin, opened, onUnauthorized, onIdentityMismatch, onPermissionDirty, onFormsDirty, registerLeaveGuard, requestSectionLeave, openForm, backToStructure, slots }: { actorId: string; appId: string; viewId?: string; bootstrapAdmin: boolean; opened: (app: Application) => void; onUnauthorized: () => void; onIdentityMismatch: () => void; onPermissionDirty: (dirty: boolean, summary: string) => void; onFormsDirty: (dirty: boolean) => void; registerLeaveGuard: RegisterLeaveGuard; requestSectionLeave: (action: () => void) => void; openForm: (viewId: string) => void; backToStructure: () => void; slots?: ApplicationFormSlots }) {
 const [result, setResult] = useState<{ id: string; app: Application | null; error: string; loading: boolean }>({ id: '', app: null, error: '', loading: true });
 const [retry, setRetry] = useState(0);
 const [permissionsOpen, setPermissionsOpen] = useState(false);
 const [permissionSummary, setPermissionSummary] = useState('');
 const [closeGuard, setCloseGuard] = useState(false);
 const permissionDirty = useCallback((dirty: boolean, summary: string) => { setPermissionSummary(dirty ? summary : ''); onPermissionDirty(dirty, summary); }, [onPermissionDirty]);
 useEffect(() => { setPermissionsOpen(false); onPermissionDirty(false, ''); }, [appId, viewId, onPermissionDirty]);
 useEffect(() => {
  let current = true; const controller = new AbortController();
  setResult({ id: appId, app: null, error: '', loading: true });
  Promise.all([
   applicationApi<Application>(actorId, 'applications/' + encodeURIComponent(appId), 'GET', undefined, controller.signal),
   applicationApi<ApplicationAccess>(actorId, 'applications/' + encodeURIComponent(appId) + '/access', 'GET', undefined, controller.signal),
  ]).then(([app, access]) => {
   if (!current) return;
   if (app.id !== appId || access.appId !== appId || !access.canEnter || !access.menus.some(menu => menu.resourceKind === 'application' && menu.resourceId === appId)) throw new ApplicationError(403, 'APPLICATION_FORBIDDEN');
   setResult({ id: appId, app, error: '', loading: false }); opened(app);
  }).catch(cause => {
   if (!current) return;
   if (cause instanceof ApplicationError && cause.status === 401) onUnauthorized();
   else if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') onIdentityMismatch();
   else setResult({ id: appId, app: null, error: cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试', loading: false });
  });
  return () => { current = false; controller.abort(); };
 }, [actorId, appId, retry, opened, onUnauthorized, onIdentityMismatch]);
 if (result.id !== appId || result.loading) return <section className="app-surface app-state" role="status">正在验证应用访问权限…</section>;
 if (result.error) return <section className="app-surface app-state"><h1>无法打开应用</h1><p role="alert">{result.error}</p><button className="admin-button" onClick={() => setRetry(v => v + 1)}>重试</button></section>;
 const canManage = bootstrapAdmin || result.app!.ownerUserId === actorId;
 return <><div className="app-workspace-heading"><h1 className="app-workspace-title">{result.app!.name}</h1>{canManage && !viewId && <button type="button" className="admin-button" aria-expanded={permissionsOpen} onClick={() => { if (permissionsOpen && permissionSummary) setCloseGuard(true); else if (permissionsOpen) setPermissionsOpen(false); else requestSectionLeave(() => setPermissionsOpen(true)); }}>权限管理</button>}</div>{permissionsOpen && canManage ? <PermissionManager actorId={actorId} appId={appId} onDirty={permissionDirty} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} /> : slots ? <FormSlots slots={slots} /> : <><div className="app-form-tabs" role="tablist" aria-label="应用表单"><button type="button" role="tab" aria-selected={!viewId} onClick={backToStructure}>应用目录</button>{viewId && <button type="button" role="tab" aria-selected="true">当前表单</button>}</div>{viewId ? <FormDesigner actorId={actorId} appId={appId} viewId={viewId} onBack={backToStructure} onDirtyChange={onFormsDirty} registerLeaveGuard={registerLeaveGuard} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} /> : <ApplicationStructurePanel actorId={actorId} appId={appId} onOpenForm={openForm} onDirtyChange={onFormsDirty} registerLeaveGuard={registerLeaveGuard} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} />}</>}{closeGuard && <Modal title="有未保存的修改" onClose={() => setCloseGuard(false)}><p>关闭权限管理将丢失未保存的更改（{permissionSummary}）。尚未发送的请求会取消。{scopedUnconfirmed(actorId).length ? '已发送的请求结果仍未确认，原操作会保留供核查。' : ''}</p><div className="dialog-actions"><button className="admin-button" onClick={() => setCloseGuard(false)}>继续编辑</button><button className="admin-button primary" onClick={() => { cancelUnsentPreflights(actorId); clearScopedDrafts(actorId, appId + '/group/'); setCloseGuard(false); setPermissionsOpen(false); onPermissionDirty(false, ''); }}>放弃权限组更改并保留待核查操作</button></div></Modal>}</>;
}
