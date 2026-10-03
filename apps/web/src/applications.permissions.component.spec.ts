import { test, expect, type Page, type Route, type TestInfo } from '@playwright/test';

const actor = { id: '00000000-0000-4000-8000-000000000001', account: 'owner' };
const app = { id: '00000000-0000-4000-8000-000000000002', name: '审批应用', ownerUserId: actor.id, policyRevision: 1 };
const groupId = '00000000-0000-4000-8000-000000000003';
const memberId = '00000000-0000-4000-8000-000000000004';
const inactiveId = '00000000-0000-4000-8000-000000000005';
const rootGrant = { resourceKind: 'application', resourceId: app.id, action: 'menu.enter', rowScope: 'all', fields: [] };
const capture = async (page: Page, info: TestInfo, name: string) => { if (process.env.V030012_CAPTURE_SCREENSHOTS && info.project.name === 'chromium') await page.screenshot({ path: info.outputPath(name + '.png') }); };

async function fixture(page: Page, options: { owner?: boolean; groups?: boolean } = {}) {
 const writes: { path: string; method: string; body: any; status: number }[] = [];
 let revision = 1;
 let groups = options.groups === false ? [] : [{ id: groupId, name: '业务管理员', enabled: true, policyRevision: revision }];
 let memberIds = [inactiveId];
 let grants = [rootGrant];
 await page.route('**/api/v1/**', async (route: Route) => {
  const url = new URL(route.request().url());
  const path = url.pathname.slice('/api/v1/'.length);
  const method = route.request().method();
  const body = method === 'GET' ? null : route.request().postDataJSON();
  let status = 200; let data: any = {};
  let meta: any = null;
  if (method !== 'GET' && path.startsWith('applications/') && body?.expectedPolicyRevision !== revision) {
   writes.push({ path, method, body, status: 409 });
   await route.fulfill({ status: 409, json: { code: 'APPLICATION_POLICY_CONFLICT', message: 'changed', data: { currentPolicyRevision: revision } } });
   return;
  }
  if (path === 'sessions/current') data = actor;
  else if (path === 'me/access') data = { user: actor, bootstrapAdmin: false, personnelManage: false, identities: [], permissions: [], applications: [] };
  else if (path === 'applications') data = { items: [app] };
  else if (path === 'applications/' + app.id) data = { ...app, policyRevision: revision };
  else if (path === 'applications/' + app.id + '/access') data = { appId: app.id, canEnter: true, policyRevision: revision, menus: [{ resourceKind: 'application', resourceId: app.id }] };
  else if (path === 'applications/' + app.id + '/permission-groups') {
   if (method === 'GET') data = { items: groups, policyRevision: revision };
   else { status = 201; revision++; groups = [...groups, { id: groupId, name: body.name, enabled: true, policyRevision: revision }]; data = groups.at(-1); }
  } else if (path === 'applications/' + app.id + '/permission-groups/' + groupId) {
   revision++; groups = groups.map(g => ({ ...g, name: body.name, enabled: body.enabled, policyRevision: revision })); data = groups[0];
  } else if (path === 'applications/' + app.id + '/permission-groups/' + groupId + '/members') {
   if (method === 'GET') data = { memberIds, members: memberIds.map((id: string) => ({ id, label: id === inactiveId ? '停用成员' : '活跃成员', status: id === inactiveId ? 'disabled' : 'active', selectable: id !== inactiveId })), policyRevision: revision };
   else { memberIds = body.memberIds; revision++; groups = groups.map(group => ({ ...group, policyRevision: revision })); data = { id: groupId, policyRevision: revision }; }
  } else if (path === 'applications/' + app.id + '/permission-groups/' + groupId + '/grants') {
   if (method === 'GET') data = { grants, policyRevision: revision };
   else { grants = body.grants; revision++; groups = groups.map(group => ({ ...group, policyRevision: revision })); data = { id: groupId, policyRevision: revision }; }
  } else if (path.startsWith('applications/' + app.id + '/member-candidates')) {
   const q = url.searchParams.get('q') || '';
   data = { items: q === '找不到' ? [] : [{ id: memberId, label: '活跃成员', status: 'active' }] };
   meta = { pagination: { hasMore: false, nextPageToken: null } };
  }
  if (method !== 'GET') writes.push({ path, method, body, status });
  await route.fulfill({ status, json: { code: 'OK', message: 'success', data, meta } });
 });
 return {
  writes, state: () => ({ revision, groups, memberIds, grants }),
  externalBasic(name: string) { revision++; groups = groups.map(group => ({ ...group, name, policyRevision: revision })); },
  externalMenu(values: typeof rootGrant[]) { revision++; grants = values; groups = groups.map(group => ({ ...group, policyRevision: revision })); },
  externalRevision() { revision++; groups = groups.map(group => ({ ...group, policyRevision: revision })); },
 };
}

