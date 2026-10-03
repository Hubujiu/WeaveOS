import { useEffect, useRef, useState } from 'react';
import { workspaceApi, WorkspaceError } from '../workspace-api';
import type { User } from '../workspace-types';
import { applicationApi, ApplicationError } from './api';
import { cancelUnsentPreflights, clearPacket, clearRecovery, getRecovery, keepPacket, registerUnsentPreflight, type ApplicationPacket } from './recovery';
import type { ApplicationOperation } from './types';

type State = { phase: 'idle' | 'preflight' | 'pending' | 'error' | 'unconfirmed'; message: string; packet: ApplicationPacket | null };

export function useApplicationOperation<T>(actorId: string, confirmed: (result: T) => void, unauthorized: () => void, identityMismatch: () => void, valid: (result: T) => boolean, scope?: string) {
 const [state, setState] = useState<State>(() => {
  const previous = getRecovery(actorId, scope);
  return previous?.unknown && previous.packet
   ? { phase: 'unconfirmed', message: '操作结果尚未确认，请核查原操作或使用同一操作重试', packet: previous.packet }
   : { phase: 'idle', message: '', packet: null };
 });
 const snapshot = useRef<ApplicationPacket | null>(getRecovery(actorId, scope)?.packet ?? null);
 const uncertain = useRef(Boolean(getRecovery(actorId, scope)?.unknown));
 const inFlight = useRef(false);
 const lifecycle = useRef(0);
 useEffect(() => () => { lifecycle.current++; }, []);
 const perform = async (packet: ApplicationPacket, query: boolean) => {
  if (inFlight.current) return;
  const token = lifecycle.current;
  inFlight.current = true;
  let sent = false;
  const unsent = !query && !snapshot.current;
  let unregister = () => {};
  if (unsent) {
   const cancel = () => {
    unregister();
    if (sent || token !== lifecycle.current) return;
    lifecycle.current++;
    inFlight.current = false;
    setState({ phase: 'idle', message: '', packet: null });
   };
   unregister = registerUnsentPreflight(actorId, cancel, scope);
  }
  // This transition happens before awaiting identity verification. The UI
  // displays the immutable packet and cannot accept a second click or rename.
  setState({ phase: unsent ? 'preflight' : 'pending', message: '', packet });
  try {
   const current = await workspaceApi<User>('sessions/current');
   if (token !== lifecycle.current) return;
   if (current?.id !== packet.actorId || packet.actorId !== actorId) {
    setState(snapshot.current || uncertain.current || query
     ? { phase: 'unconfirmed', message: '操作结果尚未确认，请核查原操作或使用同一操作重试', packet }
     : { phase: 'idle', message: '', packet: null });
    identityMismatch(); return;
   }
   // Once the request can leave this tab, a route unmount may hide its reply.
   // Keep the original key conservatively until a valid confirmation arrives.
   if (unsent) { snapshot.current = packet; keepPacket(packet, true); }
   unregister();
   setState({ phase: 'pending', message: '', packet });
   sent = true;
   let result: T;
   if (query) {
    const operation = await applicationApi<ApplicationOperation<T>>(packet.actorId, 'application-operations/' + packet.body.operationId);
    if (operation.operationId !== packet.body.operationId || operation.status !== 'confirmed' || operation.httpStatus !== packet.expectedStatus) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
    result = operation.result;
   } else result = await applicationApi<T>(packet.actorId, packet.path, packet.method, packet.body, undefined, packet.expectedStatus);
   if (!valid(result)) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
   if (token !== lifecycle.current) return;
   snapshot.current = null; uncertain.current = false;
   clearRecovery(packet.actorId, packet.scope);
   setState({ phase: 'idle', message: '', packet: null });
   confirmed(result);
  } catch (cause) {
   if (token !== lifecycle.current) return;
   if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') {
    if (sent) { snapshot.current = packet; uncertain.current = true; keepPacket(packet, true); }
    if (snapshot.current || uncertain.current || query) setState({ phase: 'unconfirmed', message: '操作结果尚未确认，请核查原操作或使用同一操作重试', packet });
    identityMismatch(); return;
   }
   const authLost = cause instanceof ApplicationError && cause.status === 401 || cause instanceof WorkspaceError && cause.status === 401;
   const unknown = uncertain.current || query || sent && (authLost || cause instanceof ApplicationError && cause.unconfirmed);
   if (unknown) { snapshot.current = packet; uncertain.current = true; keepPacket(packet, true); }
   else { snapshot.current = null; clearPacket(packet.actorId, packet.scope); }
   if (authLost) { unauthorized(); return; }
   const reason = cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试';
   const message = unknown && !(cause instanceof ApplicationError && cause.unconfirmed)
    ? reason + '。操作结果仍未确认，请稍后核查或使用同一操作重试。' : reason;
   setState({ phase: unknown ? 'unconfirmed' : 'error', packet, message });
  } finally { unregister(); if (token === lifecycle.current) inFlight.current = false; }
 };
 const start = (path: string, body: object, method: 'POST' | 'PUT' = 'POST', expectedStatus: 200 | 201 = method === 'POST' ? 201 : 200) => {
  if (inFlight.current || uncertain.current) return;
  const packet: ApplicationPacket = { actorId, path, method, expectedStatus, scope, body: { ...body, operationId: crypto.randomUUID() } };
  void perform(packet, false);
 };
 const query = () => { if (snapshot.current) void perform(snapshot.current, true); };
 const retry = () => { if (snapshot.current) void perform(snapshot.current, false); };
 return { ...state, start, query, retry, cancelPreflight: () => cancelUnsentPreflights(actorId, scope) };
}
