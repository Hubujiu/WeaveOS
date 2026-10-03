# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: applications.component.spec.ts >> V030-012 live create capability opens a name-only real create flow
- Location: src/applications.component.spec.ts:60:1

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.click: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByRole('button', { name: '新建应用', exact: true })

```

# Page snapshot

```yaml
- generic [ref=e3]:
  - banner [ref=e4]:
    - generic [ref=e5]: WaveOS
    - button "账号" [ref=e8] [cursor=pointer]
  - main "主页" [ref=e9]:
    - generic [ref=e10]:
      - heading "暂无可用应用" [level=1] [ref=e11]
      - paragraph [ref=e12]: 获得应用访问权限后，将在这里显示。
```

# Test source

```ts
  1  | import { test, expect, type Page } from '@playwright/test';
  2  | 
  3  | // Independent oracle: PRD V030-012.2/.3, AP-FR-10, B5a.4-7 and
  4  | // original Home358:18205/catalog327:2132. Mock only the HTTP boundary.
  5  | // These component fixtures are not real API acceptance.
  6  | const user = { id: '00000000-0000-4000-8000-000000000001', account: 'slice-owner' };
  7  | const application = {
  8  |   id: '00000000-0000-4000-8000-000000000002',
  9  |   name: '业务应用甲', ownerUserId: user.id, policyRevision: 1,
  10 | };
  11 | async function fixture(page: Page, create = true) {
  12 |   await page.route('**/api/v1/**', async route => {
  13 |     const url = new URL(route.request().url());
  14 |     const path = url.pathname.slice('/api/v1/'.length);
  15 |     const data = path === 'sessions/current' ? user
  16 |       : path === 'me/access' ? {
  17 |           user, bootstrapAdmin: false, personnelManage: false, identities: [],
  18 |           permissions: create ? [{ code: 'applications.create', name: '应用管理', category: 'system', appId: null }] : [],
  19 |           applications: [], // Deliberately empty old personnel catalogue.
  20 |         }
  21 |       : path === 'applications' ? { items: [application] }
  22 |       : path === 'applications/' + application.id ? application
  23 |       : path === 'applications/' + application.id + '/access' ? {
  24 |           appId: application.id, canEnter: true, policyRevision: 1,
  25 |           menus: [{ resourceKind: 'application', resourceId: application.id }],
  26 |         }
  27 |       : {};
  28 |     await route.fulfill({ status: 200, json: { code: 'OK', message: 'success', data, meta: null } });
  29 |   });
  30 | }
  31 | 
  32 | test('V030-012 Home opens the real application catalogue', async ({ page }) => {
  33 |   await fixture(page);
  34 |   await page.goto('/app');
  35 |   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  36 |   await page.getByRole('button', { name: '打开应用中心', exact: true }).click();
  37 |   await expect(page).toHaveURL(/\/app\/applications$/);
  38 |   await expect(page.getByRole('heading', { name: '应用中心', exact: true })).toBeVisible();
  39 |   await expect(page.getByRole('button', { name: application.name, exact: true })).toBeVisible();
  40 | });
  41 | 
  42 | test('V030-012 catalogue reads B5 without parameters and filters its authorized result', async ({ page }) => {
  43 |   await fixture(page);
  44 |   const urls: URL[] = [];
  45 |   page.on('request', request => {
  46 |     const url = new URL(request.url());
  47 |     if (url.pathname === '/api/v1/applications') urls.push(url);
  48 |   });
  49 |   await page.goto('/app/applications');
  50 |   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  51 |   await expect(page.getByRole('heading', { name: '应用中心', exact: true })).toBeVisible();
  52 |   await expect(page.getByRole('button', { name: application.name, exact: true })).toBeVisible();
  53 |   expect(urls.length).toBeGreaterThan(0);
  54 |   expect(urls.every(url => url.search === '')).toBe(true);
  55 |   await page.getByRole('searchbox', { name: '搜索应用名称', exact: true }).fill('没有匹配项');
  56 |   await expect(page.getByRole('button', { name: application.name, exact: true })).toHaveCount(0);
  57 |   await expect(page.getByText('没有匹配的应用', { exact: true })).toBeVisible();
  58 | });
  59 | 
  60 | test('V030-012 live create capability opens a name-only real create flow', async ({ page }) => {
  61 |   await fixture(page);
  62 |   await page.goto('/app/applications');
  63 |   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
> 64 |   await page.getByRole('button', { name: '新建应用', exact: true }).click();
     |                                                                 ^ Error: locator.click: Test timeout of 30000ms exceeded.
  65 |   const dialog = page.getByRole('dialog', { name: '新建应用', exact: true });
  66 |   await expect(dialog).toBeVisible();
  67 |   await expect(dialog.getByRole('textbox', { name: '应用名称', exact: true })).toBeVisible();
  68 |   await expect(dialog.getByRole('button', { name: '创建应用', exact: true })).toBeVisible();
  69 | });
  70 | 
  71 | test('V030-012 opening an application rechecks access and presents its actual empty workspace', async ({ page }) => {
  72 |   await fixture(page, false);
  73 |   const accesses: string[] = [];
  74 |   page.on('request', request => {
  75 |     if (new URL(request.url()).pathname === '/api/v1/applications/' + application.id + '/access') accesses.push(request.url());
  76 |   });
  77 |   await page.goto('/app/applications/' + application.id);
  78 |   await expect(page.getByText('WaveOS', { exact: true })).toBeVisible();
  79 |   await expect(page.getByRole('heading', { name: application.name, exact: true })).toBeVisible();
  80 |   await expect(page.getByText('尚未配置表单', { exact: true })).toBeVisible();
  81 |   expect(accesses.length).toBeGreaterThan(0);
  82 | });
  83 | 
  84 | 
```