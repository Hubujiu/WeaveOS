import { readFileSync } from 'node:fs';
import { test, expect } from '@playwright/test';

// Tests are credential-bearing: do not persist page screenshots, traces or videos.
test.use({ trace: 'off', screenshot: 'off', video: 'off' });

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
test('FR-018: registration exposes accessible password-strength feedback', async ({ page }) => {
  await page.goto('/register');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  // PRD requires feedback, but does not mandate a progressbar implementation.
  await expect(page.getByLabel('密码强度', { exact: true })).toBeVisible();
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
});
test('FR-005/008/009: real login survives reload, then logout revokes it', async ({ page, context }) => {
  const f = fixtures();
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill(f.user.account);
  await page.getByLabel('密码', { exact: true }).fill(f.user.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page).toHaveURL(/\/app(?:\/|\?|$)/);
  await page.reload();
  expect(await page.getByText(f.user.account, { exact: true }).isVisible(), 'current account is visible').toBe(true);
  const cookies = await context.cookies();
  const authCookies = cookies.filter(cookie => cookie.httpOnly);
  expect(authCookies.length).toBeGreaterThan(0);
  for (const cookie of authCookies) {
    expect(cookie.secure).toBe(true);
    expect(['Lax', 'Strict']).toContain(cookie.sameSite);
    const exposed = await page.evaluate(value => document.cookie.includes(value) || JSON.stringify(localStorage).includes(value) || JSON.stringify(sessionStorage).includes(value), cookie.value);
    expect(exposed, 'Session is not exposed to JavaScript').toBe(false);
  }
  await page.getByRole('button', { name: '退出登录', exact: true }).click();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
  await page.goto('/app');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});

test('WEB-01 user confirmation 2026-09-25: registration has four labeled inputs', async ({ page }) => {
  await page.goto('/register');
  for (const label of ['账号', '密码', '确认密码', '邀请码']) await expect(page.getByLabel(label, { exact: true })).toBeVisible();
  for (const label of ['密码', '确认密码']) await expect(page.getByLabel(label, { exact: true })).toHaveAttribute('type', 'password');
});

for (const path of ['/login', '/register']) {
  test(`WEB-02 FR-011: ${path} has no self-service password recovery`, async ({ page }) => {
    await page.goto(path);
    await expect(page.getByRole('heading', { name: path === '/login' ? '登录' : '注册', exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: /忘记密码|找回密码/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /忘记密码|找回密码/ })).toHaveCount(0);
  });
}

test('WEB-03 FR-004: percent-encoded invitation is decoded once and remains editable', async ({ page }) => {
  await page.goto('/register?invitationCode=synthetic%2Bcode%2F%3D%252B');
  const input = page.getByLabel('邀请码', { exact: true });
  await expect(input).toHaveValue('synthetic+code/=%2B');
  await input.fill('synthetic-corrected-code');
  await expect(input).toHaveValue('synthetic-corrected-code');
});

test('WEB-04 FR-004: URL invitation is treated as text, not executable markup', async ({ page }) => {
  const value = '<img src=x onerror="window.__invitationExecuted=true">';
  await page.goto(`/register?invitationCode=${encodeURIComponent(value)}`);
  await expect(page.getByLabel('邀请码', { exact: true })).toHaveValue(value);
  expect(await page.evaluate(() => '__invitationExecuted' in window)).toBe(false);
});

for (const [path, button, labels] of [
  ['/login', '登录', ['账号', '密码']],
  ['/register', '注册', ['账号', '密码', '确认密码', '邀请码']],
] as const) {
  for (const omitted of labels) test(`WEB-05 PRD input validation: ${path} missing ${omitted} is not submitted`, async ({ page }) => {
    await page.goto(path);
    let submissions = 0;
    page.on('request', request => { if (request.method() === 'POST' && /\/api\/v1\/(sessions|registrations)$/.test(new URL(request.url()).pathname)) submissions++; });
    const values: Record<string, string> = { '账号': 'synthetic-form-user', '密码': 'Synthetic@123', '确认密码': 'Synthetic@123', '邀请码': 'synthetic-form-invitation' };
    for (const label of labels) if (label !== omitted) await page.getByLabel(label, { exact: true }).fill(values[label]);
    const submit = page.getByRole('button', { name: button, exact: true });
    // Both disabled-submit and HTML/inline validation satisfy the PRD.
    if (await submit.isEnabled()) await submit.click();
    expect(submissions, 'invalid form must not send a credential request').toBe(0);
    await expect(page).toHaveURL(new RegExp(`${path}(?:\\?|$)`));
  });
}

