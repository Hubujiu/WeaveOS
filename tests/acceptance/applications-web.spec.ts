import { readFileSync } from 'node:fs';
import { test, expect, type Page } from '@playwright/test';

// This runs against the isolated HTTPS BFF, PostgreSQL and Redis. The fixture
// contains passwords; no login screenshots, trace, video or request logging.
test.use({ trace: 'off', screenshot: 'off', video: 'off' });
type Credentials = { account: string; password: string };
type Fixture = { admin: Credentials; adminId: string; user: Credentials; userId: string };
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
 await expect(page.getByText('尚未配置表单', { exact: true })).toBeVisible();
 await expect(page.getByRole('status').filter({ hasText: '应用已创建' })).toHaveCount(0);
 expect(accesses.length).toBeGreaterThan(0);
 if (info.project.name === 'chromium') await page.screenshot({ path: info.outputPath('workspace.png') });
 await page.getByRole('navigation', { name: '全局应用标签' }).getByRole('button', { name: '首页', exact: true }).click();
 const before = accesses.length;
 await page.getByRole('navigation', { name: '全局应用标签' }).getByRole('button', { name, exact: true }).click();
 await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
 expect(accesses.length).toBeGreaterThan(before);

 const memberContext = await browser.newContext({ baseURL: 'https://localhost:19443', ignoreHTTPSErrors: true, viewport: { width: 1920, height: 1080 } });
 try {
  const member = await memberContext.newPage();
  await login(member, f.user);
  await member.getByRole('button', { name: '打开应用中心', exact: true }).click();
  await expect(member.getByRole('button', { name, exact: true })).toHaveCount(0);
  await expect(member.getByText('暂无可用应用', { exact: true })).toBeVisible();
  if (info.project.name === 'chromium') await member.screenshot({ path: info.outputPath('catalog-empty.png') });
  await member.goto(`/app/applications/${app.id}`);
  await expect(member.getByRole('alert')).toContainText('没有应用访问或管理权限');
  if (info.project.name === 'chromium') await member.screenshot({ path: info.outputPath('access-denied.png') });
 } finally { await memberContext.close(); }
});
