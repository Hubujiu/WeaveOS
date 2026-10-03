import { useEffect, useRef, useState } from 'react';
import { workspaceApi, WorkspaceError } from '../workspace-api';
import type { User } from '../workspace-types';
import { applicationApi, applicationApiEnvelope, ApplicationError } from './api';
import { cancelUnsentPreflights, clearPacket, clearRecovery, getRecovery, keepPacket, registerUnsentPreflight, type ApplicationPacket } from './recovery';
import type { ApplicationOperation } from './types';

type State = { phase: 'idle' | 'preflight' | 'pending' | 'error' | 'unconfirmed'; message: string; packet: ApplicationPacket | null };

export type ApplicationMutationRequest = {
 path: string;
 method: 'POST' | 'PATCH' | 'DELETE';
 expectedStatus: 200 | 201 | 204;
 body?: Record<string, unknown>;
 expectedDraftVersion?: number;
 resource: NonNullable<ApplicationPacket['resource']>;
};
export type ScopedApplicationOperationOptions<T> = {
 actorId: string;
 scope?: string;
 resource: NonNullable<ApplicationPacket['resource']>;
 confirmed: (result: T | undefined) => void;
 unauthorized: () => void;
 identityMismatch: () => void;
 valid?: (result: T, packet: ApplicationPacket) => boolean;
};

function validScopedReceipt(result: unknown, packet: ApplicationPacket): boolean {
 if (!result || typeof result !== 'object' || Array.isArray(result) || !packet.resource) return false;
 const receipt = result as Record<string, unknown>;
 if (receipt.operationId !== packet.operationId || typeof receipt.id !== 'string' || !receipt.id) return false;
 if (packet.resource.id && receipt.id !== packet.resource.id) return false;
 const version = packet.resource.kind === 'draft' ? receipt.draftVersion : receipt.recordVersion;
 if (!Number.isSafeInteger(version) || (version as number) < 1) return false;
 if (packet.resource.kind === 'record') {
  if (!Number.isSafeInteger(receipt.schemaVersion) || (receipt.schemaVersion as number) < 1) return false;
  if (typeof receipt.createdAt !== 'string' || typeof receipt.updatedAt !== 'string') return false;
 }
 return true;
}

function validLocation(location: string | null | undefined, packet: ApplicationPacket, result: unknown): boolean {
 if (typeof location !== 'string' || !result || typeof result !== 'object') return false;
 const id = (result as { id?: unknown }).id;
 if (typeof id !== 'string' || !id) return false;
 const expected = '/api/v1/' + packet.path.split('?')[0].replace(/\/$/, '') + '/' + encodeURIComponent(id);
 try {
  const parsed = new URL(location, window.location.origin);
  return parsed.origin === window.location.origin && parsed.pathname === expected && !parsed.search && !parsed.hash;
 } catch { return false; }
}

export function useApplicationOperation<T>(options: ScopedApplicationOperationOptions<T>): ReturnType<typeof useOperation<T>>;
export function useApplicationOperation<T>(actorId: string, confirmed: (result: T) => void, unauthorized: () => void, identityMismatch: () => void, valid: (result: T) => boolean, scope?: string, onDefinitePolicyConflict?: () => void): ReturnType<typeof useOperation<T>>;
export function useApplicationOperation<T>(actorOrOptions: string | ScopedApplicationOperationOptions<T>, confirmed?: (result: T) => void, unauthorized?: () => void, identityMismatch?: () => void, valid?: (result: T) => boolean, scope?: string, onDefinitePolicyConflict?: () => void) {
 const scoped = typeof actorOrOptions === 'string' ? null : actorOrOptions;
 const scopedKey = scoped ? JSON.stringify([scoped.scope ?? '', scoped.resource]) : scope;
 return useOperation<T>(scoped?.actorId ?? actorOrOptions as string, (scoped?.confirmed ?? confirmed!) as (result: T | undefined) => void, scoped?.unauthorized ?? unauthorized!, scoped?.identityMismatch ?? identityMismatch!, scoped ? (result, packet) => validScopedReceipt(result, packet) && (scoped.valid?.(result, packet) ?? true) : valid!, scopedKey, onDefinitePolicyConflict, scoped?.resource);
}

