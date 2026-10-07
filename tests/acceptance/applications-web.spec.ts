import { readFileSync } from 'node:fs';
import { test, expect, type BrowserContext, type Page } from '@playwright/test';

// This runs against the isolated HTTPS BFF, PostgreSQL and Redis. The fixture
// contains passwords; no login screenshots, trace, video or request logging.
test.use({ trace: 'off', screenshot: 'off', video: 'off' });
type Credentials = { account: string; password: string };
type Fixture = { admin: Credentials; adminId: string; user: Credentials; userId: string; resetTarget: Credentials & { id: string } };
function fixture(): Fixture {
 const file = process.env.WEAVEOS_ACCEPTANCE_FIXTURES;
 if (!file) throw new Error('Isolated acceptance fixture path required');
 return JSON.parse(readFileSync(file, 'utf8'));
}
async function login(page: Page, credentials: Credentials) {
 await page.goto('/login');
 await page.getByLabel('账号', { exact: true }).fill(credentials.account);
 await page.getByLabel('密码', { exact: true }).fill(credentials.password);
 await page.getByRole('button', { name: '登录', exact: true }).click();
 await expect(page).toHaveURL(/\/app(?:\/|$)/);
 await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
}

async function realApi(context: BrowserContext, actorId: string, path: string, method = 'GET', data?: object) {
 const csrf = (await context.cookies()).find(cookie => cookie.name === '__Host-csrf')?.value;
 const headers: Record<string, string> = { Origin: 'https://localhost:19443', 'X-Expected-Actor-Id': actorId };
 if (method !== 'GET') { expect(csrf).toBeTruthy(); headers['X-CSRF-Token'] = csrf!; }
 const response = await context.request.fetch('/api/v1/' + path, { method, headers, data });
 const envelope = await response.json();
 return { status: response.status(), code: envelope.code as string, data: envelope.data as Record<string, unknown> };
}

