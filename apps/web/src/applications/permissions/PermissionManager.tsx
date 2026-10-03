import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Modal } from '../../Modal';
import { applicationApi, applicationApiEnvelope, ApplicationError } from '../api';
import { cancelUnsentPreflights, clearScopedDraft, clearScopedDrafts, getRecovery, keepScopedDraft, scopedUnconfirmed } from '../recovery';
import { useApplicationOperation } from '../useApplicationOperation';
import type { MenuGrant, PermissionGroup, PolicyResult } from '../types';

type GroupList = { items: PermissionGroup[]; policyRevision: number };
type MemberLabel = { id: string; label: string; status: 'active' | 'disabled'; selectable: boolean };
type Members = { memberIds: string[]; members: MemberLabel[]; policyRevision: number };
type Grants = { grants: unknown[]; policyRevision: number };
type Candidate = { id: string; label: string; status: 'active' };
type CandidatePage = { items: Candidate[] };
type Section = '基本信息' | '成员' | '菜单';
type BasicDraft = { name: string; enabled: boolean; originalName: string; originalEnabled: boolean; revision: number };
type MemberDraft = { ids: string[]; originalIds: string[]; labels: Record<string, MemberLabel>; revision: number };
type GrantDraft = { checked: boolean; originalChecked: boolean; unsupported: boolean; revision: number };

const validRevision = (value: unknown): value is number => Number.isSafeInteger(value) && Number(value) >= 1;
const uuid = /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i;
const sameIds = (a: string[], b: string[]) => a.length === b.length && a.every(id => b.includes(id));
const basicDirty = (value: BasicDraft) => value.name !== value.originalName || value.enabled !== value.originalEnabled;
const memberDirty = (value: MemberDraft) => !sameIds(value.ids, value.originalIds);
const grantDirty = (value: GrantDraft) => value.checked !== value.originalChecked;
const rootGrant = (appId: string): MenuGrant => ({ resourceKind: 'application', resourceId: appId, action: 'menu.enter', rowScope: 'all', fields: [] });
const isRootGrant = (value: unknown, appId: string) => {
 if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
 const grant = value as Record<string, unknown>;
 return Object.keys(grant).length === 5 && grant.resourceKind === 'application' && grant.resourceId === appId && grant.action === 'menu.enter' && grant.rowScope === 'all' && Array.isArray(grant.fields) && grant.fields.length === 0;
};
const listValid = (value: GroupList) => !!value && Array.isArray(value.items) && validRevision(value.policyRevision) && value.items.every(group => uuid.test(group.id) && typeof group.name === 'string' && typeof group.enabled === 'boolean' && validRevision(group.policyRevision));
const membersValid = (value: Members) => !!value && Array.isArray(value.memberIds) && Array.isArray(value.members) && validRevision(value.policyRevision) && value.memberIds.every(id => uuid.test(id)) && value.members.length === value.memberIds.length && value.members.every(member => value.memberIds.includes(member.id) && typeof member.label === 'string' && ['active', 'disabled'].includes(member.status) && typeof member.selectable === 'boolean');
const grantsValid = (value: Grants) => !!value && Array.isArray(value.grants) && validRevision(value.policyRevision);
const message = (cause: unknown) => cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试';

