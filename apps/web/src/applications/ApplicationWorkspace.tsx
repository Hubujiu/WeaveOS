import { useEffect, useState } from 'react';
import { applicationApi, ApplicationError } from './api';
import type { Application, ApplicationAccess, ApplicationFormSlots } from './types';
import { FormSlots } from './shell/FormSlots';

export function ApplicationWorkspace({ appId, opened, onUnauthorized, slots }: { appId: string; opened: (app: Application) => void; onUnauthorized: () => void; slots?: ApplicationFormSlots }) {
 const [result, setResult] = useState<{ id: string; app: Application | null; error: string; loading: boolean }>({ id: '', app: null, error: '', loading: true });
 const [retry, setRetry] = useState(0);
 useEffect(() => {
  let current = true; const controller = new AbortController();
  setResult({ id: appId, app: null, error: '', loading: true });
  Promise.all([
   applicationApi<Application>('applications/' + encodeURIComponent(appId), 'GET', undefined, controller.signal),
   applicationApi<ApplicationAccess>('applications/' + encodeURIComponent(appId) + '/access', 'GET', undefined, controller.signal),
  ]).then(([app, access]) => {
   if (!current) return;
   if (app.id !== appId || access.appId !== appId || !access.canEnter || !access.menus.some(menu => menu.resourceKind === 'application' && menu.resourceId === appId)) throw new ApplicationError(403, 'APPLICATION_FORBIDDEN');
   setResult({ id: appId, app, error: '', loading: false }); opened(app);
  }).catch(cause => {
   if (!current) return;
   if (cause instanceof ApplicationError && cause.status === 401) onUnauthorized();
   else setResult({ id: appId, app: null, error: cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试', loading: false });
  });
  return () => { current = false; controller.abort(); };
 }, [appId, retry, opened, onUnauthorized]);
 if (result.id !== appId || result.loading) return <section className="app-surface app-state" role="status">正在验证应用访问权限…</section>;
 if (result.error) return <section className="app-surface app-state"><h1>无法打开应用</h1><p role="alert">{result.error}</p><button className="admin-button" onClick={() => setRetry(v => v + 1)}>重试</button></section>;
 return <><h1 className="app-workspace-title">{result.app!.name}</h1><FormSlots slots={slots} /></>;
}

