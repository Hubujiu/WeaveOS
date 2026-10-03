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

async function formRequestEnvelope<T>(
  path: string, actorId: UUID, method: Method = 'GET', body?: object, signal?: AbortSignal,
  expectedStatus: 200 | 201 = 200,
): Promise<Envelope<T>> {
  const headers: Record<string, string> = {'X-Expected-Actor-Id':actorId};
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
  let raw: unknown;
  try {
    raw = await response.json();
  } catch {
    throw new FormApiError(response.status, method==='GET'?'INVALID_RESPONSE':'APPLICATION_OPERATION_UNCONFIRMED');
  }
  if(!record(raw)||typeof raw.code!=='string'||!('data' in raw))throw new FormApiError(response.status,
    method==='GET'?'INVALID_RESPONSE':'APPLICATION_OPERATION_UNCONFIRMED');
  const envelope=raw as Envelope<T>;
  if (response.ok&&envelope.code!=='OK'&&method!=='GET')
    throw new FormApiError(response.status,'APPLICATION_OPERATION_UNCONFIRMED',envelope.data);
  if (!response.ok || envelope.code !== 'OK') {
    throw new FormApiError(response.status, envelope.code ?? 'INVALID_RESPONSE', envelope.data);
  }
  if(response.status!==expectedStatus)throw new FormApiError(response.status,
    method==='GET'?'INVALID_RESPONSE':'APPLICATION_OPERATION_UNCONFIRMED');
  return envelope;
}
export async function formRequest<T>(
  path:string,actorId:UUID,method:Method='GET',body?:object,signal?:AbortSignal,expectedStatus:200|201=200,
):Promise<T>{return (await formRequestEnvelope<T>(path,actorId,method,body,signal,expectedStatus)).data;}

const prefix = (appId: UUID) => `applications/${encodeURIComponent(appId)}`;
const formPrefix = (appId: UUID, viewId: UUID) =>
  `${prefix(appId)}/forms/${encodeURIComponent(viewId)}`;
const record=(value:unknown):value is Record<string,unknown>=>
  !!value&&typeof value==='object'&&!Array.isArray(value);
