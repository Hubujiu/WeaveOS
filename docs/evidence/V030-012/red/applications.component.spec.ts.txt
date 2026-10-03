import { test, expect, type Page } from '@playwright/test';

// Independent oracle: PRD V030-012.2/.3, AP-FR-10, B5a.4-7 and
// original Home358:18205/catalog327:2132. Mock only the HTTP boundary.
// These component fixtures are not real API acceptance.
const user = { id: '00000000-0000-4000-8000-000000000001', account: 'slice-owner' };
const application = {
  id: '00000000-0000-4000-8000-000000000002',
  name: '业务应用甲', ownerUserId: user.id, policyRevision: 1,
};
async function fixture(page: Page, create = true) {
  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.slice('/api/v1/'.length);
    const data = path === 'sessions/current' ? user
      : path === 'me/access' ? {
          user, bootstrapAdmin: false, personnelManage: false, identities: [],
          permissions: create ? [{ code: 'applications.create', name: '应用管理', category: 'system', appId: null }] : [],
          applications: [], // Deliberately empty old personnel catalogue.
        }
      : path === 'applications' ? { items: [application] }
      : path === 'applications/' + application.id ? application
      : path === 'applications/' + application.id + '/access' ? {
          appId: application.id, canEnter: true, policyRevision: 1,
          menus: [{ resourceKind: 'application', resourceId: application.id }],
        }
      : {};
    await route.fulfill({ status: 200, json: { code: 'OK', message: 'success', data, meta: null } });
  });
}

test('V030-012 Home opens the real application catalogue', async ({ page }) => {
  await fixture(page);
  await page.goto('/app');
  await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '打开应用中心', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/applications$/);
  await expect(page.getByRole('heading', { name: '应用中心', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: application.name, exact: true })).toBeVisible();
});

test('V030-012 catalogue reads B5 without parameters and filters its authorized result', async ({ page }) => {
  await fixture(page);
  const urls: URL[] = [];
  page.on('request', request => {
    const url = new URL(request.url());
    if (url.pathname === '/api/v1/applications') urls.push(url);
  });
  await page.goto('/app/applications');
  await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: '应用中心', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: application.name, exact: true })).toBeVisible();
  expect(urls.length).toBeGreaterThan(0);
  expect(urls.every(url => url.search === '')).toBe(true);
  await page.getByRole('searchbox', { name: '搜索应用名称', exact: true }).fill('没有匹配项');
  await expect(page.getByRole('button', { name: application.name, exact: true })).toHaveCount(0);
  await expect(page.getByText('没有匹配的应用', { exact: true })).toBeVisible();
});

test('V030-012 live create capability opens a name-only real create flow', async ({ page }) => {
  await fixture(page);
  await page.goto('/app/applications');
  await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '新建应用', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '新建应用', exact: true });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('textbox', { name: '应用名称', exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: '创建应用', exact: true })).toBeVisible();
});

test('V030-012 opening an application rechecks access and presents its actual empty workspace', async ({ page }) => {
  await fixture(page, false);
  const accesses: string[] = [];
  page.on('request', request => {
    if (new URL(request.url()).pathname === '/api/v1/applications/' + application.id + '/access') accesses.push(request.url());
  });
  await page.goto('/app/applications/' + application.id);
  await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: application.name, exact: true })).toBeVisible();
  await expect(page.getByText('尚未配置表单', { exact: true })).toBeVisible();
  expect(accesses.length).toBeGreaterThan(0);
});