// Each permission-mutating browser case owns fresh real users. Never reuse the
// password-reset target or grant access to the baseline unprivileged fixture.
async function isolatedMember(context: BrowserContext, adminId: string): Promise<Credentials & { id: string }> {
 const account = 'app-case-' + crypto.randomUUID();
 const password = 'Synthetic@123';
 const invitation = await realApi(context, adminId, 'invitations', 'POST', {});
 expect(invitation.status).toBe(201);
 expect(typeof invitation.data.invitationCode).toBe('string');
 const registered = await realApi(context, adminId, 'registrations', 'POST', {
  account, password, invitationCode: invitation.data.invitationCode,
 });
 expect(registered.status).toBe(201);
 expect(registered.data.account).toBe(account);
 expect(registered.data.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
 return { account, password, id: registered.data.id as string };
}

test('V030-012 real B5: root creates, reopens and member without a grant is denied', async ({ page, browser }, info) => {
 const f = fixture();
 await page.setViewportSize({ width: 1920, height: 1080 });
 await login(page, f.admin);
 await expect(page.getByRole('heading', { name: '主页', exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('home.png') });

 await page.getByRole('button', { name: '打开应用中心', exact: true }).click();
 await expect(page.getByRole('heading', { name: '应用中心', exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('catalog.png') });
 await page.getByRole('button', { name: '新建应用', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: '新建应用', exact: true });
 await expect(dialog).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('create.png') });
 const name = `真实应用 ${info.project.name} ${Date.now()}`;
 await dialog.getByRole('textbox', { name: '应用名称', exact: true }).fill(name);
 const response = page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/api/v1/applications');
 await dialog.getByRole('button', { name: '创建应用', exact: true }).click();
 const written = await response;
 expect(written.status()).toBe(201);
 expect(written.request().headers()['x-expected-actor-id']).toBe(f.adminId);
 const packet = written.request().postDataJSON();
 expect(Object.keys(packet).sort()).toEqual(['name', 'operationId']);
 expect(packet.name).toBe(name);
 const body = await written.json();
 const app = body.data as { id: string; name: string; ownerUserId: string; policyRevision: number };
 expect(app.name).toBe(name); expect(app.ownerUserId).toBe(f.adminId); expect(app.policyRevision).toBe(1);
 await expect(dialog).toHaveCount(0);
 await expect(page.getByRole('status').filter({ hasText: '应用已创建' })).toBeVisible();
 await expect(page.getByRole('main').getByRole('button', { name, exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('created.png') });

 const accesses: string[] = [];
 page.on('request', req => { if (new URL(req.url()).pathname === `/api/v1/applications/${app.id}/access`) accesses.push(req.url()); });
 await page.getByRole('main').getByRole('button', { name, exact: true }).click();
 await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
 await expect(page.getByRole('region', { name: '目录与视图管理' })).toBeVisible();
 await expect(page.getByRole('status').filter({ hasText: '暂无目录或表单' })).toBeVisible();
 await expect(page.getByRole('status').filter({ hasText: '应用已创建' })).toHaveCount(0);
 expect(accesses.length).toBeGreaterThan(0);
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('workspace.png') });
 await page.getByRole('navigation', { name: '全局应用标签' }).getByRole('button', { name: '首页', exact: true }).click();
 await expect(page).toHaveURL(/\/app$/);
 await expect(page.getByRole('heading', { name: '主页', exact: true })).toBeVisible();
 const before = accesses.length;
 await page.getByRole('navigation', { name: '全局应用标签' }).getByRole('button', { name, exact: true }).click();
 await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
 await expect.poll(() => accesses.length).toBeGreaterThan(before);

 const memberContext = await browser.newContext({ baseURL: 'https://localhost:19443', ignoreHTTPSErrors: true, viewport: { width: 1920, height: 1080 } });
 try {
  const member = await memberContext.newPage();
  await login(member, f.user);
  await member.getByRole('button', { name: '打开应用中心', exact: true }).click();
  await expect(member.getByRole('button', { name, exact: true })).toHaveCount(0);
  // The shared isolated member may retain grants to apps created by other cases;
  // this new app must remain absent until its own grant is written.
  if (info.project.name === 'chromium') await member.screenshot({ path: info.outputPath('catalog-member.png') });
  await member.goto(`/app/applications/${app.id}`);
  await expect(member.getByRole('alert')).toContainText('没有应用访问或管理权限');
  if (info.project.name === 'chromium') await member.screenshot({ path: info.outputPath('access-denied.png') });
 } finally { await memberContext.close(); }
});

