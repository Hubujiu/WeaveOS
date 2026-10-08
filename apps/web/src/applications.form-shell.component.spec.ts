import { expect, test, type Page } from '@playwright/test';

const actor = { id: '00000000-0000-4000-8000-000000000181', account: 'form-owner' };
const appId = '00000000-0000-4000-8000-000000000101';
const tableId = '00000000-0000-4000-8000-000000000102';
const viewId = '00000000-0000-4000-8000-000000000103';
const application = { id: appId, name: '请假应用', ownerUserId: actor.id, policyRevision: 1 };
const table = { id: tableId, appId, name: '请假申请', directoryId: null, position: 0, schemaVersion: 0, schemaReady: false };
const form = { id: viewId, appId, tableId, name: '请假申请', directoryId: null, position: 0, viewVersion: 0 };
const structure = { appId, structureVersion: 0, directories: [], tables: [table], forms: [form], capabilities: { canManageDefinition: true } };
const definition = { appId, table, form, fields: [], systemFields: [
 { id: 'id', kind: 'id', readOnly: true }, { id: 'createdBy', kind: 'member', readOnly: true },
 { id: 'createdAt', kind: 'datetime', readOnly: true }, { id: 'updatedAt', kind: 'datetime', readOnly: true },
 { id: 'recordVersion', kind: 'number', readOnly: true },
], layout: [], capabilities: { canManageDefinition: true } };
const ok = (data: unknown) => ({ code: 'OK', message: 'success', data, meta: { requestId: 'shell-test' } });
const preflight = { appId, tableId, viewId, schemaVersion: 0, viewVersion: 0, dataRevision: 0,
 dependencyRevision: 0, plan: { schemaChanges: [], metadataChanged: true, layoutChanged: true },
 impacts: [], dependencies: [], blockingIssues: [], saveAllowed: true, confirmation: null };

async function fixture(page: Page) {
 const writes: string[] = [];
 await page.route('**/api/v1/**', route => {
  const request = route.request();
  const path = new URL(request.url()).pathname.slice('/api/v1/'.length);
  if (request.method() !== 'GET') writes.push(path);
  const data = path === 'sessions/current' ? actor
   : path === 'me/access' ? { user: actor, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
   : path === 'applications' ? { items: [application] }
   : path === 'applications/' + appId ? application
   : path === 'applications/' + appId + '/access' ? { appId, canEnter: true, policyRevision: 1, menus: [{ resourceKind: 'application', resourceId: appId }] }
   : path === 'applications/' + appId + '/structure' ? structure
   : path === 'applications/' + appId + '/forms/' + viewId + '/definition' ? definition
   : null;
  if (data === null) return route.fulfill({ status: 404, json: { code: 'APPLICATION_NOT_FOUND', data: null } });
  return route.fulfill({ status: 200, json: ok(data) });
 });
 return { writes };
}

test('V030-012 Shell opens the real structure and form view under one app tab', async ({ page }) => {
 await fixture(page);
 await page.goto('/app/applications/' + appId);
 await expect(page.getByRole('region', { name: '目录与视图管理' })).toBeVisible();
 await expect(page.getByRole('button', { name: '配置表单 请假申请' })).toBeVisible();
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '/forms/' + viewId + '/design$'));
 await expect(page.getByRole('region', { name: '表单设计器' })).toBeVisible();
 await expect(page.getByRole('tab', { name: '当前表单' })).toHaveAttribute('aria-selected', 'true');
 await expect(page.getByRole('button', { name: '关闭应用：' + application.name })).toHaveCount(1);
});