function useOperation<T>(actorId: string, confirmed: (result: T | undefined) => void, unauthorized: () => void, identityMismatch: () => void, valid: (result: T, packet: ApplicationPacket) => boolean, scope?: string, onDefinitePolicyConflict?: () => void, scopedResource?: NonNullable<ApplicationPacket['resource']>) {
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
   if (unsent) { keepPacket(packet, true); snapshot.current = getRecovery(packet.actorId, packet.scope)!.packet; }
   unregister();
   setState({ phase: 'pending', message: '', packet });
   sent = true;
   let result: T;
   let location: string | null | undefined;
   if (query) {
    const operation = await applicationApi<ApplicationOperation<T>>(packet.actorId, 'application-operations/' + (packet.operationId ?? packet.body?.operationId));
    if (operation.operationId !== (packet.operationId ?? packet.body?.operationId) || operation.status !== 'confirmed' || operation.httpStatus !== packet.expectedStatus) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
    result = operation.result;
    location = operation.location;
   } else {
    const reply = await applicationApiEnvelope<T>(packet.actorId, packet.path, packet.method, packet.body, undefined, packet.expectedStatus);
    result = reply.data; location = reply.location;
   }
   if (!(packet.expectedStatus === 204 && !query) && !valid(result, packet)) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
   if (scopedResource && packet.expectedStatus === 201 && !validLocation(location, packet, result)) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
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
   if (!unknown && cause instanceof ApplicationError && cause.status === 409 && cause.code === 'APPLICATION_POLICY_CONFLICT') onDefinitePolicyConflict?.();
  } finally { unregister(); if (token === lifecycle.current) inFlight.current = false; }
 };
 const start = (requestOrPath: string | ApplicationMutationRequest, body?: object, method: 'POST' | 'PUT' = 'POST', expectedStatus: 200 | 201 = method === 'POST' ? 201 : 200) => {
  if (inFlight.current || uncertain.current) return;
  const operationId = crypto.randomUUID();
  let packet: ApplicationPacket;
  if (typeof requestOrPath === 'string') packet = { actorId, path: requestOrPath, method, expectedStatus, scope, operationId, body: { ...body, operationId } };
  else {
   const request = requestOrPath;
   if (!scope || !scopedResource || JSON.stringify(request.resource) !== JSON.stringify(scopedResource)) throw new TypeError('Mutation resource does not match the mounted scope');
   const base = 'applications/' + encodeURIComponent(request.resource.appId) + '/forms/' + encodeURIComponent(request.resource.viewId) + '/' + (request.resource.kind === 'draft' ? 'drafts' : 'records');
   const expectedPath = request.method === 'POST' ? base : base + '/' + encodeURIComponent(request.resource.id ?? '');
   if (request.path !== expectedPath || request.path.includes('?') || request.path.includes('#') || request.method === 'DELETE' && (request.resource.kind !== 'draft' || !Number.isSafeInteger(request.expectedDraftVersion) || (request.expectedDraftVersion as number) < 1) || request.method === 'POST' && (request.expectedStatus !== 201 || !request.resource.creationNonce) || request.method === 'PATCH' && request.expectedStatus !== 200 || request.method === 'DELETE' && request.expectedStatus !== 204 || request.body && Object.hasOwn(request.body, 'operationId')) throw new TypeError('Invalid scoped mutation packet');
   const query = request.method === 'DELETE' ? { operationId, expectedDraftVersion: request.expectedDraftVersion } : undefined;
   const path = query ? request.path + '?operationId=' + encodeURIComponent(operationId) + '&expectedDraftVersion=' + encodeURIComponent(String(request.expectedDraftVersion)) : request.path;
   packet = { actorId, path, method: request.method, expectedStatus: request.expectedStatus, scope, operationId, query, resource: structuredClone(request.resource), body: request.method === 'DELETE' ? undefined : { ...structuredClone(request.body ?? {}), operationId } };
  }
  void perform(packet, false);
 };
 const query = () => { if (snapshot.current) void perform(snapshot.current, true); };
 const retry = () => { if (snapshot.current) void perform(snapshot.current, false); };
 return { ...state, start, query, retry,
  dismissError: () => setState(previous => previous.phase === 'error' ? { phase: 'idle', message: '', packet: null } : previous),
  cancelPreflight: () => cancelUnsentPreflights(actorId, scope) };
}