test('V030-012 real B5: owner saves permission-group basics, members and root menu independently', async ({ page, browser }, info) => {
 const f = fixture();
 await page.setViewportSize({ width: 1920, height: 1080 });
 await login(page, f.admin);
 const ownMember = await isolatedMember(page.context(), f.adminId);
 const appResult = await realApi(page.context(), f.adminId, 'applications', 'POST', { name: '真实权限界面 ' + info.project.name + ' ' + Date.now(), operationId: crypto.randomUUID() });
 expect(appResult.status).toBe(201);
 const app = appResult.data as { id: string; name: string; policyRevision: number };
 await page.goto('/app/applications/' + app.id);
 await expect(page.getByRole('heading', { name: app.name, exact: true })).toBeVisible();
 await page.getByRole('button', { name: '权限管理', exact: true }).click();
 await expect(page.getByText('暂无权限组', { exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-empty.png') });
 await page.getByRole('button', { name: '新建权限组', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: '新建权限组', exact: true });
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-create.png') });
 await dialog.getByRole('textbox', { name: '权限组名称', exact: true }).fill('真实权限组');
 const created = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/permission-groups'));
 await dialog.getByRole('button', { name: '创建权限组', exact: true }).click();
 expect((await created).status()).toBe(201);
 await expect(page.getByRole('button', { name: '真实权限组', exact: true })).toBeVisible();
 await page.getByRole('button', { name: '真实权限组', exact: true }).click();
 await expect(page.getByRole('checkbox', { name: '允许进入应用', exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-detail.png') });
 await page.getByRole('textbox', { name: '权限组名称', exact: true }).fill('真实权限组更新');
 await page.getByRole('button', { name: '保存基本信息', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '基本信息已保存' })).toBeVisible();
 await page.getByRole('textbox', { name: '按账号前缀搜索', exact: true }).fill(ownMember.account);
 await expect(page.getByRole('checkbox', { name: ownMember.account, exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-candidate.png') });
 await page.getByRole('checkbox', { name: ownMember.account, exact: true }).check();
 await page.getByRole('button', { name: '保存成员', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '成员已保存' })).toBeVisible();
 const groups = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups');
 const group = (groups.data.items as { id: string; name: string }[]).find(value => value.name === '真实权限组更新');
 expect(group).toBeTruthy();
 const members = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups/' + group!.id + '/members');
 expect(members.data.memberIds).toContain(ownMember.id);
 const memberContext = await browser.newContext({ baseURL: 'https://localhost:19443', ignoreHTTPSErrors: true });
 try {
  const memberPage = await memberContext.newPage();
  await login(memberPage, ownMember);
  const beforeGrant = await realApi(memberContext, ownMember.id, 'applications/' + app.id + '/access');
  expect(beforeGrant.status).toBe(403);
  await page.getByRole('checkbox', { name: '允许进入应用', exact: true }).check();
  if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-menu-dirty.png') });
  await page.getByRole('button', { name: '保存菜单', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: '菜单已保存' })).toBeVisible();
  if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-menu-saved.png') });
  const grant = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups/' + group!.id + '/grants');
  expect(grant.data.grants).toEqual([{ resourceKind: 'application', resourceId: app.id, action: 'menu.enter', rowScope: 'all', fields: [] }]);
  const afterGrant = await realApi(memberContext, ownMember.id, 'applications/' + app.id + '/access');
  expect(afterGrant.status).toBe(200);
 } finally { await memberContext.close(); }
});

test('V030-012 real B5: concurrent policy change requires explicit menu review before grant write', async ({ page }, info) => {
 const f = fixture();
 await login(page, f.admin);
 const created = await realApi(page.context(), f.adminId, 'applications', 'POST', { name: '并发授权核对 ' + info.project.name + ' ' + Date.now(), operationId: crypto.randomUUID() });
 expect(created.status).toBe(201);
 const app = created.data as { id: string; name: string };
 const groupCreated = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups', 'POST', {
  name: '初始权限组', expectedPolicyRevision: 1, operationId: crypto.randomUUID(),
 });
 expect(groupCreated.status).toBe(201);
 const group = groupCreated.data as { id: string; policyRevision: number };
 await page.goto('/app/applications/' + app.id);
 await page.getByRole('button', { name: '权限管理', exact: true }).click();
 await page.getByRole('button', { name: '初始权限组', exact: true }).click();
 const menu = page.getByRole('heading', { name: '菜单', exact: true }).locator('..');
 await menu.getByRole('checkbox', { name: '允许进入应用' }).check();
 const peer = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups/' + group.id, 'PUT', {
  name: '其他客户端已更新', enabled: true, expectedPolicyRevision: group.policyRevision, operationId: crypto.randomUUID(),
 });
 expect(peer.status).toBe(200);
 const stale = page.waitForResponse(response => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/grants'));
 await menu.getByRole('button', { name: '保存菜单' }).click();
 expect((await stale).status()).toBe(409);
 await expect(menu.getByRole('group', { name: '菜单版本核对' })).toContainText('服务器当前：不允许进入应用');
 await expect(menu.getByRole('group', { name: '菜单版本核对' })).toContainText('本地草稿：允许进入应用');
 await expect(menu.getByRole('button', { name: '保存菜单' })).toBeDisabled();
 const before = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups/' + group.id + '/grants');
 expect(before.status).toBe(200);
 expect(before.data.grants).toEqual([]);
 await menu.getByRole('group', { name: '菜单版本核对' }).scrollIntoViewIfNeeded();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('permission-real-conflict-review.png'), fullPage: true });
 const retried = page.waitForResponse(response => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/grants'));
 await menu.getByRole('button', { name: '基于最新版本重试保存' }).click();
 expect((await retried).status()).toBe(200);
 await expect(page.getByRole('status').filter({ hasText: '菜单已保存' })).toBeVisible();
 const after = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/permission-groups/' + group.id + '/grants');
 expect(after.data.grants).toEqual([{ resourceKind: 'application', resourceId: app.id, action: 'menu.enter', rowScope: 'all', fields: [] }]);
});

test('V030-012 real HTTPS Shell persists a directory, form and saved designer field', async ({ page }, info) => {
 const f = fixture();
 await page.setViewportSize({ width: 1920, height: 1080 });
 await login(page, f.admin);
 const created = await realApi(page.context(), f.adminId, 'applications', 'POST', {
  name: '真实表单壳 ' + info.project.name + ' ' + Date.now(), operationId: crypto.randomUUID(),
 });
 expect(created.status).toBe(201);
 const app = created.data as { id: string; name: string };
 const formName = '审批视图 ' + info.project.name + ' ' + Date.now();
 const folderName = '业务目录 ' + info.project.name + ' ' + Date.now();
 const writes: { path: string; actor: string | undefined; method: string }[] = [];
 page.on('request', request => {
  const path = new URL(request.url()).pathname;
  if (path.startsWith('/api/v1/applications/' + app.id) && request.method() !== 'GET')
   writes.push({ path, actor: request.headers()['x-expected-actor-id'], method: request.method() });
 });
 const initialStructure = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/structure');
 expect(initialStructure.status, initialStructure.code).toBe(200);
 await page.goto('/app/applications/' + app.id);
 await expect(page.getByRole('region', { name: '目录与视图管理' })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('form-structure-empty.png'), fullPage: true });
 await page.getByRole('button', { name: '新建目录' }).click();
 await page.getByLabel('目录名称').fill(folderName);
 const directoryWrite = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/directories'));
 await page.getByRole('button', { name: '创建目录', exact: true }).click();
 const directoryResponse = await directoryWrite;
 const directoryReceipt=await directoryResponse.json();
 expect(directoryResponse.status(),directoryReceipt.code).toBe(201);
 expect(directoryReceipt.data.id).toEqual(expect.any(String));
 await expect(page.getByRole('treeitem', { name: folderName, exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('form-directory-created.png'), fullPage: true });
 await page.getByRole('treeitem', { name: folderName, exact: true }).getByRole('button').first().click();
 await page.getByRole('button', { name: '新建表单' }).click();
 await page.getByLabel('表单名称').fill(formName);
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('form-create-dialog.png'), fullPage: true });
 await page.getByRole('button', { name: '创建表单', exact: true }).click();
 await expect(page.getByRole('treeitem', { name: formName, exact: true })).toBeVisible();
 const savedStructure = await realApi(page.context(), f.adminId, 'applications/' + app.id + '/structure');
 expect(savedStructure.status).toBe(200);
 const forms = savedStructure.data.forms as { id: string; name: string; directoryId: string | null }[];
 const form = forms.find(value => value.name === formName);
 expect(form?.directoryId).toBe(directoryReceipt.data.id);
 await page.getByRole('button', { name: '配置表单 ' + formName }).click();
 await expect(page).toHaveURL(new RegExp('/app/applications/' + app.id + '/forms/' + form!.id + '/design$'));
 await expect(page.getByRole('region', { name: '表单设计器' })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('form-designer-empty.png'), fullPage: true });
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByLabel('字段名称').fill('申请事项');
 await page.getByRole('button', { name: '保存', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '已保存' })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('form-designer-saved.png'), fullPage: true });
 await page.reload();
 await expect(page.getByRole('region', { name: '表单画布' })).toContainText('申请事项');
 expect(writes.some(write => write.path.endsWith('/directories') && write.method === 'POST')).toBe(true);
 expect(writes.some(write => write.path.endsWith('/forms') && write.method === 'POST')).toBe(true);
 expect(writes.some(write => write.path.endsWith('/definition/preflight') && write.method === 'POST')).toBe(true);
 expect(writes.some(write => write.path.endsWith('/definition') && write.method === 'PUT')).toBe(true);
 expect(writes.every(write => write.actor === f.adminId)).toBe(true);
});

