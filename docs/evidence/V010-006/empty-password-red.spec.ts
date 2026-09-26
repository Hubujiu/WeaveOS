import { test, expect } from '@playwright/test';

test('WaveOS Figma 13:2 login shell uses the new brand and card geometry', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto('/login');
  await expect(page.getByRole('banner')).toContainText('WaveOS');
  await expect(page.getByRole('banner')).toContainText('安全 · 高效 · 连接世界');
  await expect(page.getByText('登录您的账号，开始高效沟通')).toBeVisible();
  const header = await page.getByRole('banner').boundingBox();
  const card = await page.locator('.auth-card').boundingBox();
  expect(header?.height).toBeGreaterThanOrEqual(90);
  expect(card?.width).toBeGreaterThanOrEqual(590);
  expect(card?.width).toBeLessThanOrEqual(610);
  expect(card?.height).toBeGreaterThanOrEqual(710);
  expect(card?.height).toBeLessThanOrEqual(730);
});

test('WaveOS Figma 40:2 register has four large fields and the invitation', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto('/register');
  await expect(page.getByText('创建您的账号，开始使用 WaveOS')).toBeVisible();
  await expect(page.getByLabel('邀请码', { exact: true })).toBeVisible();
  const fields = await page.locator('.auth-card input').all();
  expect(fields).toHaveLength(4);
  for (const field of fields) {
    const box = await field.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(54);
    expect(box?.height).toBeLessThanOrEqual(58);
  }
});

// Browser-only component tests. Each API response is controlled at the network boundary.
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
  await page.route('**/api/v1/registrations', route => {
    registrations += 1;
    return route.fulfill({ status: 500, body: '{}' });
  });
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
  await page.route('**/api/v1/sessions/current', route => route.fulfill({ status: 401, body: '{"code":"AUTH_UNAUTHENTICATED","message":"login required","data":null,"meta":null}', contentType: 'application/json' }));
  await page.goto('/app');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('API-14: a current-session service failure is shown as a system error', async ({ page }) => {
  await page.route('**/api/v1/sessions/current', route => route.fulfill({ status: 503, body: '{"code":"SYSTEM_UNAVAILABLE","message":"temporary","data":null,"meta":null}', contentType: 'application/json' }));
  await page.goto('/app');
  await expect(page).toHaveURL(/\/app(?:\?|$)/);
  await expect(page.getByRole('alert')).toContainText('服务暂时不可用');
});
test('FR-001 UX: login reports a network failure in plain language', async ({ page }) => {
  await page.route('**/api/v1/sessions', route => route.abort('failed'));
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-user');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('网络连接失败');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('FR-001: wrong credentials show an error and remain unauthenticated', async ({ page }) => {
  await page.route('**/api/v1/sessions', route => route.fulfill({ status: 401, body: '{"code":"AUTH_INVALID_CREDENTIALS","message":"invalid credentials","data":null,"meta":null}', contentType: 'application/json' }));
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-nonexistent-user');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});

test('PRD login rule: an empty password is rejected before submission', async ({ page }) => {
  let requests = 0;
  await page.route('**/api/v1/sessions', route => {
    requests += 1;
    return route.fulfill({ status: 400, body: '{}' });
  });
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-user');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('请输入密码');
  expect(requests).toBe(0);
});