test('V030-012 external Back discard removes a structure draft before returning', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications');
 await page.getByRole('main').getByRole('button', { name: application.name, exact: true }).click();
 await page.getByRole('button', { name: '新建目录' }).click();
 const dialog = page.getByRole('dialog', { name: '新建目录' });
 await dialog.getByRole('textbox', { name: '目录名称' }).fill('放弃的目录');
 await page.goBack();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toContainText('应用目录草稿将丢失');
 await guard.getByRole('button', { name: '放弃修改' }).click();
 await expect(page).toHaveURL(/\/app\/applications$/);
 await page.getByRole('main').getByRole('button', { name: application.name, exact: true }).click();
 await expect(page.getByRole('dialog', { name: '新建目录' })).toHaveCount(0);
 await page.getByRole('button', { name: '新建目录' }).click();
 await expect(page.getByRole('dialog', { name: '新建目录' }).getByRole('textbox', { name: '目录名称' })).toHaveValue('');
 expect(backend.writes).toEqual([]);
});

test('V030-012 external Back discard removes a designer draft before returning', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + appId);
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByLabel('字段名称').fill('放弃的字段');
 await page.goBack();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toContainText('表单设计器草稿将丢失');
 await guard.getByRole('button', { name: '放弃修改' }).click();
 await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '$'));
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 const canvas=page.getByRole('region',{name:'表单画布'});
 await expect(canvas.locator('.forms-canvas-node')).toHaveCount(0);
 await expect(canvas.getByRole('heading')).toHaveText(form.name);
 await expect(canvas).toHaveText(form.name+' 拖拽或用键盘添加字段，保存后才会生效 从左侧选择字段，或拖入这里开始设计表单',{useInnerText:true});
 expect(backend.writes).toEqual([]);
});

test('V030-012 closing the current app tab waits for form discard and clears only the unsent draft', async ({ page }) => {
 const backend = await fixture(page);
 await page.goto('/app/applications/' + appId);
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByLabel('字段名称').fill('关闭前字段');
 const close = page.getByRole('button', { name: '关闭应用：' + application.name });
 await close.click();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toContainText('表单设计器草稿将丢失');
 await guard.getByRole('button', { name: '继续编辑' }).click();
 await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '/forms/' + viewId + '/design$'));
 await expect(close).toHaveCount(1);
 await close.click();
 await guard.getByRole('button', { name: '放弃修改' }).click();
 await expect(page).toHaveURL(/\/app\/applications$/);
 await expect(close).toHaveCount(0);
 await page.getByRole('main').getByRole('button', { name: application.name, exact: true }).click();
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 const canvas=page.getByRole('region',{name:'表单画布'});
 await expect(canvas.locator('.forms-canvas-node')).toHaveCount(0);
 await expect(canvas.getByRole('heading')).toHaveText(form.name);
 await expect(canvas).toHaveText(form.name+' 拖拽或用键盘添加字段，保存后才会生效 从左侧选择字段，或拖入这里开始设计表单',{useInnerText:true});
 expect(backend.writes).toEqual([]);
});

test('V030-012 closing the current app tab retains a sent unknown form operation', async ({ page }) => {
 await fixture(page);
 const writes: Record<string, unknown>[] = [];
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition/preflight', route => route.fulfill({ json: ok(preflight) }));
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition', route => {
  if (route.request().method() === 'GET') return route.fulfill({ json: ok(definition) });
  writes.push(route.request().postDataJSON());
  return route.abort('failed');
 });
 await page.goto('/app/applications/' + appId);
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByRole('button', { name: '保存', exact: true }).click();
 await expect(page.getByRole('button', { name: '查询保存结果' })).toBeVisible();
 const close = page.getByRole('button', { name: '关闭应用：' + application.name });
 await close.click();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toContainText('已发送的操作会保留原请求');
 await expect(close).toHaveCount(1);
 await guard.getByRole('button', { name: '放弃修改' }).click();
 await expect(page).toHaveURL(/\/app\/applications$/);
 await expect(close).toHaveCount(0);
 await page.getByRole('main').getByRole('button', { name: application.name, exact: true }).click();
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await expect(page.getByRole('button', { name: '查询保存结果' })).toBeVisible();
 expect(writes).toHaveLength(1);
 expect(typeof writes[0].operationId).toBe('string');
});