test('V030-012 real B5: create-only owner, same-app menu member and cross-app denial', async ({ page, browser }, info) => {
 const f = fixture();
 const suffix = `${info.project.name}-${Date.now()}`;
 await login(page, f.admin);
 const admin = page.context();
 const ownCreator = await isolatedMember(admin, f.adminId);
 const ownMember = await isolatedMember(admin, f.adminId);
 const create = async (name: string) => {
  const result = await realApi(admin, f.adminId, 'applications', 'POST', { name, operationId: crypto.randomUUID() });
  expect(result.status).toBe(201); expect(result.code).toBe('OK');
  return result.data as { id: string; name: string; ownerUserId: string; policyRevision: number };
 };
 const appA = await create('授权应用 ' + suffix);
 const appB = await create('隔离应用 ' + suffix);
 expect(appA.ownerUserId).toBe(f.adminId); expect(appB.ownerUserId).toBe(f.adminId);

 const identity = await realApi(admin, f.adminId, 'personnel/identities', 'POST', {
  name: '应用创建者 ' + suffix, description: '', templateIds: [], permissionCodes: ['applications.create'],
 });
 expect(identity.status).toBe(201);
 const identityId = identity.data.id as string;
 const search = await realApi(admin, f.adminId, 'personnel/members/search', 'POST', { page: 1, pageSize: 20, search: ownCreator.account });
 expect(search.status).toBe(200);
 const member = await realApi(admin, f.adminId, 'personnel/members/' + ownCreator.id);
 expect(member.status).toBe(200);
 const assigned = await realApi(admin, f.adminId, 'personnel/members/' + ownCreator.id + '/identities', 'PUT', {
  identityIds: [identityId], version: member.data.version, queryVersion: search.data.queryVersion,
 });
 expect(assigned.status).toBe(200);

 const base = { baseURL: 'https://localhost:19443', ignoreHTTPSErrors: true, viewport: { width: 1920, height: 1080 } };
 const creatorContext = await browser.newContext(base);
 const memberContext = await browser.newContext(base);
 try {
  const creator = await creatorContext.newPage();
  await login(creator, ownCreator);
  await expect(creator.getByRole('button', { name: '设置', exact: true })).toBeHidden();
  await creator.getByRole('button', { name: '打开应用中心', exact: true }).click();
  await expect(creator.getByRole('button', { name: '新建应用', exact: true })).toBeVisible();
  await expect(creator.getByRole('button', { name: appA.name, exact: true })).toHaveCount(0);
  await creator.getByRole('button', { name: '新建应用', exact: true }).click();
  const dialog = creator.getByRole('dialog', { name: '新建应用', exact: true });
  const ownName = '创建者自有应用 ' + suffix;
  await dialog.getByRole('textbox', { name: '应用名称', exact: true }).fill(ownName);
  const created = creator.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/v1/applications');
  await dialog.getByRole('button', { name: '创建应用', exact: true }).click();
  const ownResponse = await created;
  expect(ownResponse.status()).toBe(201);
  const ownApp = (await ownResponse.json()).data as { id: string; ownerUserId: string };
  expect(ownApp.ownerUserId).toBe(ownCreator.id);
  expect((await realApi(creatorContext, ownCreator.id, 'applications/' + ownApp.id + '/permission-groups')).status).toBe(200);
  expect((await realApi(creatorContext, ownCreator.id, 'applications/' + appA.id + '/permission-groups')).status).toBe(403);

  const group = await realApi(admin, f.adminId, 'applications/' + appA.id + '/permission-groups', 'POST', {
   name: '入口成员 ' + suffix, operationId: crypto.randomUUID(), expectedPolicyRevision: 1,
  });
  expect(group.status).toBe(201);
  const groupId = group.data.id as string;
  const members = await realApi(admin, f.adminId, 'applications/' + appA.id + '/permission-groups/' + groupId + '/members', 'PUT', {
   memberIds: [ownMember.id], operationId: crypto.randomUUID(), expectedPolicyRevision: group.data.policyRevision,
  });
  expect(members.status).toBe(200);
  const grant = { resourceKind: 'application', resourceId: appA.id, action: 'menu.enter', rowScope: 'all', fields: [] };
  const grants = await realApi(admin, f.adminId, 'applications/' + appA.id + '/permission-groups/' + groupId + '/grants', 'PUT', {
   grants: [grant], operationId: crypto.randomUUID(), expectedPolicyRevision: members.data.policyRevision,
  });
  expect(grants.status).toBe(200);

  const memberPage = await memberContext.newPage();
  await login(memberPage, ownMember);
  await expect(memberPage.getByRole('button', { name: '设置', exact: true })).toBeHidden();
  await memberPage.getByRole('button', { name: '打开应用中心', exact: true }).click();
  await expect(memberPage.getByRole('main').getByRole('button', { name: appA.name, exact: true })).toBeVisible();
  await expect(memberPage.getByRole('main').getByRole('button', { name: appB.name, exact: true })).toHaveCount(0);
  await memberPage.getByRole('main').getByRole('button', { name: appA.name, exact: true }).click();
  await expect(memberPage.getByRole('heading', { name: appA.name, exact: true })).toBeVisible();
  expect((await realApi(memberContext, ownMember.id, 'applications/' + appA.id + '/permission-groups')).status).toBe(403);
  await memberPage.goto('/app/applications/' + appB.id);
  await expect(memberPage.getByRole('alert')).toContainText('没有应用访问或管理权限');

  const revoked = await realApi(admin, f.adminId, 'applications/' + appA.id + '/permission-groups/' + groupId + '/grants', 'PUT', {
   grants: [], operationId: crypto.randomUUID(), expectedPolicyRevision: grants.data.policyRevision,
  });
  expect(revoked.status).toBe(200);
  await memberPage.goto('/app/applications');
  await memberPage.reload();
  await expect(memberPage.getByRole('main').getByRole('button', { name: appA.name, exact: true })).toHaveCount(0);
  await memberPage.goto('/app/applications/' + appA.id);
  await expect(memberPage.getByRole('alert')).toContainText('没有应用访问或管理权限');
 } finally { await creatorContext.close(); await memberContext.close(); }
});

