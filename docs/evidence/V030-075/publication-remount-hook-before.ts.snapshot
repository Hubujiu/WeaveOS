import {validWorkflowManagementRequest,validWorkflowManagementReceipt} from './workflows/managementContract';
import {validPublicationRequest,validPublicationResult} from './workflows/publicationContract';
import {validWorkflowDefinitionRequest,validWorkflowDefinitionReceipt} from './workflows/definitionSaveContract';
import { useEffect, useRef, useState } from 'react';
import { workspaceApi, WorkspaceError } from '../workspace-api';
import type { User } from '../workspace-types';
import { applicationApi, applicationApiEnvelope, applicationWorkflowApi, applicationPublicationApi, validWorkflowTransportRequest, ApplicationError } from './api';
import { cancelUnsentPreflights, clearPacket, clearRecovery, getRecovery, keepPacket, registerUnsentPreflight, type ApplicationPacket } from './recovery';
import type { ApplicationOperation } from './types';
import { validTaskRecordRequest } from './workflows/taskSaveContract';
import { validWorkflowOperation } from './workflows/taskActionContract';
import { validPresetMutationRequest, validPresetMutationReceipt } from './records/presetContract';
import {isMutationResult} from './records/contracts';
import {validRoundStartRequest,validRoundStartReceipt,validRoundReworkRequest} from './workflows/roundContract';
import {validManualStartRequest,validManualStartReceipt} from './workflows/manualStartContract';
import { validInstanceActionRequest } from './workflows/lifecycleContract';

type State = { phase: 'idle' | 'preflight' | 'pending' | 'accepted' | 'error' | 'unconfirmed'; message: string; packet: ApplicationPacket | null; errorCode?: string };