test('V030-012 Shell cancels a slow unsent form preflight before route leave', async ({ page }) => {
 await fixture(page);
 let started!: () => void; const entered = new Promise<void>(resolve => { started = resolve; });
 let release!: () => void; const gate = new Promise<void>(resolve => { release = resolve; });
 let finish!: () => void; const finished = new Promise<void>(resolve => { finish = resolve; });
 let writes = 0;
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition/preflight', async route => {
  started(); await gate; await route.fulfill({ json: ok(preflight) }).catch(() => {}); finish();
 });
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition', route => {
  if (route.request().method() === 'GET') return route.fallback();
  if (route.request().method() === 'PUT') writes++;
  return route.fulfill({ status: 503, json: { code: 'COMMON_SERVICE_UNAVAILABLE', data: null } });
 });
 await page.goto('/app/applications/' + appId);
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByRole('button', { name: '保存', exact: true }).click();
 await entered;
 await page.goBack();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toContainText('尚未发送的预检将取消');
 await guard.getByRole('button', { name: '放弃修改' }).click();
 release();
 await finished;
 await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '$'));
 await page.waitForLoadState('networkidle');
 expect(writes).toBe(0);
});

test('V030-012 Shell re-confirms a changed preflight outcome and retains the original unknown write', async ({ page }) => {
 await fixture(page);
 let release!: () => void; const gate = new Promise<void>(resolve => { release = resolve; });
 let started!: () => void; const entered = new Promise<void>(resolve => { started = resolve; });
 const writes: Record<string, unknown>[] = [];
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition/preflight', async route => {
  started(); await gate; await route.fulfill({ json: ok(preflight) }).catch(() => {});
 });
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition', route => {
  if (route.request().method() === 'GET') return route.fulfill({ json: ok(definition) });
  writes.push(route.request().postDataJSON());
  return route.abort('failed');
 });
 await page.goto('/app/applications/' + appId);
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByRole('button', { name: '保存', exact: true }).click();
 await entered;
 await page.goBack();
 const guard = page.getByRole('dialog', { name: '有未保存的修改' });
 await expect(guard).toContainText('尚未发送的预检将取消');
 release();
 await expect(page.getByRole('button', { name: '查询保存结果' })).toBeVisible();
 await guard.getByRole('button', { name: '放弃修改' }).click();
 await expect(guard).toContainText('状态已变化');
 await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '/forms/' + viewId + '/design$'));
 await expect(guard).toContainText('已发送的操作会保留原请求');
 await guard.getByRole('button', { name: '放弃修改' }).click();
 await expect(page).toHaveURL(new RegExp('/app/applications/' + appId + '$'));
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await expect(page.getByRole('button', { name: '查询保存结果' })).toBeVisible();
 expect(writes).toHaveLength(1);
 expect(typeof writes[0].operationId).toBe('string');
});

test('V030-012 forced form 401 routes to login and restores only the same actor draft', async ({ page }) => {
 await fixture(page);
 let writes = 0;
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition/preflight', route => route.fulfill({ status: 401, json: { code: 'AUTH_UNAUTHENTICATED', data: null } }));
 await page.route('**/api/v1/applications/' + appId + '/forms/' + viewId + '/definition', route => {
  if (route.request().method() === 'PUT') writes++;
  return route.fallback();
 });
 await page.route('**/api/v1/sessions', route => route.fulfill({ status: 200, json: ok(actor) }));
 await page.goto('/app/applications/' + appId);
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await page.getByRole('button', { name: '文本', exact: true }).click();
 await page.getByLabel('字段名称').fill('复登保留字段');
 await page.getByRole('button', { name: '保存', exact: true }).click();
 await expect(page).toHaveURL(/\/login$/);
 expect(writes).toBe(0);
 await expect(page.getByRole('heading', { name: '登录', exact: true })).toBeVisible();
 const account = page.getByRole('textbox', { name: '账号', exact: true });
 await account.fill(actor.account);
 const password = page.getByLabel('密码', { exact: true });
 await password.fill('test-password');
 await expect(account).toHaveValue(actor.account);
 await expect(password).toHaveValue('test-password');
 await page.getByRole('button', { name: '登录' }).click();
 await expect(page).toHaveURL(/\/app$/);
 await page.getByRole('button', { name: '打开应用中心' }).click();
 await page.getByRole('main').getByRole('button', { name: application.name, exact: true }).click();
 await page.getByRole('button', { name: '配置表单 请假申请' }).click();
 await expect(page.getByRole('region', { name: '表单画布' })).toContainText('复登保留字段');
});