test('V030-012 real B5: expected actor guard rejects cross-session reads, writes and operation lookup', async ({ page, browser }) => {
 const f = fixture();
 await login(page, f.admin);
 const admin = page.context();
 const operationId = crypto.randomUUID();
 const created = await realApi(admin, f.adminId, 'applications', 'POST', { name: 'guard验证 ' + Date.now(), operationId });
 expect(created.status).toBe(201);
 const appId = created.data.id as string;
 const memberContext = await browser.newContext({ baseURL: 'https://localhost:19443', ignoreHTTPSErrors: true });
 try {
  const member = await memberContext.newPage();
  await login(member, f.user);
  const ordinary = await realApi(memberContext, f.userId, 'applications');
  expect(ordinary.status).toBe(200);
  const wrongRead = await realApi(memberContext, f.adminId, 'applications');
  expect(wrongRead.status).toBe(409); expect(wrongRead.code).toBe('AUTH_SESSION_CHANGED');
  expect(JSON.stringify(wrongRead.data)).not.toContain(appId);
  const wrongWrite = await realApi(memberContext, f.adminId, 'applications', 'POST', { name: '不得创建', operationId: crypto.randomUUID() });
  expect(wrongWrite.status).toBe(409); expect(wrongWrite.code).toBe('AUTH_SESSION_CHANGED');
  const wrongOperation = await realApi(memberContext, f.adminId, 'application-operations/' + operationId);
  expect(wrongOperation.status).toBe(409); expect(wrongOperation.code).toBe('AUTH_SESSION_CHANGED');
  const malformed = await memberContext.request.get('/api/v1/applications', { headers: { 'X-Expected-Actor-Id': 'not-a-uuid' } });
  expect(malformed.status()).toBe(400); expect((await malformed.json()).code).toBe('COMMON_VALIDATION_FAILED');
  const legacy = await memberContext.request.get('/api/v1/applications');
  expect(legacy.status()).toBe(200);
  const after = await realApi(memberContext, f.userId, 'applications');
  expect(after.status).toBe(200); expect(after.data).toEqual(ordinary.data);
 } finally { await memberContext.close(); }
});