test('WEB-06 user confirmation: mismatched password confirmation prevents registration', async ({ page }) => {
  await page.goto('/register');
  let submissions = 0;
  page.on('request', r => { if (r.method() === 'POST' && new URL(r.url()).pathname === '/api/v1/registrations') submissions++; });
  for (const [label, value] of Object.entries({ '账号': 'synthetic-confirm-user', '密码': 'Synthetic@123', '确认密码': 'Different@123', '邀请码': 'synthetic-code' })) await page.getByLabel(label, { exact: true }).fill(value);
  const button = page.getByRole('button', { name: '注册', exact: true });
  if (await button.isEnabled()) await button.click();
  expect(submissions).toBe(0);
  await expect(page).toHaveURL(/\/register(?:\?|$)/);
  await expect(page.getByText(/密码.*不一致|密码.*不匹配|两次.*密码/)).toBeVisible();
});

for (const [label, value] of [['uppercase', 'lowercase@123'], ['lowercase', 'UPPERCASE@123'], ['digit', 'NoDigits@Here'], ['special', 'NoSpecial123']]) {
  test(`WEB-07 FR-018: missing ${label} gives feedback and prevents submission`, async ({ page }) => {
    await page.goto('/register');
    let submissions = 0;
    page.on('request', r => { if (r.method() === 'POST' && new URL(r.url()).pathname === '/api/v1/registrations') submissions++; });
    await page.getByLabel('账号', { exact: true }).fill('synthetic-policy-user');
    await page.getByLabel('密码', { exact: true }).fill(value);
    await page.getByLabel('确认密码', { exact: true }).fill(value);
    await page.getByLabel('邀请码', { exact: true }).fill('synthetic-policy-code');
    await expect(page.getByLabel('密码强度', { exact: true })).toBeVisible();
    const button = page.getByRole('button', { name: '注册', exact: true });
    if (await button.isEnabled()) await button.click();
    expect(submissions).toBe(0);
    await expect(page).toHaveURL(/\/register(?:\?|$)/);
  });
}

test('WEB-08 FR-004: prefilled invalid invitation is revalidated by the real server', async ({ page }) => {
  await page.goto('/register?invitationCode=synthetic-invalid-invitation');
  await page.getByLabel('账号', { exact: true }).fill(`invalid-${Date.now()}`);
  for (const label of ['密码', '确认密码']) await page.getByLabel(label, { exact: true }).fill('Synthetic@123');
  const response = page.waitForResponse(r => new URL(r.url()).pathname === '/api/v1/registrations' && r.request().method() === 'POST');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  expect((await response).status()).toBe(400);
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page).toHaveURL(/\/register(?:\?|$)/);
});

test('WEB-09 PRD keyboard: Enter submits login without mouse interaction', async ({ page }) => {
  const f = fixtures();
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill(f.user.account);
  await page.getByLabel('密码', { exact: true }).fill(f.user.password);
  await page.getByLabel('密码', { exact: true }).press('Enter');
  await expect(page).toHaveURL(/\/app(?:\/|\?|$)/);
});

test('WEB-10 FR-017: registration returns to login and requires password re-entry', async ({ page }, info) => {
  const f = fixtures();
  const code = f.uiInvitations[`${info.project.name}-reentry`];
  if (!code) throw new Error('BLOCKED: isolated per-test reentry invitation fixture missing');
  await page.goto(`/register?invitationCode=${encodeURIComponent(code)}`);
  await page.getByLabel('账号', { exact: true }).fill(`reentry-${info.project.name}-${Date.now()}`);
  for (const label of ['密码', '确认密码']) await page.getByLabel(label, { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
  expect(await page.getByLabel('密码', { exact: true }).inputValue() === '', 'password is not carried into login').toBe(true);
});

test('WEB-11 PRD UX: login shows pending state and blocks duplicate submit during a real request', async ({ page }) => {
  const f = fixtures();
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill(f.user.account);
  await page.getByLabel('密码', { exact: true }).fill(f.user.password);
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let entered!: () => void;
  const intercepted = new Promise<void>(resolve => { entered = resolve; });
  let submissions = 0;
  // Delay transport; forward to the actual BFF. Never fulfill a fabricated response.
  await page.route('**/api/v1/sessions', async route => {
    if (route.request().method() !== 'POST') return route.continue();
    submissions++;
    entered();
    await gate;
    await route.continue();
  });
  try {
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await intercepted;
    const pending = page.getByRole('button', { name: /登录|登录中/ });
    await expect(pending).toBeDisabled();
    await page.getByLabel('密码', { exact: true }).press('Enter');
    expect(submissions).toBe(1);
  } finally { release(); }
  await expect(page).toHaveURL(/\/app(?:\/|\?|$)/);
  expect(submissions).toBe(1);
});

test('WEB-12 PRD login: network failure differs from credential failure and permits retry', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-unknown-user');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  const credentialsError = await page.getByRole('alert').textContent();
  // Real transport fault injection, not a mocked business response.
  await page.route('**/api/v1/sessions', route => route.abort('connectionfailed'));
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  await expect.poll(async () => (await page.getByRole('alert').textContent()) !== credentialsError).toBe(true);
  await expect(page.getByRole('button', { name: '登录', exact: true })).toBeEnabled();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
