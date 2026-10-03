import { useCallback, useEffect, useState } from 'react';
import { applicationApi, ApplicationError } from './api';
import type { Application, ApplicationList } from './types';

export function useApplications(actorId: string, enabled: boolean, onUnauthorized: () => void, onIdentityMismatch: () => void) {
 const [items, setItems] = useState<Application[]>([]);
 const [loading, setLoading] = useState(true);
 const [error, setError] = useState('');
 const [revision, setRevision] = useState(0);
 const reload = useCallback(() => setRevision(v => v + 1), []);
 useEffect(() => {
  if (!enabled) { setItems([]); setLoading(false); setError(''); return; }
  const controller = new AbortController(); let current = true;
  setLoading(true); setError('');
  applicationApi<ApplicationList>(actorId, 'applications', 'GET', undefined, controller.signal).then(value => {
   if (!Array.isArray(value?.items)) throw new ApplicationError(503, 'COMMON_SERVICE_UNAVAILABLE');
   if (current) setItems(value.items);
  }).catch(cause => {
   if (!current) return;
   if (cause instanceof ApplicationError && cause.status === 401) onUnauthorized();
   else if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') onIdentityMismatch();
   else { setItems([]); setError(cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试'); }
  }).finally(() => { if (current) setLoading(false); });
  return () => { current = false; controller.abort(); };
 }, [actorId, enabled, revision, onUnauthorized, onIdentityMismatch]);
 return { items, loading, error, reload };
}