export function PermissionManager({ actorId, appId, onDirty, onUnauthorized, onIdentityMismatch }: {
 actorId: string; appId: string; onDirty: (dirty: boolean, summary: string) => void; onUnauthorized: () => void; onIdentityMismatch: () => void;
}) {
 const [list, setList] = useState<GroupList | null>(null);
 const [loading, setLoading] = useState(true);
 const [error, setError] = useState('');
 const [selectedId, setSelectedId] = useState('');
 const [editorDirty, setEditorDirty] = useState('');
 const [nextId, setNextId] = useState<string | null>(null);
 const [create, setCreate] = useState(false);
 const [newName, setNewName] = useState(() => { const recovery = getRecovery(actorId, appId + '/group/new'); return typeof recovery?.draft === 'string' ? recovery.draft : typeof recovery?.packet?.body.name === 'string' ? recovery.packet.body.name : ''; });
 const [createError, setCreateError] = useState('');
 const [createStatus, setCreateStatus] = useState('');
 const nameInput = useRef<HTMLInputElement>(null);
 const current = useRef(0);
 const path = 'applications/' + encodeURIComponent(appId) + '/permission-groups';
 const handleError = useCallback((cause: unknown) => {
  if (cause instanceof ApplicationError && cause.status === 401) onUnauthorized();
  else if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') onIdentityMismatch();
  else setError(message(cause));
 }, [onUnauthorized, onIdentityMismatch]);
 const reload = useCallback(async () => {
  const sequence = ++current.current;
  setLoading(true); setError('');
  try {
   const response = await applicationApi<GroupList>(actorId, path);
   if (!listValid(response)) throw new Error('权限组响应无效，请重试');
   if (sequence === current.current) setList(response);
  } catch (cause) { if (sequence === current.current) handleError(cause); }
  finally { if (sequence === current.current) setLoading(false); }
 }, [actorId, path, handleError]);
 useEffect(() => { void reload(); return () => { current.current++; }; }, [reload]);
 const createOperation = useApplicationOperation<PermissionGroup>(actorId, group => {
  clearScopedDraft(actorId, appId + '/group/new'); setNewName(''); setCreate(false); setSelectedId(group.id); setCreateStatus('权限组已创建'); void reload();
 }, onUnauthorized, onIdentityMismatch, group => !!group && uuid.test(group.id) && typeof group.name === 'string' && group.enabled === true && validRevision(group.policyRevision), appId + '/group/new');
 const createBusy = createOperation.phase === 'preflight' || createOperation.phase === 'pending';
 useEffect(() => { if (create) queueMicrotask(() => nameInput.current?.focus()); }, [create]);
 const unknown = createOperation.phase === 'unconfirmed' || scopedUnconfirmed(actorId).some(packet => packet.scope?.startsWith(appId + '/group/'));
 const otherUnknown = scopedUnconfirmed(actorId).some(packet => packet.scope?.startsWith(appId + '/group/') && packet.scope !== appId + '/group/new');
 useEffect(() => { if (newName) keepScopedDraft(actorId, appId + '/group/new', newName); else clearScopedDraft(actorId, appId + '/group/new'); }, [actorId, appId, newName]);
 useEffect(() => {
  const names = [editorDirty, newName.trim() ? '新建权限组名称' : '', createBusy ? '新建权限组请求尚未发送或正在提交' : '', unknown ? '权限组请求已发送，结果仍未确认' : ''].filter(Boolean);
  onDirty(names.length > 0, names.join('、'));
  return () => onDirty(false, '');
 }, [editorDirty, newName, createBusy, unknown, onDirty]);
 const select = (id: string) => { if (id === selectedId) return; if (editorDirty || createBusy || newName.trim()) { setNextId(id); return; } setSelectedId(id); setCreate(false); };
 const selected = list?.items.find(group => group.id === selectedId);
 const createGroup = () => {
  const name = newName.trim();
  if (!name || [...name].length > 100 || name.includes('\0')) { setCreateError('权限组名称须为 1–100 个字符'); return; }
  if (!list) return;
  setCreateError(''); setCreateStatus('');
  createOperation.start(path, { name, expectedPolicyRevision: list.policyRevision });
 };
 return <section className="app-surface app-permission-manager" aria-label="权限管理">
  <div className="app-permission-heading"><div><h2>权限管理</h2><p>基本信息、成员和菜单分别保存。菜单仅支持当前应用入口。</p></div><button type="button" className="admin-button" onClick={reload} disabled={loading}>刷新权限组</button></div>
  {createStatus && <p className="app-success" role="status">{createStatus}</p>}
  {error && <div className="app-state"><p role="alert">{error}</p><button className="admin-button" onClick={reload}>重试</button></div>}
  {loading && <p role="status">正在加载权限组…</p>}
  {unknown && <p className="workspace-status" role="status">有权限组操作结果尚未确认。选择原权限组或打开新建权限组，使用原操作核查。</p>}
  {list && <div className="app-permission-layout">
   <div className="definition-list app-group-list"><button className="admin-button primary" disabled={otherUnknown} onClick={() => { if (editorDirty || createBusy) setNextId('__create__'); else setCreate(true); }}>新建权限组</button>
    {list.items.length ? list.items.map(group => <button type="button" aria-label={group.name} className={'definition-item' + (group.id === selectedId ? ' selected' : '')} key={group.id} onClick={() => select(group.id)}><strong>{group.name}</strong><span>{group.enabled ? '已启用' : '已停用'}</span></button>) : <p className="empty-state">暂无权限组</p>}
   </div>
   <div className="definition-details app-group-details">{selected ? <GroupEditor key={appId + selected.id} actorId={actorId} appId={appId} group={selected} reloadList={reload} onDirty={setEditorDirty} onUnauthorized={onUnauthorized} onIdentityMismatch={onIdentityMismatch} /> : <p className="empty-state">选择权限组查看配置</p>}</div>
  </div>}
  {create && <Modal title="新建权限组" onClose={() => { if (!createBusy && !newName.trim() && createOperation.phase !== 'unconfirmed') setCreate(false); else setNextId('__close__'); }} busy={createOperation.phase === 'pending'}><label>权限组名称<input ref={nameInput} autoFocus value={(createBusy || createOperation.phase === 'unconfirmed') && typeof createOperation.packet?.body.name === 'string' ? createOperation.packet.body.name : newName} disabled={createBusy || createOperation.phase === 'unconfirmed'} onChange={event => { setNewName(event.target.value); setCreateError(''); }} /></label>{createError && <p role="alert">{createError}</p>}{createOperation.message && <p role="alert">{createOperation.message}</p>}<div className="dialog-actions">{createOperation.phase === 'unconfirmed' ? <><button className="admin-button" onClick={createOperation.query}>核查操作</button><button className="admin-button primary" onClick={createOperation.retry}>使用同一操作重试</button></> : <><button className="admin-button" disabled={createOperation.phase === 'pending'} onClick={() => { if (newName.trim() || createBusy) setNextId('__close__'); else setCreate(false); }}>取消</button><button className="admin-button primary" disabled={createBusy} onClick={createGroup}>{createBusy ? '正在核验或创建…' : '创建权限组'}</button></>}</div></Modal>}
  {nextId !== null && <Modal title="有未保存的修改" onClose={() => setNextId(null)}><p>{editorDirty || newName.trim() ? '离开将丢失未保存的权限组更改；尚未发送的请求会取消。' : '尚未发送的请求会取消。'}{unknown && ' 已发送的权限组操作结果仍未确认，原操作会保留供核查。'}</p><div className="dialog-actions"><button className="admin-button" onClick={() => setNextId(null)}>继续编辑</button><button className="admin-button primary" onClick={() => { cancelUnsentPreflights(actorId); if (selectedId) clearScopedDrafts(actorId, appId + '/group/' + selectedId + '/'); clearScopedDraft(actorId, appId + '/group/new'); const destination = nextId; setNextId(null); setNewName(''); setEditorDirty(''); if (destination === '__close__') setCreate(false); else if (destination === '__create__') { setSelectedId(''); setCreate(true); } else { setSelectedId(destination); setCreate(false); } }}>放弃未保存并保留待核查操作</button></div></Modal>}
 </section>;
}