test('Root real HTTPS full journey creates an application, saves a form, and creates and edits a persisted record',async({page},info)=>{
 test.setTimeout(120_000);
 const f=fixture();await page.setViewportSize({width:1920,height:1080});await login(page,f.admin);
 const appName='记录闭环 '+info.project.name+' '+Date.now(),formName='报销单 '+Date.now();
 await page.getByRole('button',{name:'打开应用中心',exact:true}).click();
 await page.getByRole('button',{name:'新建应用',exact:true}).click();
 const createApp=page.getByRole('dialog',{name:'新建应用',exact:true});await createApp.getByLabel('应用名称',{exact:true}).fill(appName);
 const appResponse=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/api/v1/applications');
 await createApp.getByRole('button',{name:'创建应用',exact:true}).click();const appWritten=await appResponse;expect(appWritten.status()).toBe(201);
 const app=(await appWritten.json()).data as {id:string};await expect(createApp).toHaveCount(0);
 await page.getByRole('main').getByRole('button',{name:appName,exact:true}).click();
 const shell=await page.getByTestId('application-shell').elementHandle();
 await page.getByRole('button',{name:'新建表单',exact:true}).click();await page.getByLabel('表单名称',{exact:true}).fill(formName);
 const formResponse=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/api/v1/applications/'+app.id+'/forms');
 await page.getByRole('button',{name:'创建表单',exact:true}).click();const formWritten=await formResponse;expect(formWritten.status()).toBe(201);
 const form=(await formWritten.json()).data.form as {id:string};await expect(page.getByRole('treeitem',{name:formName,exact:true})).toBeVisible();
 await page.getByRole('button',{name:'配置表单 '+formName,exact:true}).click();
 await expect(page.getByRole('region',{name:'表单设计器'})).toBeVisible();
 await page.getByRole('button',{name:'金额',exact:true}).click();await page.getByLabel('字段名称',{exact:true}).fill('报销金额');
 await page.getByLabel('总精度',{exact:true}).fill('20');await page.getByLabel('小数位数',{exact:true}).fill('2');await page.getByLabel('处理位数',{exact:true}).fill('2');const rounding=page.getByRole('combobox',{name:/^舍入规则/});await expect(rounding).toHaveCount(1);await rounding.selectOption('HALF_UP');await expect(rounding).toHaveValue('HALF_UP');
 await page.getByRole('button',{name:'文本',exact:true}).click();await page.getByLabel('字段名称',{exact:true}).fill('报销事由');
 const definitionResponse=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname==='/api/v1/applications/'+app.id+'/forms/'+form.id+'/definition');
 await page.getByRole('button',{name:'保存',exact:true}).click();expect((await definitionResponse).status()).toBe(200);
 await expect(page.getByRole('status').filter({hasText:'已保存'})).toBeVisible();
 await page.getByRole('button',{name:'返回工作台',exact:true}).click();
 await page.getByRole('button',{name:'打开表单 '+formName,exact:true}).click();await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();
 if(info.project.name==='chromium')await page.screenshot({path:info.outputPath('record-real-empty-list.png'),fullPage:true});
 const prefix='applications/'+app.id+'/forms/'+form.id;
 const runtimeReply=await realApi(page.context(),f.adminId,prefix+'/runtime');expect(runtimeReply.status).toBe(200);
 const fields=runtimeReply.data.fields as {id:string;name:string}[];const amount=fields.find(x=>x.name==='报销金额')!.id,title=fields.find(x=>x.name==='报销事由')!.id;
 await page.getByRole('button',{name:'新建记录',exact:true}).click();const createRecord=page.getByRole('dialog',{name:'新建记录',exact:true});
 await createRecord.getByLabel('报销金额',{exact:true}).fill('12.345');await createRecord.getByLabel('报销事由',{exact:true}).fill('真实差旅');
 if(info.project.name==='chromium')await page.screenshot({path:info.outputPath('record-real-create-input.png'),fullPage:true});
 const recordResponse=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/api/v1/'+prefix+'/records');
 await createRecord.getByRole('button',{name:'保存记录',exact:true}).click();const recordWritten=await recordResponse;expect(recordWritten.status()).toBe(201);
 const receipt=(await recordWritten.json()).data as {id:string;recordVersion:number};
 await expect(page.getByRole('status').filter({hasText:'记录已保存'})).toBeVisible();
 const detail=page.getByRole('dialog',{name:'记录详情',exact:true});await expect(detail.getByLabel('报销金额',{exact:true})).toHaveValue('12.35');
 await expect(detail.getByLabel('报销事由',{exact:true})).toHaveValue('真实差旅');
 const persisted=await realApi(page.context(),f.adminId,prefix+'/records/'+receipt.id);expect(persisted.status).toBe(200);
 expect(persisted.data.values).toMatchObject({[amount]:'12.35',[title]:'真实差旅'});expect(persisted.data.recordVersion).toBe(receipt.recordVersion);
 if(info.project.name==='chromium')await page.screenshot({path:info.outputPath('record-real-confirmed-detail.png'),fullPage:true});
 await detail.getByRole('button',{name:'编辑记录',exact:true}).click();await detail.getByLabel('报销金额',{exact:true}).fill('20.10');
 const editedResponse=page.waitForResponse(r=>r.request().method()==='PATCH'&&new URL(r.url()).pathname==='/api/v1/'+prefix+'/records/'+receipt.id);
 await detail.getByRole('button',{name:'保存记录',exact:true}).click();const edited=await editedResponse;expect(edited.status()).toBe(200);
 const editBody=edited.request().postDataJSON();expect(editBody.expectedRecordVersion).toBe(receipt.recordVersion);expect(editBody.changes).toEqual({[amount]:'20.10'});
 await expect(detail.getByLabel('报销金额',{exact:true})).not.toBeEditable();await expect(detail.getByLabel('报销金额',{exact:true})).toHaveValue('20.10');
 if(info.project.name==='chromium')await page.screenshot({path:info.outputPath('record-real-edited-detail.png'),fullPage:true});
 expect(await shell!.evaluate(node=>node.isConnected)).toBe(true);
 await page.reload();await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();
 await page.getByRole('button',{name:'打开记录：报销金额 20.10',exact:true}).click();
 await expect(page.getByRole('dialog',{name:'记录详情',exact:true}).getByLabel('报销事由',{exact:true})).toHaveValue('真实差旅');
});