export type ApplicationMutationRequest = {
 path: string;
 method: 'POST' | 'PUT' | 'PATCH' | 'DELETE';
 expectedStatus: 200 | 201 | 202 | 204;
 body?: Record<string, unknown>;
 expectedDraftVersion?: number;
 expectedVersion?: number;
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
 // Task execution receipts are checked against their distinct closed wire
 // in perform(), including actual HTTP status and the confirmed command ID.
 if (packet.resource.kind === 'workflow-publication') return true;
 if (packet.resource.kind === 'task-action'||packet.resource.kind === 'instance-action') return true;
 if (packet.resource.kind === 'workflow-management') return validWorkflowManagementReceipt(result,packet.operationId??'',packet.resource.id,packet.body?.expectedRevision as number,packet.resource.action);
 if (packet.resource.kind === 'workflow-definition') return validWorkflowDefinitionReceipt(result,packet.operationId??'',packet.resource.id,packet.body?.expectedRevision as number);
 if (packet.resource.kind === 'round-record') return isMutationResult(result,packet.operationId??'',packet.resource.id)&&result.recordVersion>0&&result.schemaVersion>0;
 if (packet.resource.kind === 'round-start') return validRoundStartReceipt(result,packet.operationId??'',packet.resource.flowId,packet.resource.instanceId,packet.resource.roundKind);
 if (packet.resource.kind === 'manual-start') return validManualStartReceipt(result,packet.operationId??'',packet.resource.flowId);
 if (packet.resource.kind === 'preset') return validPresetMutationReceipt(result,packet.operationId??'',packet.resource.id);
 const receipt = result as Record<string, unknown>;
 if (receipt.operationId !== packet.operationId || typeof receipt.id !== 'string' || !receipt.id) return false;
 if (packet.resource.id && receipt.id !== packet.resource.id) return false;
 const version = packet.resource.kind === 'draft' ? receipt.draftVersion : receipt.recordVersion;
 if (!Number.isSafeInteger(version) || (version as number) < 1) return false;
 if (packet.resource.kind === 'record' || packet.resource.kind === 'task-record') {
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
 const preflight = useRef(false);
 const lifecycle = useRef(0);
 useEffect(() => () => { lifecycle.current++; }, []);
 const perform = async (packet: ApplicationPacket, query: boolean) => {
  if (inFlight.current) return;
  const token = ++lifecycle.current;
  inFlight.current = true;
  let sent = false;
  const unsent = !query && !snapshot.current;
  let unregister = () => {};
  if (unsent) {
   preflight.current = true;
   const cancel = () => {
    unregister();
    if (sent || token !== lifecycle.current) return;
    lifecycle.current++;
    preflight.current = false;
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
    preflight.current = false;
    setState(snapshot.current || uncertain.current || query
     ? { phase: 'unconfirmed', message: '操作结果尚未确认，请核查原操作或使用同一操作重试', packet }
     : { phase: 'idle', message: '', packet: null });
    identityMismatch(); return;
   }
   // Once the request can leave this tab, a route unmount may hide its reply.
   // Keep the original key conservatively until a valid confirmation arrives.
   if (unsent) { keepPacket(packet, true); snapshot.current = getRecovery(packet.actorId, packet.scope)!.packet; }
   unregister();
   preflight.current = false;
   setState({ phase: 'pending', message: '', packet });
   sent = true;
   let result: T;
   let location: string | null | undefined;
   const taskAction = packet.resource?.kind === 'task-action'||packet.resource?.kind === 'instance-action';
   if (packet.resource?.kind === 'workflow-publication') {
    const resource=packet.resource,base='applications/'+resource.appId+'/forms/'+resource.viewId+'/workflows/'+resource.id;
    const reply=await applicationPublicationApi<T>(packet.actorId,query?base+'/publications/'+packet.operationId:packet.path,query?'GET':'POST',query?undefined:packet.body);
    result=reply.data;location=reply.location;
    if(!validPublicationResult(result,packet.operationId??'',resource.id,resource.version,reply.httpStatus,query?'GET':'POST'))throw new ApplicationError(503,'APPLICATION_OPERATION_UNCONFIRMED',true);
    if(result.status==='pending'||result.status==='unknown'){
     if(!query){let target:URL|null=null;try{if(typeof location==='string')target=new URL(location,window.location.origin);}catch{/* Keep the original uncertainty. */}
      if(!target||target.origin!==window.location.origin||target.pathname!=='/api/v1/'+base+'/publications/'+packet.operationId||target.search||target.hash)throw new ApplicationError(503,'APPLICATION_OPERATION_UNCONFIRMED',true);
     }
     if(token!==lifecycle.current)return;
     snapshot.current=packet;uncertain.current=true;keepPacket(packet,true);inFlight.current=false;
     setState({phase:'accepted',message:result.status==='pending'?'流程发布处理中':'发布结果待确认',packet});return;
    }
   } else if (taskAction) {
    const reply = await applicationWorkflowApi<T>(packet.actorId, query ? 'application-workflow-operations/' + packet.operationId : packet.path, query ? 'GET' : 'POST', query ? undefined : packet.body);
    result = reply.data; location = reply.location;
    const context = packet.context as {commandId?:string} | undefined;
    if (!validWorkflowOperation(result, {operationId:packet.operationId??'', instanceId:packet.resource&&(packet.resource.kind==='task-action'||packet.resource.kind==='instance-action')?packet.resource.instanceId:'', commandId:context?.commandId,...(packet.resource?.kind==='instance-action'?{action:'withdraw' as const}:{})}, reply.httpStatus, query?'GET':'POST')) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
    if (!query && result.status === 'pending') {
     let target:URL|null=null;
     try { if(typeof location==='string')target=new URL(location,window.location.origin); } catch { /* Invalid receipt Location remains unknown. */ }
     if (!target || target.origin !== window.location.origin || target.pathname !== '/api/v1/application-workflow-operations/' + packet.operationId || target.search || target.hash) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
    }
    if (token !== lifecycle.current) return;
    if (result.status === 'pending') {
     packet = {...packet, context:{commandId:result.commandId}};
     keepPacket(packet, true); snapshot.current = getRecovery(packet.actorId, packet.scope)!.packet; uncertain.current = true;
     setState({phase:'accepted',message:'审批处理中',packet:snapshot.current});
     return;
    }
   } else if (query) {
    const operation = await applicationApi<ApplicationOperation<T>>(packet.actorId, 'application-operations/' + (packet.operationId ?? packet.body?.operationId));
    if (operation.operationId !== (packet.operationId ?? packet.body?.operationId) || operation.status !== 'confirmed' || operation.httpStatus !== packet.expectedStatus) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
    result = operation.result;
    location = operation.location;
   } else {
    const reply = await applicationApiEnvelope<T>(packet.actorId, packet.path, packet.method, packet.body, undefined, packet.expectedStatus);
    result = reply.data; location = reply.location;
   }
   if (!(packet.expectedStatus === 204 && !query) && !valid(result, packet)) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
   if (scopedResource && packet.resource?.kind !== 'workflow-definition' && packet.expectedStatus === 201 && !validLocation(location, packet, result)) throw new ApplicationError(503, 'APPLICATION_OPERATION_UNCONFIRMED', true);
   if (packet.resource?.kind==='manual-start'||packet.resource?.kind==='round-start') {
    let target:URL|null=null;
    try { if(typeof location==='string')target=new URL(location,window.location.origin); } catch { /* Invalid durable intent receipt remains uncertain. */ }
    if(!target||target.origin!==window.location.origin||target.pathname!=='/api/v1/application-operations/'+packet.operationId||target.search||target.hash)throw new ApplicationError(503,'APPLICATION_OPERATION_UNCONFIRMED',true);
   }
   if (token !== lifecycle.current) return;
   snapshot.current = null; uncertain.current = false;
   clearRecovery(packet.actorId, packet.scope);
   setState({ phase: 'idle', message: '', packet: null });
   // A valid receipt ends the write synchronously, before observers can start
   // another operation from inside the confirmation callback.
   inFlight.current = false;
   lifecycle.current++;
   confirmed(result);
  } catch (cause) {
   if (token !== lifecycle.current) return;
   preflight.current = false;
   if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') {
    if (sent) { snapshot.current = packet; uncertain.current = true; keepPacket(packet, true); }
    if (snapshot.current || uncertain.current || query) setState({ phase: 'unconfirmed', message: '操作结果尚未确认，请核查原操作或使用同一操作重试', packet });
    identityMismatch(); return;
   }
   const authLost = cause instanceof ApplicationError && cause.status === 401 || cause instanceof WorkspaceError && cause.status === 401;
   const ambiguousHttp = cause instanceof ApplicationError && (cause.status === 408 || cause.status >= 500);
   const unknown = uncertain.current || query || sent && (authLost || cause instanceof ApplicationError && (cause.unconfirmed || ambiguousHttp));
   if (unknown) { snapshot.current = packet; uncertain.current = true; keepPacket(packet, true); }
   else { snapshot.current = null; clearPacket(packet.actorId, packet.scope); }
   inFlight.current = false;
   if (authLost) { unauthorized(); return; }
   const reason = cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试';
   const message = unknown && !(cause instanceof ApplicationError && cause.unconfirmed)
    ? reason + '。操作结果仍未确认，请稍后核查或使用同一操作重试。' : reason;
   setState({ phase: unknown ? 'unconfirmed' : 'error', packet, message, ...(unknown || !(cause instanceof ApplicationError) ? {} : { errorCode: cause.code }) });
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
   if (request.resource.kind === 'workflow-management') {
    if (!validWorkflowManagementRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'workflow-publication') {
    if (!validPublicationRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'workflow-definition') {
    if (!validWorkflowDefinitionRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'round-record') {
    if (!validRoundReworkRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'round-start') {
    if (!validRoundStartRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'manual-start') {
    if (!validManualStartRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'preset') {
    if (!validPresetMutationRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'instance-action') {
    if (!validInstanceActionRequest(request.resource,request)) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'task-action') {
    const r=request.resource;
    const expectedPath=`applications/${r.appId}/forms/${r.viewId}/records/${r.id}/workflow-instances/${r.instanceId}/tasks/${r.taskId}/actions`;
    if (request.path!==expectedPath || request.method!=='POST' || request.expectedStatus!==202 || request.body&&Object.hasOwn(request.body,'operationId') || !validWorkflowTransportRequest(request.path,request.method,{...request.body,operationId})) throw new TypeError('Invalid scoped mutation packet');
   } else if (request.resource.kind === 'task-record') {
    if (!validTaskRecordRequest(request.resource, request)) throw new TypeError('Invalid scoped mutation packet');
   } else {
   const base = 'applications/' + encodeURIComponent(request.resource.appId) + '/forms/' + encodeURIComponent(request.resource.viewId) + '/' + (request.resource.kind === 'draft' ? 'drafts' : 'records');
   const expectedPath = request.method === 'POST' ? base : base + '/' + encodeURIComponent(request.resource.id ?? '');
   if (!['POST','PATCH','DELETE'].includes(request.method) || request.path !== expectedPath || request.path.includes('?') || request.path.includes('#') || request.method === 'DELETE' && (request.resource.kind !== 'draft' || !Number.isSafeInteger(request.expectedDraftVersion) || (request.expectedDraftVersion as number) < 1) || request.method === 'POST' && (request.expectedStatus !== 201 || !request.resource.creationNonce) || request.method === 'PATCH' && request.expectedStatus !== 200 || request.method === 'DELETE' && request.expectedStatus !== 204 || request.body && Object.hasOwn(request.body, 'operationId')) throw new TypeError('Invalid scoped mutation packet');
   }
   const versionKey = request.resource.kind === 'preset' ? 'expectedVersion' : 'expectedDraftVersion';
   const version = request.resource.kind === 'preset' ? request.expectedVersion : request.expectedDraftVersion;
   const query = request.method === 'DELETE' ? { operationId, [versionKey]: version } : undefined;
   const path = query ? request.path + '?operationId=' + encodeURIComponent(operationId) + '&' + versionKey + '=' + encodeURIComponent(String(version)) : request.path;
   packet = { actorId, path, method: request.method, expectedStatus: request.expectedStatus, scope, operationId, query, resource: structuredClone(request.resource), body: request.method === 'DELETE' ? undefined : { ...structuredClone(request.body ?? {}), operationId } };
  }
  void perform(packet, false);
 };
 const query = () => { if (snapshot.current) void perform(snapshot.current, true); };
 const retry = () => { if (snapshot.current) void perform(snapshot.current, false); };
 const getPendingStatus = (): 'preflight' | 'write_in_flight' | 'unknown' | null => {
  if (uncertain.current) return 'unknown';
  if (preflight.current) return 'preflight';
  if (inFlight.current) return 'write_in_flight';
  return snapshot.current ? 'unknown' : null;
 };
 return { ...state, start, query, retry, getPendingStatus,
  dismissError: () => setState(previous => previous.phase === 'error' ? { phase: 'idle', message: '', packet: null } : previous),
  cancelPreflight: () => cancelUnsentPreflights(actorId, scope) };
}
