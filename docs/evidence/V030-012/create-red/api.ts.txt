export class ApplicationError extends Error {
 constructor(readonly status: number, readonly code: string, readonly unconfirmed = false) {
  super(unconfirmed ? '操作结果尚未确认，请核查原操作或使用同一操作重试'
   : status === 401 ? '登录已失效，请重新登录'
   : code === 'COMMON_CSRF_REJECTED' ? '安全校验失败，请重新登录后再试'
   : status === 403 ? '没有应用访问或管理权限'
   : code === 'APPLICATION_POLICY_CONFLICT' ? '权限配置已变化，请重新加载后再编辑'
   : code === 'APPLICATION_OPERATION_CONFLICT' ? '操作标识冲突，请核查原操作'
   : status === 404 ? '应用或配置不存在'
   : status === 400 ? '输入信息不合法，请检查后重试'
   : '服务暂时不可用，请稍后重试');
 }
}

export async function applicationApi<T>(path: string, method = 'GET', body?: object, signal?: AbortSignal): Promise<T> {
 const write = method !== 'GET';
 const headers: Record<string, string> = {};
 if (body) headers['Content-Type'] = 'application/json';
 if (write) {
  const csrf = document.cookie.split(';').map(v => v.trim()).find(v => v.startsWith('__Host-csrf='));
  if (csrf) headers['X-CSRF-Token'] = csrf.slice('__Host-csrf='.length);
 }
 let response: Response;
 try { response = await fetch('/api/v1/' + path, { method, credentials: 'include', headers, body: body ? JSON.stringify(body) : undefined, signal }); }
 catch (cause) { if (signal?.aborted) throw cause; throw new ApplicationError(0, 'COMMON_SERVICE_UNAVAILABLE', write); }
 let envelope: { code: string; data: T };
 try { envelope = await response.json(); }
 catch { throw new ApplicationError(response.status, 'COMMON_SERVICE_UNAVAILABLE', write); }
 if (!response.ok || envelope.code !== 'OK') throw new ApplicationError(response.status, envelope.code, write && envelope.code === 'APPLICATION_OPERATION_UNCONFIRMED');
 return envelope.data;
}