test('V030-012 owner manages a real group with three independent server saves', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理', exact: true }).click();
 await expect(page.getByRole('button', { name: '业务管理员', exact: true })).toBeVisible();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await expect(page.getByRole('textbox', { name: '权限组名称', exact: true })).toHaveValue('业务管理员');
 await expect(page.getByText('停用成员', { exact: true })).toBeVisible();
 await page.getByRole('textbox', { name: '权限组名称', exact: true }).fill('流程负责人');
 await page.getByRole('button', { name: '保存基本信息', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '基本信息已保存' })).toBeVisible();
 await page.getByRole('checkbox', { name: '活跃成员', exact: true }).check();
 await page.getByRole('button', { name: '保存成员', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '成员已保存' })).toBeVisible();
 await page.getByRole('checkbox', { name: '允许进入应用', exact: true }).uncheck();
 await page.getByRole('button', { name: '保存菜单', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '菜单已保存' })).toBeVisible();
 expect(backend.writes.map(w => w.method)).toEqual(['PUT', 'PUT', 'PUT']);
 expect(backend.writes.map(w => w.body.expectedPolicyRevision)).toEqual([1, 2, 3]);
 expect(backend.writes.map(w => w.body.operationId).every((id: string) => /^[0-9a-f-]{36}$/.test(id))).toBe(true);
 expect(backend.state().memberIds).toEqual([inactiveId, memberId]);
 expect(backend.state().grants).toEqual([]);
});

test('V030-012 a non-owner has no group-management entrance', async ({ page }, info) => {
 await fixture(page, { owner: false });
 await page.route('**/api/v1/applications/' + app.id, route => route.fulfill({ status: 200, json: { code: 'OK', data: { ...app, ownerUserId: '00000000-0000-4000-8000-000000000099' } } }));
 await page.goto('/app/applications/' + app.id);
 await expect(page.getByRole('heading', { name: app.name, exact: true })).toBeVisible();
 await expect(page.getByRole('button', { name: '权限管理', exact: true })).toHaveCount(0);
 await capture(page, info, 'permission-no-access');
});

test('V030-012 canceling a dirty current-tab close keeps the tab and its permission draft', async ({ page }) => {
 await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('textbox', { name: '权限组名称' }).fill('尚未保存的组名');
 await page.getByRole('button', { name: '关闭应用：' + app.name }).click();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toBeVisible();
 await guard.getByRole('button', { name: '继续编辑' }).click();
 await expect(page).toHaveURL(new RegExp('/app/applications/' + app.id + '$'));
 await expect(page.getByRole('button', { name: '关闭应用：' + app.name })).toBeVisible();
 await expect(page.getByRole('textbox', { name: '权限组名称' })).toHaveValue('尚未保存的组名');
});

test('V030-012 a confirmed group write with failed reread stays confirmed and blocks another save', async ({ page }) => {
 const backend = await fixture(page);
 let failRead = false;
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups/' + groupId, route => { if (route.request().method() === 'PUT') failRead = true; return route.fallback(); });
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups', route => {
  if (route.request().method() === 'GET' && failRead) return route.fulfill({ status: 503, json: { code: 'COMMON_SERVICE_UNAVAILABLE', data: null } });
  return route.fallback();
 });
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('textbox', { name: '权限组名称' }).fill('已确认但重读失败');
 await page.getByRole('button', { name: '保存基本信息' }).click();
 await expect(page.getByRole('status').filter({ hasText: '基本信息已保存，但重读失败' })).toBeVisible();
 await expect(page.getByRole('button', { name: '保存基本信息' })).toBeDisabled();
 expect(backend.writes).toHaveLength(1);
 failRead = false;
 await page.getByRole('button', { name: '重新加载配置' }).click();
 await expect(page.getByRole('textbox', { name: '权限组名称' })).toHaveValue('已确认但重读失败');
 expect(backend.writes).toHaveLength(1);
});