for (const width of [1280, 1920]) {
 test('Root R26 nested long form actions stay inside the tree and remain clickable at ' + width, async ({page}, info) => {
  await page.setViewportSize({width,height:1080});
  await fixture(page);
  const parent='00000000-0000-4000-8000-000000000191', child='00000000-0000-4000-8000-000000000192';
  const longName='审批视图 '+ 'A'.repeat(90);
  const nested={...structure,directories:[
   {id:parent,appId,name:'业务目录',parentId:null,position:0},
   {id:child,appId,name:'财务分组',parentId:parent,position:0},
  ],tables:[{...table,name:longName,directoryId:child}],forms:[{...form,name:longName,directoryId:child}]};
  await page.route('**/api/v1/applications/'+appId+'/structure',route=>route.fulfill({json:ok(nested)}));
  await page.goto('/app/applications/'+appId);
  const tree=page.locator('.forms-structure-tree');
  const row=tree.getByRole('treeitem',{name:longName,exact:true}).locator('.forms-tree-row');
  await expect(row).toBeVisible();
  const actions=row.getByRole('button');
  await expect(actions).toHaveCount(3);
  for(let i=0;i<3;i++){
   const action=actions.nth(i);
   await action.scrollIntoViewIfNeeded();
   await expect.poll(()=>action.evaluate(node=>{
    const panel=node.closest('.forms-structure-tree');
    if(!panel)return false;
    const a=node.getBoundingClientRect(),p=panel.getBoundingClientRect();
    const hit=document.elementFromPoint(a.x+a.width/2,a.y+a.height/2);
    return a.width>=24&&a.height>=24&&a.left>=p.left&&a.right<=p.right&&!!hit&&node.contains(hit);
   }),{message:'Every tree action must fit its panel and receive normal pointer input'}).toBe(true);
  }
  await page.screenshot({path:info.outputPath('nested-form-tree-'+width+'.png'),fullPage:true});
  await row.getByRole('button',{name:'配置表单 '+longName,exact:true}).click();
  await expect(page).toHaveURL(new RegExp('/forms/'+viewId+'/design$'));
  await expect(page.getByRole('region',{name:'表单设计器',exact:true})).toBeVisible();
 });
}
test('Root R26 money configuration exposes a unique rounding control and preserves its selected rule',async({page})=>{
 await fixture(page);
 await page.goto('/app/applications/'+appId+'/forms/'+viewId+'/design');
 await page.getByRole('button',{name:'金额',exact:true}).click();
 await page.getByLabel('字段名称',{exact:true}).fill('报销金额');
 await page.getByLabel('总精度',{exact:true}).fill('20');
 await page.getByLabel('小数位数',{exact:true}).fill('2');
 await page.getByLabel('处理位数',{exact:true}).fill('2');
 const rounding=page.getByRole('combobox',{name:/^舍入规则/});
 await expect(rounding).toHaveCount(1);
 await rounding.selectOption('HALF_EVEN');await expect(rounding).toHaveValue('HALF_EVEN');
 await rounding.selectOption('HALF_UP');await expect(rounding).toHaveValue('HALF_UP');
 await expect(page.getByLabel('总精度',{exact:true})).toHaveValue('20');
 await expect(page.getByLabel('处理位数',{exact:true})).toHaveValue('2');
});