function GroupEditor({ actorId, appId, group, reloadList, onDirty, onUnauthorized, onIdentityMismatch }: {
 actorId: string; appId: string; group: PermissionGroup; reloadList: () => Promise<void>; onDirty: (summary: string) => void; onUnauthorized: () => void; onIdentityMismatch: () => void;
}) {
 const scope = appId + '/group/' + group.id + '/';
 const recoveredBasic = getRecovery(actorId, scope + 'basic')?.packet?.body;
 const basicDraft = getRecovery(actorId, scope + 'basic')?.draft as BasicDraft | undefined;
 const [basic, setBasic] = useState<BasicDraft>(basicDraft ?? { name: typeof recoveredBasic?.name === 'string' ? recoveredBasic.name : group.name, enabled: typeof recoveredBasic?.enabled === 'boolean' ? recoveredBasic.enabled : group.enabled, originalName: group.name, originalEnabled: group.enabled, revision: typeof recoveredBasic?.expectedPolicyRevision === 'number' ? recoveredBasic.expectedPolicyRevision : group.policyRevision });
 const [members, setMembers] = useState<MemberDraft | null>(() => (getRecovery(actorId, scope + 'members')?.draft as MemberDraft | undefined) ?? null);
 const [grants, setGrants] = useState<GrantDraft | null>(() => (getRecovery(actorId, scope + 'menu')?.draft as GrantDraft | undefined) ?? null);
 const [loading, setLoading] = useState(true);
 const [error, setError] = useState('');
 const [status, setStatus] = useState('');
 const [reloadRequired, setReloadRequired] = useState(false);
 const confirmedSection = useRef<Section | null>(null);
 const [candidateQuery, setCandidateQuery] = useState('');
 const [pageToken, setPageToken] = useState('');
 const [previousTokens, setPreviousTokens] = useState<string[]>([]);
 const [candidates, setCandidates] = useState<Candidate[]>([]);
 const [candidateLoading, setCandidateLoading] = useState(true);
 const [candidateError, setCandidateError] = useState('');
 const [candidateRetry, setCandidateRetry] = useState(0);
 const [nextToken, setNextToken] = useState<string | null>(null);
 const [hasMore, setHasMore] = useState(false);
 const sequence = useRef(0);
 const base = 'applications/' + encodeURIComponent(appId) + '/permission-groups/' + encodeURIComponent(group.id);
 const handleError = useCallback((cause: unknown) => {
  if (cause instanceof ApplicationError && cause.status === 401) onUnauthorized();
  else if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') onIdentityMismatch();
  else setError(message(cause));
 }, [onUnauthorized, onIdentityMismatch]);
 const reread = useCallback(async (saved?: Section) => {
  if (saved) confirmedSection.current = saved;
  const section = saved ?? confirmedSection.current;
  const token = ++sequence.current;
  setError(''); setLoading(true);
  if (section) setReloadRequired(true);
  try {
   const [list, memberResult, grantResult] = await Promise.all([
    applicationApi<GroupList>(actorId, 'applications/' + encodeURIComponent(appId) + '/permission-groups'),
    applicationApi<Members>(actorId, base + '/members'),
    applicationApi<Grants>(actorId, base + '/grants'),
   ]);
   const latest = list.items?.find(value => value.id === group.id);
   if (!listValid(list) || !latest || !membersValid(memberResult) || !grantsValid(grantResult)) throw new Error('权限配置响应无效，请重试');
   if (token !== sequence.current) return;
   setBasic(previous => section === '基本信息' || !basicDirty(previous) ? { name: latest.name, enabled: latest.enabled, originalName: latest.name, originalEnabled: latest.enabled, revision: list.policyRevision } : previous);
   setMembers(previous => {
    if (previous && section !== '成员' && memberDirty(previous)) return previous;
    const packet = section ? null : getRecovery(actorId, scope + 'members')?.packet?.body;
    return { ids: Array.isArray(packet?.memberIds) && packet.memberIds.every(id => typeof id === 'string') ? packet.memberIds as string[] : [...memberResult.memberIds], originalIds: [...memberResult.memberIds], labels: Object.fromEntries(memberResult.members.map(value => [value.id, value])), revision: typeof packet?.expectedPolicyRevision === 'number' ? packet.expectedPolicyRevision : list.policyRevision };
   });
   setGrants(previous => {
    if (previous && section !== '菜单' && grantDirty(previous)) return previous;
    const packet = section ? null : getRecovery(actorId, scope + 'menu')?.packet?.body;
    const originalChecked = grantResult.grants.some(value => isRootGrant(value, appId));
    return { checked: Array.isArray(packet?.grants) ? packet.grants.some(value => isRootGrant(value, appId)) : originalChecked, originalChecked, unsupported: grantResult.grants.some(value => !isRootGrant(value, appId)), revision: typeof packet?.expectedPolicyRevision === 'number' ? packet.expectedPolicyRevision : list.policyRevision };
   });
   confirmedSection.current = null;
   setReloadRequired(false);
   if (section) { setStatus(section + '已保存'); void reloadList(); }
  } catch (cause) { if (token === sequence.current) { if (section) setStatus(section + '已保存，但重读失败；请重试读取配置'); handleError(cause); } }
  finally { if (token === sequence.current) setLoading(false); }
 }, [actorId, appId, base, group.id, scope, handleError, reloadList]);
 useEffect(() => { void reread(); return () => { sequence.current++; }; }, [reread]);
 const basicOperation = useApplicationOperation<PermissionGroup>(actorId, () => { clearScopedDraft(actorId, scope + 'basic'); void reread('基本信息'); }, onUnauthorized, onIdentityMismatch, value => !!value && value.id === group.id && typeof value.name === 'string' && typeof value.enabled === 'boolean' && validRevision(value.policyRevision), scope + 'basic');
 const memberOperation = useApplicationOperation<PolicyResult>(actorId, () => { clearScopedDraft(actorId, scope + 'members'); void reread('成员'); }, onUnauthorized, onIdentityMismatch, value => !!value && value.id === group.id && validRevision(value.policyRevision), scope + 'members');
 const menuOperation = useApplicationOperation<PolicyResult>(actorId, () => { clearScopedDraft(actorId, scope + 'menu'); void reread('菜单'); }, onUnauthorized, onIdentityMismatch, value => !!value && value.id === group.id && validRevision(value.policyRevision), scope + 'menu');
 useEffect(() => { if (basicDirty(basic)) keepScopedDraft(actorId, scope + 'basic', basic); else clearScopedDraft(actorId, scope + 'basic'); }, [actorId, scope, basic]);
 useEffect(() => { if (members && memberDirty(members)) keepScopedDraft(actorId, scope + 'members', members); else if (members) clearScopedDraft(actorId, scope + 'members'); }, [actorId, scope, members]);
 useEffect(() => { if (grants && grantDirty(grants)) keepScopedDraft(actorId, scope + 'menu', grants); else if (grants) clearScopedDraft(actorId, scope + 'menu'); }, [actorId, scope, grants]);
 const busy = [basicOperation, memberOperation, menuOperation].some(operation => operation.phase === 'preflight' || operation.phase === 'pending');
 const writeBlocked = busy || reloadRequired || [basicOperation, memberOperation, menuOperation].some(operation => operation.phase === 'unconfirmed');
 const dirtySections = useMemo(() => [basicDirty(basic) ? '基本信息' : '', members && memberDirty(members) ? '成员' : '', grants && grantDirty(grants) ? '菜单' : '', reloadRequired ? '已确认的配置待重读' : '', ...[basicOperation, memberOperation, menuOperation].filter(operation => operation.phase === 'unconfirmed').map(() => '请求结果待核查')].filter(Boolean), [basic, members, grants, reloadRequired, basicOperation.phase, memberOperation.phase, menuOperation.phase]);
 useEffect(() => { onDirty([...dirtySections, ...(busy ? ['请求正在提交'] : [])].join('、')); return () => onDirty(''); }, [dirtySections, busy, onDirty]);
 useEffect(() => {
  let live = true; const controller = new AbortController();
  if ([...candidateQuery.trim()].length > 100) { setCandidateError('账号前缀最多 100 个字符'); setCandidateLoading(false); return () => { live = false; controller.abort(); }; }
  setCandidateLoading(true); setCandidateError('');
  const params = new URLSearchParams({ q: candidateQuery.trim(), pageSize: '20' });
  if (pageToken) params.set('pageToken', pageToken);
  applicationApiEnvelope<CandidatePage>(actorId, 'applications/' + encodeURIComponent(appId) + '/member-candidates?' + params, 'GET', undefined, controller.signal).then(value => {
   if (!live) return;
   const pagination = value.meta?.pagination;
   if (!Array.isArray(value.data?.items) || !value.data.items.every(candidate => uuid.test(candidate.id) && candidate.status === 'active' && typeof candidate.label === 'string') || !pagination || typeof pagination.hasMore !== 'boolean' || !(pagination.nextPageToken === null || typeof pagination.nextPageToken === 'string')) throw new Error('成员候选响应无效，请重试');
   setCandidates(value.data.items); setHasMore(pagination.hasMore); setNextToken(pagination.nextPageToken);
  }).catch(cause => { if (live) { if (cause instanceof ApplicationError && cause.status === 401) onUnauthorized(); else if (cause instanceof ApplicationError && cause.code === 'AUTH_SESSION_CHANGED') onIdentityMismatch(); else setCandidateError(message(cause)); } }).finally(() => { if (live) setCandidateLoading(false); });
  return () => { live = false; controller.abort(); };
 }, [actorId, appId, candidateQuery, pageToken, candidateRetry, onUnauthorized, onIdentityMismatch]);
 const saveBasic = () => {
  if (writeBlocked) return;
  const name = basic.name.trim();
  if (!name || [...name].length > 100 || name.includes('\0')) { setError('权限组名称须为 1–100 个字符'); return; }
  setError(''); setStatus(''); basicOperation.start(base, { name, enabled: basic.enabled, expectedPolicyRevision: basic.revision }, 'PUT', 200);
 };
 const saveMembers = () => { if (!members || writeBlocked) return; setError(''); setStatus(''); memberOperation.start(base + '/members', { memberIds: members.ids, expectedPolicyRevision: members.revision }, 'PUT', 200); };
 const saveMenu = () => { if (!grants || grants.unsupported || writeBlocked) return; setError(''); setStatus(''); menuOperation.start(base + '/grants', { grants: grants.checked ? [rootGrant(appId)] : [], expectedPolicyRevision: grants.revision }, 'PUT', 200); };
 const operationControl = (operation: typeof basicOperation) => operation.phase === 'unconfirmed' ? <div className="app-operation-controls"><p role="alert">{operation.message}</p><button className="admin-button" onClick={operation.query}>核查原操作</button><button className="admin-button" onClick={operation.retry}>使用同一操作重试</button></div> : operation.message ? <p role="alert">{operation.message}</p> : null;
 return <>
  <div className="detail-heading"><h2>{group.name}</h2><button className="admin-button" onClick={() => void reread()} disabled={loading}>重新加载配置</button></div>
  {status && <p className="app-success" role="status">{status}</p>}
  {error && <p className="workspace-error" role="alert">{error}</p>}
  {loading && <p role="status">正在读取权限配置…</p>}
  <section className="config-section"><h3>基本信息</h3><label>权限组名称<input value={basic.name} disabled={busy || basicOperation.phase === 'unconfirmed'} onChange={event => setBasic(value => ({ ...value, name: event.target.value }))} /></label><label className="template-choice"><input type="checkbox" checked={basic.enabled} disabled={busy || basicOperation.phase === 'unconfirmed'} onChange={event => setBasic(value => ({ ...value, enabled: event.target.checked }))} />启用权限组</label><div><button className="admin-button primary" disabled={!basicDirty(basic) || writeBlocked || basicOperation.phase === 'unconfirmed'} onClick={saveBasic}>保存基本信息</button></div>{operationControl(basicOperation)}</section>
  <section className="config-section"><h3>成员</h3><p>仅当前应用 owner 可查询活跃成员。已停用的现有成员可以保留或移除。</p>
   {members ? <>
    {members.ids.map(id => { const member = members.labels[id]; return <div className="app-member-selected" key={id}><span><span>{member?.label || id}</span>{member?.status === 'disabled' && <small>（已停用）</small>}</span><button type="button" className="text-button" disabled={busy || memberOperation.phase === 'unconfirmed'} onClick={() => setMembers(value => value && ({ ...value, ids: value.ids.filter(valueId => valueId !== id) }))}>移除{member?.status === 'disabled' ? '停用成员' : '已选成员'}</button></div>; })}
    <label>按账号前缀搜索<input value={candidateQuery} onChange={event => { setCandidateQuery(event.target.value); setPageToken(''); setPreviousTokens([]); }} /></label>
    {candidateLoading ? <p role="status">正在加载成员候选…</p> : candidateError ? <div className="app-state"><p role="alert">{candidateError}</p><button className="admin-button" onClick={() => setCandidateRetry(value => value + 1)}>重试候选</button></div> : candidates.length ? candidates.map(candidate => <label className="template-choice" key={candidate.id}><input type="checkbox" checked={members.ids.includes(candidate.id)} disabled={busy || memberOperation.phase === 'unconfirmed'} onChange={event => setMembers(value => value && ({ ...value, ids: event.target.checked ? [...value.ids, candidate.id] : value.ids.filter(id => id !== candidate.id), labels: { ...value.labels, [candidate.id]: { ...candidate, selectable: true } } }))} />{candidate.label}</label>) : <p className="empty-state">没有匹配的活跃成员</p>}
    <div className="app-candidate-pages"><button className="admin-button" disabled={!previousTokens.length || candidateLoading} onClick={() => { const previous = [...previousTokens]; setPageToken(previous.pop() || ''); setPreviousTokens(previous); }}>上一页</button><button className="admin-button" disabled={!hasMore || !nextToken || candidateLoading} onClick={() => { setPreviousTokens(tokens => [...tokens, pageToken]); setPageToken(nextToken || ''); }}>下一页</button></div>
    <div><button className="admin-button primary" disabled={!memberDirty(members) || writeBlocked || memberOperation.phase === 'unconfirmed'} onClick={saveMembers}>保存成员</button></div>{operationControl(memberOperation)}
   </> : !loading && <p className="empty-state">成员配置暂不可用</p>}
  </section>
  <section className="config-section"><h3>菜单</h3><p>此阶段只配置当前应用根入口；成员身份本身不会授予菜单或数据权限。</p>{grants ? <>
   {grants.unsupported && <p className="workspace-error" role="alert">包含当前客户端不理解的授权，请使用新版客户端。此处为只读，避免替换时丢失权限。</p>}
   <label className="template-choice"><input type="checkbox" checked={grants.checked} disabled={busy || grants.unsupported || menuOperation.phase === 'unconfirmed'} onChange={event => setGrants(value => value && ({ ...value, checked: event.target.checked }))} />允许进入应用</label><div><button className="admin-button primary" disabled={!grantDirty(grants) || writeBlocked || grants.unsupported || menuOperation.phase === 'unconfirmed'} onClick={saveMenu}>保存菜单</button></div>{operationControl(menuOperation)}
  </> : !loading && <p className="empty-state">菜单配置暂不可用</p>}</section>
 </>;
}
