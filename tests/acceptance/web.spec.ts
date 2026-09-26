import { readFileSync } from 'node:fs';
import { test, expect } from '@playwright/test';

// PRD-defined visible behavior; proposed /login, /register, /app route bindings are reviewed in V010-002/006.
function fixtures(): { user: { account: string; password: string }; uiInvitations: Record<string, string> } {
  const file = process.env.WEAVEOS_ACCEPTANCE_FIXTURES;
  if (!file) throw new Error('BLOCKED: generate isolated acceptance fixtures in V010-003');
  return JSON.parse(readFileSync(file, 'utf8'));
}
test('FR-001/011: login is accessible and offers no self-service recovery', async ({ page }) => {
  await page.goto('/login');
  await expect(page.getByRole('heading', { name: '登录', exact: true })).toBeVisible();
  await expect(page.getByLabel('账号', { exact: true })).toBeVisible();
  await expect(page.getByLabel('密码', { exact: true })).toHaveAttribute('type', 'password');
  await expect(page.getByRole('button', { name: '登录', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: /忘记密码|找回密码/ })).toHaveCount(0);
});
test('PRD interaction: password visibility can be toggled', async ({ page }) => {
  await page.goto('/login');
  const input = page.getByLabel('密码', { exact: true });
  await input.fill('Synthetic@123');
  await page.getByRole('button', { name: '显示密码', exact: true }).click();
  await expect(input).toHaveAttribute('type', 'text');
  await page.getByRole('button', { name: '隐藏密码', exact: true }).click();
  await expect(input).toHaveAttribute('type', 'password');
});
test('FR-004: invitation URL fills the registration field', async ({ page }) => {
  await page.goto('/register?invitationCode=synthetic-prefill-value');
  await expect(page.getByLabel('邀请码', { exact: true })).toHaveValue('synthetic-prefill-value');
});

test('FR-002: registration rejects an account containing spaces before submission', async ({ page }) => {
  let registrations = 0;
  page.on('request', request => { if (request.url().endsWith('/api/v1/registrations')) registrations += 1; });
  await page.goto('/register?invitationCode=synthetic-prefill-value');
  await page.getByLabel('账号', { exact: true }).fill('Alice Smith');
  await page.getByLabel('密码', { exact: true }).fill('A@1a');
  await page.getByLabel('确认密码', { exact: true }).fill('A@1a');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('空格');
  expect(registrations).toBe(0);
});

test('FR-018: four required character classes suffice without a length hint', async ({ page }) => {
  await page.goto('/register?invitationCode=synthetic-prefill-value');
  await expect(page.getByText('至少 10 位')).toHaveCount(0);
  await page.getByLabel('密码', { exact: true }).fill('A@1a');
  await expect(page.getByRole('progressbar', { name: '密码强度' })).toBeVisible();
});
test('FR-018: registration exposes accessible password-strength feedback', async ({ page }) => {
  await page.goto('/register');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await expect(page.getByRole('progressbar', { name: '密码强度' })).toBeVisible();
});
test('FR-007: an anonymous browser cannot enter the protected app', async ({ page }) => {
  await page.goto('/app');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('FR-001: wrong credentials show an error and remain unauthenticated', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-nonexistent-user');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('FR-017: successful registration returns to login without a session', async ({ page, context }, info) => {
  const f = fixtures();
  const invitation = f.uiInvitations[info.project.name];
  expect(invitation).toBeTruthy();
  await page.goto(`/register?invitationCode=${encodeURIComponent(invitation)}`);
  await page.getByLabel('账号', { exact: true }).fill(`ui-${info.project.name}-${Date.now()}`);
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await page.getByLabel('确认密码', { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
  expect((await context.cookies()).filter(cookie => cookie.httpOnly)).toHaveLength(0);
  await page.goto('/app');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('FR-005/008/009: real login survives reload, then logout revokes it', async ({ page, context }) => {
  const f = fixtures();
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill(f.user.account);
  await page.getByLabel('密码', { exact: true }).fill(f.user.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page).toHaveURL(/\/app(?:\/|\?|$)/);
  await page.reload();
  await expect(page.getByText(f.user.account, { exact: true })).toBeVisible();
  const cookies = await context.cookies();
  const session = cookies.find(cookie => cookie.name === '__Host-session');
  const csrf = cookies.find(cookie => cookie.name === '__Host-csrf');
  expect(Boolean(session && session.httpOnly && session.secure && session.sameSite === 'Lax' && session.path === '/')).toBe(true);
  expect(Boolean(csrf && !csrf.httpOnly && csrf.secure && csrf.sameSite === 'Lax' && csrf.path === '/')).toBe(true);
  const authCookies = cookies.filter(cookie => cookie.httpOnly);
  expect(authCookies.length).toBeGreaterThan(0);
  for (const cookie of authCookies) {
    expect(cookie.secure).toBe(true);
    expect(['Lax', 'Strict']).toContain(cookie.sameSite);
    const exposed = await page.evaluate(value => document.cookie.includes(value) || JSON.stringify(localStorage).includes(value) || JSON.stringify(sessionStorage).includes(value), cookie.value);
    expect(exposed).toBe(false);
  }
  const logoutRequest = page.waitForRequest(request => request.method() === 'DELETE' && request.url().endsWith('/api/v1/sessions/current'));
  await page.getByRole('button', { name: '退出登录', exact: true }).click();
  expect((await logoutRequest).headers()['x-csrf-token'] === csrf?.value).toBe(true);
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
  await page.goto('/app');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
