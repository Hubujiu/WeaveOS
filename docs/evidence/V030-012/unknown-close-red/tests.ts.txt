import { test, expect, type Page } from '@playwright/test';

const user = { id: '00000000-0000-4000-8000-000000000101', account: 'states-owner' };
const app = { id: '00000000-0000-4000-8000-000000000102', name: '业务应用乙', ownerUserId: user.id, policyRevision: 1 };
async function fixture(page: Page) {
 await page.route('**/api/v1/**', async route => {
  const path = new URL(route.request().url()).pathname.replace('/api/v1/', '');
  const data = path === 'sessions/current' ? user
   : path === 'me/access' ? { user, bootstrapAdmin: true, personnelManage: false, identities: [], permissions: [], applications: [] }
   : path === 'applications' ? { items: [app] }
   : path === `applications/${app.id}` ? app
   : path === `applications/${app.id}/access` ? { appId: app.id, canEnter: true, policyRevision: 1, menus: [{ resourceKind: 'application', resourceId: app.id }] }
   : {};
  await route.fulfill({ json: { code: 'OK', data } });
 });
}

test('V030-012 catalog error, loading, retry and empty states remain honest', async ({ page }, info) => {
 await fixture(page);
 let failure = true;
 let release!: () => void;
 const gate = new Promise<void>(resolve => { release = resolve; });
 await page.route('**/api/v1/applications', async route => {
  if (failure) await route.fulfill({ status: 503, json: { code: 'COMMON_SERVICE_UNAVAILABLE' } });
  else { await gate; await route.fulfill({ json: { code: 'OK', data: { items: [] } } }); }
 });
 await page.goto('/app/applications');
 await expect(page.getByRole('alert')).toContainText('服务暂时不可用');
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('catalog-error.png') });
 failure = false;
 await page.getByRole('button', { name: '重试', exact: true }).click();
 await expect(page.getByRole('status').filter({ hasText: '正在加载应用' })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('catalog-loading.png') });
 release();
 await expect(page.getByText('暂无可用应用', { exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('catalog-empty.png') });
});

test('V030-012 stale app access is denied and a fresh retry reauthorizes it', async ({ page }, info) => {
 await fixture(page);
 let denied = true;
 await page.route(`**/api/v1/applications/${app.id}/access`, route => route.fulfill(denied
  ? { status: 403, json: { code: 'APPLICATION_FORBIDDEN' } }
  : { json: { code: 'OK', data: { appId: app.id, canEnter: true, policyRevision: 1, menus: [{ resourceKind: 'application', resourceId: app.id }] } } }));
 await page.goto(`/app/applications/${app.id}`);
 await expect(page.getByRole('alert')).toContainText('没有应用访问或管理权限');
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('access-denied.png') });
 denied = false;
 await page.getByRole('button', { name: '重试', exact: true }).click();
 await expect(page.getByText('尚未配置表单', { exact: true })).toBeVisible();
});

test('V030-012 catalog 401 sends the user to login', async ({ page }) => {
 await fixture(page);
 await page.route('**/api/v1/applications', route => route.fulfill({ status: 401, json: { code: 'COMMON_UNAUTHENTICATED' } }));
 await page.goto('/app/applications');
 await expect(page).toHaveURL(/\/login$/);
 await expect(page.getByRole('heading', { name: '登录', exact: true })).toBeVisible();
});

test('V030-012 unknown create and dirty-close states show the retained operation', async ({ page }, info) => {
 await fixture(page);
 await page.route('**/api/v1/applications', route => route.fulfill(route.request().method() === 'GET'
  ? { json: { code: 'OK', data: { items: [] } } }
  : { status: 503, json: { code: 'APPLICATION_OPERATION_UNCONFIRMED' } }));
 await page.goto('/app/applications');
 await page.getByRole('button', { name: '新建应用', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: '新建应用', exact: true });
 await dialog.getByRole('textbox', { name: '应用名称', exact: true }).fill('未确认的应用');
 await dialog.getByRole('button', { name: '创建应用', exact: true }).click();
 await expect(dialog.getByRole('alert')).toContainText('操作结果尚未确认');
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('create-unconfirmed.png') });
 await dialog.getByRole('button', { name: '关闭', exact: true }).click();
 const guard = page.getByRole('dialog', { name: '有未保存的修改', exact: true });
 await expect(guard).toBeVisible();
 await expect(guard.getByRole('button', { name: '关闭并保留待核查操作', exact: true })).toBeVisible();
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('create-dirty-close.png') });
 await guard.getByRole('button', { name: '继续编辑', exact: true }).click();
 await expect(dialog.getByRole('button', { name: '核查操作', exact: true })).toBeVisible();
 await dialog.getByRole('button', { name: '关闭', exact: true }).click();
 await guard.getByRole('button', { name: '关闭并保留待核查操作', exact: true }).click();
 await expect(dialog).toHaveCount(0);
 await page.getByRole('button', { name: '新建应用', exact: true }).click();
 await expect(dialog.getByRole('textbox', { name: '应用名称', exact: true })).toHaveValue('未确认的应用');
 await expect(dialog.getByRole('button', { name: '核查操作', exact: true })).toBeVisible();
});