const integer=(value:unknown)=>Number.isSafeInteger(value)&&Number(value)>=0;
const id=(value:unknown):value is UUID=>typeof value==='string'&&
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value);
const list=(value:unknown):value is unknown[]=>Array.isArray(value);
function validDirectory(value:unknown,appId:UUID,target?:UUID):boolean {
  return record(value)&&id(value.id)&&value.appId===appId&&(!target||value.id===target)&&
    typeof value.name==='string'&&(value.parentId===null||id(value.parentId))&&integer(value.position);
}
function validTable(value:unknown,appId:UUID):boolean {
  return record(value)&&id(value.id)&&value.appId===appId&&typeof value.name==='string'&&
    (value.directoryId===null||id(value.directoryId))&&integer(value.position)&&
    integer(value.schemaVersion)&&typeof value.schemaReady==='boolean';
}
function validForm(value:unknown,appId:UUID,target?:UUID):boolean {
  return record(value)&&id(value.id)&&value.appId===appId&&(!target||value.id===target)&&
    id(value.tableId)&&typeof value.name==='string'&&
    (value.directoryId===null||id(value.directoryId))&&integer(value.position)&&integer(value.viewVersion);
}
function validLayout(value:unknown):boolean {
  return list(value)&&value.every(node=>record(node)&&id(node.id)&&typeof node.kind==='string'&&
    (node.kind==='field'||node.kind==='system_field'?typeof node.fieldId==='string'&&integer(node.span??12):
      node.kind==='group'?typeof node.title==='string'&&integer(node.span??12)&&validLayout(node.children):
      node.kind==='divider'||node.kind==='description'&&typeof node.text==='string'));
}
export function validDefinition(value:unknown,appId:UUID,viewId:UUID,tableId?:UUID):value is Definition {
  if(!record(value)||value.appId!==appId||!validTable(value.table,appId)||
    !validForm(value.form,appId,viewId)||!record(value.table)||!record(value.form)||
    value.form.tableId!==value.table.id||(tableId&&value.table.id!==tableId)||
    !list(value.fields)||!value.fields.every(item=>record(item)&&id(item.id)&&
      typeof item.name==='string'&&typeof item.kind==='string'&&typeof item.required==='boolean'&&
      record(item.config)&&record(item.presentation))||!list(value.systemFields)||
    !validLayout(value.layout)||!record(value.capabilities)||
    typeof value.capabilities.canManageDefinition!=='boolean')return false;
  return true;
}
function checked<T>(value:T,valid:boolean,write=false):T {
  if(!valid)throw new FormApiError(200,write?'APPLICATION_OPERATION_UNCONFIRMED':'INVALID_RESPONSE');
  return value;
}
function validPreflight(value:unknown,appId:UUID,tableId:UUID,viewId:UUID,input:DefinitionInput):boolean {
  return record(value)&&value.appId===appId&&value.tableId===tableId&&value.viewId===viewId&&
    value.schemaVersion===input.expectedSchemaVersion&&value.viewVersion===input.expectedViewVersion&&
    integer(value.dataRevision)&&integer(value.dependencyRevision)&&record(value.plan)&&
    list(value.plan.schemaChanges)&&typeof value.plan.metadataChanged==='boolean'&&
    typeof value.plan.layoutChanged==='boolean'&&list(value.impacts)&&list(value.dependencies)&&
    list(value.blockingIssues)&&typeof value.saveAllowed==='boolean'&&
    (value.confirmation===null||record(value.confirmation)&&typeof value.confirmation.token==='string'&&
      typeof value.confirmation.expiresAt==='string');
}
export function validDirectoryResult(value:unknown,appId:UUID,input:StructureWrite,target?:UUID):value is DirectoryResult {
  return record(value)&&validDirectory(value.directory,appId,target)&&record(value.directory)&&
    value.directory.name===input.name&&value.directory.parentId===input.parentId&&
    value.structureVersion===input.expectedStructureVersion+1;
}
export function validFormResult(value:unknown,appId:UUID,input:FormWrite|FormUpdate,target?:UUID):value is FormResult {
  return record(value)&&validTable(value.table,appId)&&validForm(value.form,appId,target)&&
    record(value.table)&&record(value.form)&&value.form.tableId===value.table.id&&
    value.form.name===input.name&&value.form.directoryId===input.directoryId&&
    (!('source' in input)||input.source.kind==='new_table'||value.table.id===input.source.tableId)&&
    value.structureVersion===input.expectedStructureVersion+1;
}

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
/** Keep an original retry packet byte-for-byte stable within this SPA document. */
export function immutablePacket<T extends object>(input:T):T {
  const copy=structuredClone(input);
  const freeze=(value:unknown):void=>{
    if(!value||typeof value!=='object'||Object.isFrozen(value))return;
    Object.values(value).forEach(freeze);Object.freeze(value);
  };
  freeze(copy);return copy;
}
export type ApplicationOperation = {
  operationId: UUID; status: 'confirmed'; httpStatus: 200 | 201;
  location: string; result: unknown;
};
export type ReferenceCandidate={id:UUID;label:string;status:'active';parentId?:UUID|null};
export type CandidatePage={items:ReferenceCandidate[];nextPageToken:string|null;hasMore:boolean};
export function validSaveResult(value:unknown,appId:UUID,tableId:UUID,viewId:UUID,input:SaveWrite):value is DefinitionSave {
  return record(value)&&value.operationId===input.operationId&&
    validDefinition(value.definition,appId,viewId,tableId)&&record(value.appliedPlan)&&
    list(value.appliedPlan.schemaChanges)&&record(value.definition)&&
    record(value.definition.table)&&record(value.definition.form)&&
    Number(value.definition.table.schemaVersion)>=input.expectedSchemaVersion&&
    Number(value.definition.form.viewVersion)>=input.expectedViewVersion&&
    list(value.definition.fields)&&value.definition.fields.length===input.fields.length&&
    value.definition.fields.every((item,i)=>record(item)&&item.id===input.fields[i]?.id);
}

