import { useEffect, useRef, useState } from 'react';
import { applicationApi, ApplicationError } from './api';
import type { ApplicationOperation } from './types';

type Packet = { path: string; body: { operationId: string; [key: string]: unknown } };
type State = { phase: 'idle' | 'pending' | 'error' | 'unconfirmed'; message: string; packet: Packet | null };

export function useApplicationOperation<T>(confirmed: (result: T) => void, unauthorized: () => void, valid: (result: T) => boolean) {
 const [state, setState] = useState<State>({ phase: 'idle', message: '', packet: null });
 const snapshot = useRef<Packet | null>(null);
 const uncertain = useRef(false);
 const inFlight = useRef(false);
 const lifecycle = useRef(0);
 useEffect(() => () => { lifecycle.current++; }, []);
 const perform = async (packet: Packet, query: boolean) => {
  if (inFlight.current) return;
  const token = lifecycle.current;
  inFlight.current = true;
  setState({ phase: 'pending', message: '', packet });
  try {
   let result: T;
   if (query) {
    const operation = await applicationApi<ApplicationOperation<T>>('application-operations/' + packet.body.operationId);
    if (operation.operationId !== packet.body.operationId || operation.status !== 'confirmed' || operation.httpStatus !== 201) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
    result = operation.result;
   } else result = await applicationApi<T>(packet.path, 'POST', packet.body);
   if (!valid(result)) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
   if (token !== lifecycle.current) return;
   snapshot.current = null; uncertain.current = false;
   setState({ phase: 'idle', message: '', packet: null });
   confirmed(result);
  } catch (cause) {
   if (token !== lifecycle.current) return;
   if (cause instanceof ApplicationError && cause.status === 401) { unauthorized(); return; }
   const unknown = query || cause instanceof ApplicationError && cause.unconfirmed;
   uncertain.current = unknown;
   setState({ phase: unknown ? 'unconfirmed' : 'error', packet, message: query
    ? '操作结果仍未确认，请稍后核查或使用同一操作重试。'
    : cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试' });
  } finally { inFlight.current = false; }
 };
 const start = (path: string, body: object) => {
  if (inFlight.current || uncertain.current) return;
  const packet = { path, body: { ...body, operationId: crypto.randomUUID() } };
  snapshot.current = packet;
  void perform(packet, false);
 };
 const query = () => { if (snapshot.current) void perform(snapshot.current, true); };
 const retry = () => { if (snapshot.current) void perform(snapshot.current, false); };
 return { ...state, start, query, retry };
}

