export class ApplicationError extends Error {
 constructor(readonly status: number, readonly code: string, readonly unconfirmed = false) {
  super(unconfirmed ? '操作结果尚未确认，请核查原操作或使用同一操作重试'
   : status === 401 ? '登录已失效，请重新登录'
   : code === 'AUTH_SESSION_CHANGED' ? '登录身份已变化，请重新核对身份'
   : code === 'COMMON_CSRF_REJECTED' ? '安全校验失败，请重新登录后再试'
   : status === 403 ? '没有应用访问或管理权限'
   : code === 'APPLICATION_POLICY_CONFLICT' ? '权限配置已变化，请重新加载后再编辑'
   : code === 'APPLICATION_OPERATION_CONFLICT' ? '操作标识冲突，请核查原操作'
   : status === 404 ? '应用或配置不存在'
   : status === 400 ? '输入信息不合法，请检查后重试'
   : '服务暂时不可用，请稍后重试');
 }
}

export type ApplicationEnvelope<T> = { data: T; meta: { pagination?: { nextPageToken: string | null; hasMore: boolean } } | null; location?: string | null };

async function request<T>(actorId: string, path: string, method: string, body: object | undefined, signal: AbortSignal | undefined, expectedStatus: number | undefined, mutation: boolean, withStatus = false): Promise<ApplicationEnvelope<T> & {httpStatus?:number}> {
 const headers: Record<string, string> = { 'X-Expected-Actor-Id': actorId };
 if (body) headers['Content-Type'] = 'application/json';
 if (method !== 'GET') {
  const csrf = document.cookie.split(';').map(v => v.trim()).find(v => v.startsWith('__Host-csrf='));
  if (csrf) headers['X-CSRF-Token'] = csrf.slice('__Host-csrf='.length);
 }
 let response: Response;
 try { response = await fetch('/api/v1/' + path, { method, credentials: 'include', headers, body: body ? JSON.stringify(body) : undefined, signal }); }
 catch (cause) { if (signal?.aborted) throw cause; throw new ApplicationError(0, 'COMMON_SERVICE_UNAVAILABLE', mutation); }
 // Draft DELETE is an exact 204 with no envelope. A body, or a different
 // successful status, must never be mistaken for a confirmed deletion.
 if (expectedStatus === 204 && response.status === 204) {
  if ((await response.text()) !== '') throw new ApplicationError(204, 'APPLICATION_OPERATION_UNCONFIRMED', mutation);
  return { data: undefined as T, meta: null, location: response.headers.get('Location'), ...(withStatus?{httpStatus:response.status}:{}) };
 }
 let raw: unknown;
 try { raw = await response.json(); }
 catch { throw new ApplicationError(response.status, 'COMMON_SERVICE_UNAVAILABLE', mutation); }
 const envelope = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as { code?: unknown; data?: T; meta?: ApplicationEnvelope<T>['meta'] } : null;
 const code = typeof envelope?.code === 'string' ? envelope.code : 'COMMON_SERVICE_UNAVAILABLE';
 if (!response.ok || code !== 'OK') throw new ApplicationError(response.status, code, mutation && (response.ok || code === 'APPLICATION_OPERATION_UNCONFIRMED'));
 if (expectedStatus !== undefined && response.status !== expectedStatus) throw new ApplicationError(response.status, 'APPLICATION_OPERATION_UNCONFIRMED', mutation);
 if (!envelope || !Object.hasOwn(envelope, 'data')) throw new ApplicationError(response.status, 'APPLICATION_OPERATION_UNCONFIRMED', mutation);
 return { data: envelope.data as T, meta: envelope.meta ?? null, location: response.headers.get('Location'), ...(withStatus?{httpStatus:response.status}:{}) };
}

export async function applicationApiEnvelope<T>(actorId: string, path: string, method = 'GET', body?: object, signal?: AbortSignal, expectedStatus?: number): Promise<ApplicationEnvelope<T>> {
 return request(actorId, path, method, body, signal, expectedStatus, method !== 'GET');
}

// Search is a POST at the transport layer, but cannot create an operation.
// Keep this entrypoint narrow so mutation callers cannot erase uncertainty.
export async function applicationReadPost<T>(actorId: string, path: string, body: object, signal?: AbortSignal): Promise<T> {
 const uuid = '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';
 const recordWorkflows = new RegExp(`^applications/${uuid}/forms/${uuid}/records/${uuid}/workflow-instances/search$`).test(path);
 if (path !== 'workflow-tasks/search' && !recordWorkflows && !new RegExp(`^applications/${uuid}/forms/${uuid}/records/search$`, 'i').test(path)) throw new TypeError('Read-only POST is restricted to record search');
 return (await request<T>(actorId, path, 'POST', body, signal, 200, false)).data;
}

export async function applicationApi<T>(actorId: string, path: string, method = 'GET', body?: object, signal?: AbortSignal, expectedStatus?: number): Promise<T> {
 return (await applicationApiEnvelope<T>(actorId, path, method, body, signal, expectedStatus)).data;
}

// Only the frozen task action and its original execution-status resource may
// consume both 202 acceptance and 200 completion. Ordinary calls stay unchanged.
export function validWorkflowTransportRequest(path:string,method:string,body?:object):boolean{
 const id='[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}';
 const canonical=(value:unknown)=>typeof value==='string'&&new RegExp('^'+id+'$').test(value)&&value!=='00000000-0000-0000-0000-000000000000';
 const actionPath=new RegExp(`^applications/(${id})/forms/(${id})/records/(${id})/workflow-instances/(${id})/tasks/(${id})/actions$`).exec(path);
 const operationPath=new RegExp(`^application-workflow-operations/(${id})$`).exec(path);
 const data=body as Record<string,unknown>|undefined;
 const validPost=method==='POST'&&actionPath&&actionPath.slice(1).every(canonical)&&data&&!Array.isArray(data)&&Object.keys(data).length===3&&['operationId','action','basisToken'].every(k=>Object.hasOwn(data,k))&&canonical(data.operationId)&&(data.action==='agree'||data.action==='reject')&&typeof data.basisToken==='string'&&/^[A-Za-z0-9_-]{43}$/.test(data.basisToken);
 const validGet=method==='GET'&&operationPath&&canonical(operationPath[1])&&body===undefined;
 return Boolean(validPost||validGet);
}
export async function applicationWorkflowApi<T>(actorId:string,path:string,method:'GET'|'POST',body?:object,signal?:AbortSignal):Promise<ApplicationEnvelope<T>&{httpStatus:number}>{
 if(!validWorkflowTransportRequest(path,method,body))throw new TypeError('Workflow transport is restricted to task actions and original execution status');
 const result=await request<T>(actorId,path,method,body,signal,method==='GET'?200:undefined,method==='POST',true);
 if(result.httpStatus!==200&&result.httpStatus!==202)throw new ApplicationError(result.httpStatus??0,'APPLICATION_OPERATION_UNCONFIRMED',method==='POST');
 return {...result,httpStatus:result.httpStatus};
}