export const formApi = {
  candidates: async(appId:UUID,actorId:UUID,kind:'member'|'department',q:string,pageToken?:string,signal?:AbortSignal):Promise<CandidatePage>=>{
    const params=new URLSearchParams({q:q.trim(),pageSize:'20'});
    if(pageToken)params.set('pageToken',pageToken);
    const envelope=await formRequestEnvelope<{items:ReferenceCandidate[]}>(
      `${prefix(appId)}/${kind}-candidates?${params}`,actorId,'GET',undefined,signal);
    const data=envelope.data,meta=envelope.meta;
    if(!record(data)||!list(data.items)||!data.items.every(item=>record(item)&&id(item.id)&&
      typeof item.label==='string'&&item.status==='active'&&
      (kind==='member'||item.parentId===null||id(item.parentId)))||
      !record(meta)||!record(meta.pagination)||
      !(meta.pagination.nextPageToken===null||typeof meta.pagination.nextPageToken==='string')||
      typeof meta.pagination.hasMore!=='boolean')throw new FormApiError(200,'INVALID_RESPONSE');
    return {items:data.items as ReferenceCandidate[],nextPageToken:meta.pagination.nextPageToken as string|null,
      hasMore:meta.pagination.hasMore};
  },
  structure: async(appId:UUID,actorId:UUID,signal?:AbortSignal)=>{
    const value=await formRequest<Structure>(`${prefix(appId)}/structure`,actorId,'GET',undefined,signal);
    return checked(value,record(value)&&value.appId===appId&&integer(value.structureVersion)&&
      list(value.directories)&&value.directories.every(item=>validDirectory(item,appId))&&
      list(value.tables)&&value.tables.every(item=>validTable(item,appId))&&
      list(value.forms)&&value.forms.every(item=>validForm(item,appId))&&
      record(value.capabilities)&&typeof value.capabilities.canManageDefinition==='boolean');
  },
  createDirectory: async(appId:UUID,actorId:UUID,input:StructureWrite)=>{
    const value=await formRequest<DirectoryResult>(`${prefix(appId)}/directories`,actorId,'POST',input,undefined,201);
    return checked(value,validDirectoryResult(value,appId,input),true);
  },
  updateDirectory: async(appId:UUID,actorId:UUID,directoryId:UUID,input:StructureWrite)=>{
    const value=await formRequest<DirectoryResult>(`${prefix(appId)}/directories/${encodeURIComponent(directoryId)}`,actorId,'PUT',input);
    return checked(value,validDirectoryResult(value,appId,input,directoryId),true);
  },
  createForm: async(appId:UUID,actorId:UUID,input:FormWrite)=>{
    const value=await formRequest<FormResult>(`${prefix(appId)}/forms`,actorId,'POST',input,undefined,201);
    return checked(value,validFormResult(value,appId,input),true);
  },
  updateForm: async(appId:UUID,actorId:UUID,viewId:UUID,input:FormUpdate)=>{
    const value=await formRequest<FormResult>(formPrefix(appId,viewId),actorId,'PUT',input);
    return checked(value,validFormResult(value,appId,input,viewId),true);
  },
  definition: async(appId:UUID,actorId:UUID,viewId:UUID,signal?:AbortSignal)=>{
    const value=await formRequest<Definition>(`${formPrefix(appId,viewId)}/definition`,actorId,'GET',undefined,signal);
    return checked(value,validDefinition(value,appId,viewId));
  },
  preflight: async(appId:UUID,actorId:UUID,tableId:UUID,viewId:UUID,input:DefinitionInput,signal?:AbortSignal)=>{
    const value=await formRequest<Preflight>(`${formPrefix(appId,viewId)}/definition/preflight`,actorId,'POST',input,signal);
    return checked(value,validPreflight(value,appId,tableId,viewId,input));
  },
  save: async(appId:UUID,actorId:UUID,tableId:UUID,viewId:UUID,input:SaveWrite)=>{
    const value=await formRequest<DefinitionSave>(`${formPrefix(appId,viewId)}/definition`,actorId,'PUT',input);
    return checked(value,validSaveResult(value,appId,tableId,viewId,input),true);
  },
  operation: async(operationId:UUID,actorId:UUID)=>{
    const value=await formRequest<ApplicationOperation>(`application-operations/${encodeURIComponent(operationId)}`,actorId);
    return checked(value,record(value)&&value.operationId===operationId&&value.status==='confirmed'&&
      (value.httpStatus===200||value.httpStatus===201)&&record(value.result));
  },
};

export function formErrorText(error: unknown): string {
  if (!(error instanceof FormApiError)) return '服务暂时不可用，请稍后重试';
  if (error.status === 401) return '登录已失效，请重新登录';
  if (error.code === 'AUTH_SESSION_CHANGED') return '当前账号已变化，请切回原账号后核查未确认操作';
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
