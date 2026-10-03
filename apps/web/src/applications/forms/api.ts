import type {
  Definition, DefinitionInput, DefinitionSave, DirectoryResult, FormResult,
  FormSource, Preflight, Structure, UUID,
} from './contracts';

export class FormApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly details: unknown = null,
  ) {
    super(code);
    this.name = 'FormApiError';
  }
}

type Method = 'GET' | 'POST' | 'PUT';
type Envelope<T> = { code: string; message: string; data: T; meta: unknown };

function csrfToken(): string | undefined {
  return document.cookie.split(';').map(part => part.trim())
    .find(part => part.startsWith('__Host-csrf='))?.slice('__Host-csrf='.length);
}

export async function formRequest<T>(
  path: string, method: Method = 'GET', body?: object, signal?: AbortSignal,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET') {
    const csrf = csrfToken();
    if (csrf) headers['X-CSRF-Token'] = csrf;
  }
  let response: Response;
  try {
    response = await fetch(`/api/v1/${path}`, {
      method, headers, credentials: 'include',
      body: body === undefined ? undefined : JSON.stringify(body), signal,
    });
  } catch (error) {
    if (signal?.aborted) throw error;
    // A lost write response is not proof of rollback.
    throw new FormApiError(0, 'NETWORK_UNKNOWN');
  }
  let envelope: Envelope<T>;
  try {
    envelope = await response.json() as Envelope<T>;
  } catch {
    throw new FormApiError(response.status, 'INVALID_RESPONSE');
  }
  if (!response.ok || envelope.code !== 'OK') {
    throw new FormApiError(response.status, envelope.code ?? 'INVALID_RESPONSE', envelope.data);
  }
  return envelope.data;
}

const prefix = (appId: UUID) => `applications/${encodeURIComponent(appId)}`;
const formPrefix = (appId: UUID, viewId: UUID) =>
  `${prefix(appId)}/forms/${encodeURIComponent(viewId)}`;

export type StructureWrite = {
  operationId: UUID; name: string; parentId: UUID | null;
  position: number; expectedStructureVersion: number;
};
export type FormWrite = {
  operationId: UUID; name: string; source: FormSource;
  directoryId: UUID | null; position: number; expectedStructureVersion: number;
};
export type FormUpdate = {
  operationId: UUID; name: string; directoryId: UUID | null;
  position: number; expectedStructureVersion: number;
};
export type SaveWrite = DefinitionInput & {
  operationId: UUID; confirmationToken: string | null;
};
export type ApplicationOperation = {
  operationId: UUID; status: 'confirmed'; httpStatus: 200 | 201;
  location: string; result: unknown;
};

export const formApi = {
  structure: (appId: UUID, signal?: AbortSignal) =>
    formRequest<Structure>(`${prefix(appId)}/structure`, 'GET', undefined, signal),
  createDirectory: (appId: UUID, input: StructureWrite) =>
    formRequest<DirectoryResult>(`${prefix(appId)}/directories`, 'POST', input),
  updateDirectory: (appId: UUID, directoryId: UUID, input: StructureWrite) =>
    formRequest<DirectoryResult>(`${prefix(appId)}/directories/${encodeURIComponent(directoryId)}`, 'PUT', input),
  createForm: (appId: UUID, input: FormWrite) =>
    formRequest<FormResult>(`${prefix(appId)}/forms`, 'POST', input),
  updateForm: (appId: UUID, viewId: UUID, input: FormUpdate) =>
    formRequest<FormResult>(formPrefix(appId, viewId), 'PUT', input),
  definition: (appId: UUID, viewId: UUID, signal?: AbortSignal) =>
    formRequest<Definition>(`${formPrefix(appId, viewId)}/definition`, 'GET', undefined, signal),
  preflight: (appId: UUID, viewId: UUID, input: DefinitionInput) =>
    formRequest<Preflight>(`${formPrefix(appId, viewId)}/definition/preflight`, 'POST', input),
  save: (appId: UUID, viewId: UUID, input: SaveWrite) =>
    formRequest<DefinitionSave>(`${formPrefix(appId, viewId)}/definition`, 'PUT', input),
  operation: (operationId: UUID) =>
    formRequest<ApplicationOperation>(`application-operations/${encodeURIComponent(operationId)}`),
};

export function formErrorText(error: unknown): string {
  if (!(error instanceof FormApiError)) return '服务暂时不可用，请稍后重试';
  if (error.status === 401) return '登录已失效，请重新登录';
  if (error.status === 403) return '没有此应用的表单配置权限';
  if (error.status === 404) return '对象不存在或当前无权访问';
  if (error.code === 'APPLICATION_SCHEMA_CONFLICT' || error.code === 'APPLICATION_VIEW_CONFLICT'
      || error.code === 'APPLICATION_STRUCTURE_CONFLICT') return '配置已被其他编辑者修改，请核对最新版本';
  if (error.code === 'APPLICATION_SCHEMA_DEPENDENCY_BLOCKED') return '字段仍被流程或其他视图引用，无法保存';
  if (error.code === 'APPLICATION_SCHEMA_REQUIRED_BACKFILL') return '旧记录需要默认值或先补齐后才能保存';
  if (error.code === 'APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED') return '请为已使用的选项指定映射';
  if (error.code === 'APPLICATION_SCHEMA_CONFIRMATION_REQUIRED') return '请先预检并确认当前数据影响';
  if (error.code === 'APPLICATION_SCHEMA_CONFIRMATION_STALE') return '影响确认已失效，请重新预检';
  if (error.code === 'APPLICATION_SCHEMA_CONVERSION_FAILED') return '旧数据无法转换为所选字段类型';
  if (error.code === 'APPLICATION_OPERATION_UNCONFIRMED') return '操作结果暂未确认，请查询原操作';
  if (error.code === 'NETWORK_UNKNOWN') return '网络连接中断，请稍后重试';
  if (error.status === 400) return '输入不符合字段或布局规则，请检查后重试';
  return '服务暂时不可用，请稍后重试';
}