test('V030-012 creates an enabled group with the real POST and rereads it', async ({ page }) => {
 const backend = await fixture(page, { groups: false });
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理', exact: true }).click();
 await expect(page.getByText('暂无权限组', { exact: true })).toBeVisible();
 await page.getByRole('button', { name: '新建权限组', exact: true }).click();
 await page.getByRole('dialog', { name: '新建权限组' }).getByRole('textbox', { name: '权限组名称' }).fill('新组');
 await page.getByRole('dialog', { name: '新建权限组' }).getByRole('button', { name: '创建权限组' }).click();
 await expect(page.getByRole('status').filter({ hasText: '权限组已创建' })).toBeVisible();
 await expect(page.getByRole('button', { name: '新组', exact: true })).toBeVisible();
 expect(backend.writes).toHaveLength(1);
 expect(backend.writes[0]).toMatchObject({ method: 'POST', status: 201, body: { name: '新组', expectedPolicyRevision: 1 } });
});

test('V030-012 group names count Unicode characters and reject a 101-character write', async ({ page }) => {
 const backend = await fixture(page, { groups: false });
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '新建权限组' }).click();
 const dialog = page.getByRole('dialog', { name: '新建权限组' });
 await dialog.getByRole('textbox', { name: '权限组名称' }).fill('😀'.repeat(101));
 await dialog.getByRole('button', { name: '创建权限组' }).click();
 await expect(page.getByRole('alert').filter({ hasText: '1–100 个字符' })).toBeVisible();
 expect(backend.writes).toHaveLength(0);
 await dialog.getByRole('textbox', { name: '权限组名称' }).fill('😀'.repeat(100));
 await dialog.getByRole('button', { name: '创建权限组' }).click();
 await expect(page.getByRole('status').filter({ hasText: '权限组已创建' })).toBeVisible();
 expect(backend.writes[0].body.name).toBe('😀'.repeat(100));
});

test('V030-012 permission management remains keyboard reachable with reduced motion', async ({ page }) => {
 await fixture(page);
 await page.emulateMedia({ reducedMotion: 'reduce' });
 await page.goto('/app/applications/' + app.id);
 const entrance = page.getByRole('button', { name: '权限管理', exact: true });
 await entrance.focus(); await page.keyboard.press('Enter');
 await expect(page.getByRole('button', { name: '业务管理员', exact: true })).toBeVisible();
 const create = page.getByRole('button', { name: '新建权限组', exact: true });
 await create.focus(); await page.keyboard.press('Enter');
 const dialog = page.getByRole('dialog', { name: '新建权限组' });
 await expect(dialog.getByRole('textbox', { name: '权限组名称' })).toBeFocused();
 await dialog.getByRole('textbox', { name: '权限组名称' }).fill('键盘草稿');
 await page.keyboard.press('Escape');
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toBeVisible();
 await guard.getByRole('button', { name: '继续编辑' }).focus(); await page.keyboard.press('Enter');
 await expect(dialog.getByRole('textbox', { name: '权限组名称' })).toHaveValue('键盘草稿');
});

test('V030-012 an existing disabled member remains visible and can be explicitly removed', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await expect(page.getByText('停用成员', { exact: true })).toBeVisible();
 await page.getByRole('button', { name: '移除停用成员' }).click();
 await page.getByRole('button', { name: '保存成员' }).click();
 await expect(page.getByRole('status').filter({ hasText: '成员已保存' })).toBeVisible();
 expect(backend.writes[0].body.memberIds).toEqual([]);
 expect(backend.state().memberIds).toEqual([]);
});

test('V030-012 permission-group loading and failed read have visible retry states', async ({ page }, info) => {
 await fixture(page);
 let release!: () => void; const gate = new Promise<void>(resolve => { release = resolve; });
 let started!: () => void; const requestStarted = new Promise<void>(resolve => { started = resolve; });
 let fail = true;
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups', async route => {
  if (route.request().method() !== 'GET') return route.fallback();
  if (fail) { started(); await gate; return route.fulfill({ status: 503, json: { code: 'COMMON_SERVICE_UNAVAILABLE', data: null } }); }
  return route.fallback();
 });
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await requestStarted;
 await expect(page.getByRole('status').filter({ hasText: '正在加载权限组' })).toBeVisible();
 await capture(page, info, 'permission-loading');
 release();
 await expect(page.getByRole('alert').filter({ hasText: '服务暂时不可用' })).toBeVisible();
 await capture(page, info, 'permission-error');
 fail = false;
 await page.getByRole('button', { name: '重试', exact: true }).click();
 await expect(page.getByRole('button', { name: '业务管理员', exact: true })).toBeVisible();
});

test('V030-012 member-candidate empty and failure states remain retryable', async ({ page }, info) => {
 await fixture(page);
 let fail = true;
 await page.route('**/api/v1/applications/' + app.id + '/member-candidates?**', route => {
  if (fail) return route.fulfill({ status: 503, json: { code: 'COMMON_SERVICE_UNAVAILABLE', data: null } });
  return route.fallback();
 });
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await expect(page.getByRole('button', { name: '重试候选' })).toBeVisible();
 await page.getByRole('button', { name: '重试候选' }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-candidate-error');
 fail = false;
 await page.getByRole('button', { name: '重试候选' }).click();
 await expect(page.getByRole('checkbox', { name: '活跃成员' })).toBeVisible();
 await page.getByRole('textbox', { name: '按账号前缀搜索' }).fill('找不到');
 await expect(page.getByText('没有匹配的活跃成员', { exact: true })).toBeVisible();
 await page.getByText('没有匹配的活跃成员', { exact: true }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-candidate-empty');
});

test('V030-012 preserves an unsupported grant by making the menu read-only', async ({ page }, info) => {
 const backend = await fixture(page);
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups/' + groupId + '/grants', route => route.fulfill({ status: 200, json: { code: 'OK', data: { grants: [rootGrant, { resourceKind: 'table', resourceId: groupId, action: 'records.read', rowScope: 'all', fields: [] }], policyRevision: 1 } } }));
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await expect(page.getByText('包含当前客户端不理解的授权')).toBeVisible();
 await expect(page.getByRole('checkbox', { name: '允许进入应用' })).toBeDisabled();
 await expect(page.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 await page.getByRole('alert').filter({ hasText: '包含当前客户端不理解的授权' }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-unknown-grant');
 expect(backend.writes).toHaveLength(0);
});

test('V030-012 selected members survive cursor paging and a dirty menu requires review after member save', async ({ page }, info) => {
 const backend = await fixture(page);
 const secondId = '00000000-0000-4000-8000-000000000006';
 await page.route('**/api/v1/applications/' + app.id + '/member-candidates?**', route => {
  const url = new URL(route.request().url());
  const second = !!url.searchParams.get('pageToken');
  return route.fulfill({ status: 200, json: { code: 'OK', data: { items: [{ id: second ? secondId : memberId, label: second ? '第二页成员' : '活跃成员', status: 'active' }] }, meta: { pagination: { hasMore: !second, nextPageToken: second ? null : 'page-two' } } } });
 });
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('checkbox', { name: '活跃成员' }).check();
 await page.getByRole('button', { name: '下一页' }).click();
 await page.getByRole('checkbox', { name: '第二页成员' }).check();
 await capture(page, info, 'permission-cursor-selected');
 await page.getByRole('checkbox', { name: '允许进入应用' }).uncheck();
 await page.getByRole('button', { name: '保存成员' }).click();
 await expect(page.getByRole('status').filter({ hasText: '成员已保存' })).toBeVisible();
 expect(backend.state().memberIds).toEqual([inactiveId, memberId, secondId]);
 await expect(page.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 await expect(page.getByRole('group', { name: '菜单版本核对' })).toBeVisible();
 await page.getByRole('group', { name: '菜单版本核对' }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-conflict');
 expect(backend.writes).toHaveLength(1);
 expect(backend.state().grants).toEqual([rootGrant]);
});

test('V030-012 CAS review lets two dirty sections save in sequence only after explicit retry', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('checkbox', { name: '活跃成员' }).check();
 await page.getByRole('checkbox', { name: '允许进入应用' }).uncheck();
 await page.getByRole('button', { name: '保存成员' }).click();
 await expect(page.getByRole('status').filter({ hasText: '成员已保存' })).toBeVisible();
 const menu = page.getByRole('heading', { name: '菜单', exact: true }).locator('..');
 await expect(menu.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 await expect(menu).toContainText('服务器当前');
 await expect(menu).toContainText('本地草稿');
 await menu.getByRole('button', { name: '基于最新版本重试保存' }).click();
 await expect(page.getByRole('status').filter({ hasText: '菜单已保存' })).toBeVisible();
 expect(backend.writes.map(write => write.body.expectedPolicyRevision)).toEqual([1, 2]);
 expect(backend.state().grants).toEqual([]);
});

test('V030-012 CAS review keeps a same-section draft after another actor changes its server baseline', async ({ page }, info) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 const basic = page.getByRole('heading', { name: '基本信息', exact: true }).locator('..');
 await basic.getByRole('textbox', { name: '权限组名称' }).fill('我的草稿');
 backend.externalBasic('他人版本');
 await basic.getByRole('button', { name: '保存基本信息' }).click();
 await expect(basic).toContainText('服务器当前：他人版本');
 await expect(basic).toContainText('本地草稿：我的草稿');
 await expect(basic.getByRole('textbox', { name: '权限组名称' })).toHaveValue('我的草稿');
 await basic.getByRole('group', { name: '基本信息版本核对' }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-basic-conflict-review');
 await basic.getByRole('button', { name: '基于最新版本重试保存' }).click();
 await expect(page.getByRole('status').filter({ hasText: '基本信息已保存' })).toBeVisible();
 expect(backend.writes.map(write => write.status)).toEqual([409, 200]);
 expect(backend.writes.map(write => write.body.expectedPolicyRevision)).toEqual([1, 2]);
 expect(backend.writes[1].body.operationId).not.toBe(backend.writes[0].body.operationId);
 expect(backend.state().groups[0].name).toBe('我的草稿');
});

test('V030-012 CAS review discards only the selected section and retains a dirty peer', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('textbox', { name: '权限组名称' }).fill('保留的名称');
 await page.getByRole('checkbox', { name: '允许进入应用' }).uncheck();
 backend.externalMenu([]);
 await page.getByRole('button', { name: '保存菜单' }).click();
 const menu = page.getByRole('heading', { name: '菜单', exact: true }).locator('..');
 await expect(menu).toContainText('服务器当前');
 await menu.getByRole('button', { name: '放弃本段修改并重载' }).click();
 await expect(menu.getByRole('checkbox', { name: '允许进入应用' })).not.toBeChecked();
 await expect(page.getByRole('textbox', { name: '权限组名称' })).toHaveValue('保留的名称');
 expect(backend.writes).toHaveLength(1);
});

test('V030-012 CAS review handles a second definite conflict without losing input or reusing a key', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 const basic = page.getByRole('heading', { name: '基本信息', exact: true }).locator('..');
 await basic.getByRole('textbox', { name: '权限组名称' }).fill('持久草稿');
 backend.externalBasic('外部一');
 await basic.getByRole('button', { name: '保存基本信息' }).click();
 await expect(basic).toContainText('服务器当前：外部一');
 backend.externalBasic('外部二');
 await basic.getByRole('button', { name: '基于最新版本重试保存' }).click();
 await expect(basic).toContainText('服务器当前：外部二');
 await expect(basic.getByRole('textbox', { name: '权限组名称' })).toHaveValue('持久草稿');
 await basic.getByRole('button', { name: '基于最新版本重试保存' }).click();
 await expect(page.getByRole('status').filter({ hasText: '基本信息已保存' })).toBeVisible();
 expect(backend.writes.map(write => write.status)).toEqual([409, 409, 200]);
 expect(new Set(backend.writes.map(write => write.body.operationId)).size).toBe(3);
});

test('V030-012 CAS review rejects a split policy snapshot before any grant write', async ({ page }, info) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 const menu = page.getByRole('heading', { name: '菜单', exact: true }).locator('..');
 await menu.getByRole('checkbox', { name: '允许进入应用' }).uncheck();
 backend.externalRevision();
 let skewOnce = true;
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups/' + groupId + '/members', route => {
  if (!skewOnce || route.request().method() !== 'GET') return route.fallback();
  skewOnce = false;
  return route.fulfill({ status: 200, json: { code: 'OK', data: { memberIds: [inactiveId], members: [{ id: inactiveId, label: '停用成员', status: 'disabled', selectable: false }], policyRevision: 1 } } });
 });
 await page.getByRole('button', { name: '重新加载配置' }).click();
 await expect(page.getByRole('alert').filter({ hasText: '权限配置读取期间发生变化' })).toBeVisible();
 await expect(menu.getByRole('checkbox', { name: '允许进入应用' })).not.toBeChecked();
 await expect(menu.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 expect(backend.writes).toHaveLength(0);
 await capture(page, info, 'permission-split-snapshot-blocked');
 await page.getByRole('button', { name: '重新加载配置' }).click();
 await expect(menu).toContainText('服务器当前：允许进入应用');
 await expect(menu).toContainText('本地草稿：不允许进入应用');
 await expect(menu.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 expect(backend.writes).toHaveLength(0);
 await menu.getByRole('button', { name: '基于最新版本重试保存' }).click();
 await expect(page.getByRole('status').filter({ hasText: '菜单已保存' })).toBeVisible();
 expect(backend.writes).toHaveLength(1);
 expect(backend.writes[0].body.expectedPolicyRevision).toBe(2);
});

test('V030-012 CAS review rejects a stale group item against its list revision', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 const menu = page.getByRole('heading', { name: '菜单', exact: true }).locator('..');
 await menu.getByRole('checkbox', { name: '允许进入应用' }).uncheck();
 backend.externalRevision();
 let staleOnce = true;
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups', route => {
  if (!staleOnce || route.request().method() !== 'GET') return route.fallback();
  staleOnce = false;
  return route.fulfill({ status: 200, json: { code: 'OK', data: {
   policyRevision: 2,
   items: [{ id: groupId, name: '业务管理员', enabled: true, policyRevision: 1 }],
  } } });
 });
 await page.getByRole('button', { name: '重新加载配置' }).click();
 await expect(page.getByRole('alert').filter({ hasText: '权限配置读取期间发生变化' })).toBeVisible();
 await expect(menu.getByRole('checkbox', { name: '允许进入应用' })).not.toBeChecked();
 await expect(menu.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 expect(backend.writes).toHaveLength(0);
 await page.getByRole('button', { name: '重新加载配置' }).click();
 await expect(menu.getByRole('group', { name: '菜单版本核对' })).toBeVisible();
 expect(backend.writes).toHaveLength(0);
});

test('V030-012 Back discard cancels a gated group preflight before PUT', async ({ page }, info) => {
 const backend = await fixture(page);
 await page.goto('/app/applications');
 await page.getByRole('main').getByRole('button', { name: app.name, exact: true }).click();
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('textbox', { name: '权限组名称' }).fill('放弃的编辑');
 let entered!: () => void; const started = new Promise<void>(resolve => { entered = resolve; });
 let release!: () => void; const gate = new Promise<void>(resolve => { release = resolve; });
 let completed!: () => void; const preflightCompleted = new Promise<void>(resolve => { completed = resolve; });
 await page.route('**/api/v1/sessions/current', async route => { entered(); await gate; await route.fulfill({ status: 200, json: { code: 'OK', data: actor } }).catch(() => {}); completed(); });
 await page.getByRole('button', { name: '保存基本信息' }).click();
 await started;
 await expect(page.getByRole('textbox', { name: '权限组名称' })).toBeDisabled();
 await page.goBack();
 await expect(page.getByRole('dialog', { name: '有未保存的修改' })).toContainText('尚未发送的请求会取消');
 await capture(page, info, 'permission-unsent-discard');
 await page.getByRole('dialog', { name: '有未保存的修改' }).getByRole('button', { name: '放弃权限组更改并保留待核查操作' }).click();
 release();
 await expect(page).toHaveURL(/\/app\/applications$/);
 await preflightCompleted;
 expect(backend.writes).toHaveLength(0);
});

test('V030-012 an unconfirmed group PUT keeps its original actor, body, key and editable scope', async ({ page }, info) => {
 const backend = await fixture(page);
 const writes: any[] = [];
 const queries: string[] = [];
 await page.route('**/api/v1/applications/' + app.id + '/permission-groups/' + groupId, route => {
  writes.push({ body: route.request().postDataJSON(), actor: route.request().headers()['x-expected-actor-id'] });
  if (writes.length === 1) return route.fulfill({ status: 503, json: { code: 'APPLICATION_OPERATION_UNCONFIRMED', data: { operationId: writes[0].body.operationId } } });
  return route.fulfill({ status: 200, json: { code: 'OK', data: { id: groupId, name: '保留的组名', enabled: true, policyRevision: 2 } } });
 });
 await page.route('**/api/v1/application-operations/*', route => { queries.push(new URL(route.request().url()).pathname.split('/').at(-1)!); return route.fulfill({ status: 404, json: { code: 'APPLICATION_NOT_FOUND', data: null } }); });
 await page.goto('/app/applications');
 await page.getByRole('main').getByRole('button', { name: app.name, exact: true }).click();
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('textbox', { name: '权限组名称' }).fill('保留的组名');
 await page.getByRole('button', { name: '保存基本信息' }).click();
 await expect(page.getByRole('button', { name: '核查原操作' })).toBeVisible();
 await page.getByRole('button', { name: '核查原操作' }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-unconfirmed');
 backend.externalRevision();
 await page.getByRole('button', { name: '重新加载配置' }).click();
 await expect(page.getByRole('button', { name: '核查原操作' })).toBeVisible();
 await expect(page.getByRole('group', { name: '基本信息版本核对' })).toHaveCount(0);
 expect(writes).toHaveLength(1);
 await page.getByRole('checkbox', { name: '允许进入应用' }).uncheck();
 await expect(page.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 await page.getByRole('navigation', { name: '全局应用标签' }).getByRole('button', { name: '全部应用' }).click();
 await page.getByRole('dialog', { name: '有未保存的修改' }).getByRole('button', { name: '放弃权限组更改并保留待核查操作' }).click();
 await page.getByRole('main').getByRole('button', { name: app.name, exact: true }).click();
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await expect(page.getByRole('textbox', { name: '权限组名称' })).toHaveValue('保留的组名');
 await page.getByRole('button', { name: '核查原操作' }).scrollIntoViewIfNeeded();
 await capture(page, info, 'permission-recovery');
 await page.getByRole('button', { name: '核查原操作' }).click();
 await expect(page.getByRole('button', { name: '使用同一操作重试' })).toBeVisible();
 await page.getByRole('button', { name: '使用同一操作重试' }).click();
 await expect(page.getByRole('status').filter({ hasText: '基本信息已保存' })).toBeVisible();
 expect(queries).toEqual([writes[0].body.operationId]);
 expect(writes).toHaveLength(2);
 expect(writes[1]).toEqual(writes[0]);
 expect(writes[0].actor).toBe(actor.id);
});

test('V030-012 group preflight 401 preserves the unsent scoped draft for the same actor', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await page.getByRole('textbox', { name: '权限组名称' }).fill('登录后保留的组名');
 let reauthenticated = false;
 await page.route('**/api/v1/sessions', route => { reauthenticated = true; return route.fulfill({ status: 200, json: { code: 'OK', data: actor } }); });
 await page.route('**/api/v1/sessions/current', route => route.fulfill(reauthenticated ? { status: 200, json: { code: 'OK', data: actor } } : { status: 401, json: { code: 'AUTH_UNAUTHENTICATED', data: null } }));
 await page.getByRole('button', { name: '保存基本信息' }).click();
 await expect(page).toHaveURL(/\/login$/);
 expect(backend.writes).toHaveLength(0);
 await expect(page.getByRole('heading', { name: '登录', exact: true })).toBeVisible();
 const account = page.getByRole('textbox', { name: '账号', exact: true });
 await account.fill(actor.account);
 await expect(account).toHaveValue(actor.account);
 const password = page.getByLabel('密码', { exact: true });
 await password.fill('test-password');
 await expect(password).toHaveValue('test-password');
 await expect(account).toHaveValue(actor.account);
 await page.getByRole('button', { name: '登录' }).click();
 await expect(page).toHaveURL(/\/app$/);
 await page.getByRole('button', { name: '打开应用中心' }).click();
 await page.getByRole('main').getByRole('button', { name: app.name, exact: true }).click();
 await page.getByRole('button', { name: '权限管理' }).click();
 await page.getByRole('button', { name: '业务管理员', exact: true }).click();
 await expect(page.getByRole('textbox', { name: '权限组名称' })).toHaveValue('登录后保留的组名');
 expect(backend.writes).toHaveLength(0);
});
