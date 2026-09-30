import { test, expect } from '@playwright/test';

test('approved account dictionary: 254 Unicode characters can be entered and submitted', async ({ page }) => {
 const account='𠮷'.repeat(254);
 const sent: string[]=[];
 await page.route('**/api/v1/sessions',async route=>{sent.push(route.request().postDataJSON().account);await route.fulfill({status:401,body:'{}'});});
 await page.goto('/login');
 await page.getByLabel('账号',{exact:true}).pressSequentially(account);
 await expect(page.getByLabel('账号',{exact:true})).toHaveValue(account);
 await page.getByLabel('密码',{exact:true}).fill('Aa1!');
 await page.getByRole('button',{name:'登录',exact:true}).click();
 await expect.poll(()=>sent).toEqual([account]);
});

test('approved account dictionary: non-breaking edge spaces are preserved', async ({ page }) => {
 const account='\u00a0Alice\u00a0';const sent: string[]=[];
 await page.route('**/api/v1/sessions',async route=>{sent.push(route.request().postDataJSON().account);await route.fulfill({status:401,body:'{}'});});
 await page.goto('/login');await page.getByLabel('账号',{exact:true}).fill(account);
 await page.getByLabel('密码',{exact:true}).fill('Aa1!');
 await page.getByRole('button',{name:'登录',exact:true}).click();
 await expect.poll(()=>sent).toEqual([account]);
});

test('Figma brand text keeps its #102044 color above the glow on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/login');
  await page.evaluate(() => document.fonts.ready);
  const box = await page.locator('.brand-identity span').boundingBox();
  if (!box) throw new Error('visible brand text is required');
  const png = await page.screenshot({ clip: box });
  const matchingPixels = await page.evaluate(async encoded => {
    const img = new Image();
    img.src = `data:image/png;base64,${encoded}`;
    await img.decode();
    const canvas = document.createElement('canvas');
    canvas.width = img.width; canvas.height = img.height;
    const context = canvas.getContext('2d')!;
    context.drawImage(img, 0, 0);
    const pixels = context.getImageData(0, 0, img.width, img.height).data;
    let matches = 0;
    for (let i = 0; i < pixels.length; i += 4) if (Math.abs(pixels[i] - 16) <= 2 && Math.abs(pixels[i + 1] - 32) <= 2 && Math.abs(pixels[i + 2] - 68) <= 2) matches += 1;
    return matches;
  }, png.toString('base64'));
  expect(matchingPixels).toBeGreaterThan(0);
});

test('PRD UX: login loading prevents a second submission and restores after failure', async ({ page }) => {
  let count = 0;
  let finish: (() => void) | undefined;
  const waiting = new Promise<void>(resolve => { finish = resolve; });
  await page.route('**/api/v1/sessions', async route => {
    count += 1;
    await waiting;
    await route.fulfill({ status: 401, body: '{}' });
  });
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-user');
  await page.getByLabel('密码', { exact: true }).fill('Synthetic@123');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('button', { name: '登录中…', exact: true })).toBeDisabled();
  await page.getByLabel('密码', { exact: true }).press('Enter');
  expect(count).toBe(1);
  finish?.();
  await expect(page.getByRole('alert')).toContainText('邮箱或密码不正确');
  await expect(page.getByRole('button', { name: '登录', exact: true })).toBeEnabled();
  await expect(page.getByLabel('账号', { exact: true })).toHaveValue('synthetic-user');
});

test('PRD FR-017: successful registration returns to login without restoring a Session', async ({ page }) => {
  let sessionReads = 0;
  await page.route('**/api/v1/sessions/current', route => { sessionReads += 1; return route.fulfill({ status: 401, body: '{}' }); });
  await page.route('**/api/v1/registrations', route => route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ code: 'OK', message: 'ok', data: { id: 'synthetic-id', account: 'Alice' }, meta: null }) }));
  await page.goto('/register?invitationCode=synthetic-invitation');
  await page.getByLabel('账号', { exact: true }).fill('Alice');
  await page.getByLabel('密码', { exact: true }).fill('A@1a');
  await page.getByLabel('确认密码', { exact: true }).fill('A@1a');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(sessionReads).toBe(0);
});

test('Q7: logout sends the frontend-readable host CSRF cookie in the header', async ({ page, context }) => {
  const csrf = 'AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE'; // synthetic only
  await context.addCookies([{ name: '__Host-csrf', value: csrf, url: 'https://127.0.0.1:4173/', secure: true, sameSite: 'Lax' }]);
  let received: string | undefined;
  await page.route('**/api/v1/sessions/current', route => {
    if (route.request().method() === 'DELETE') {
      received = route.request().headers()['x-csrf-token'];
      return route.fulfill({ status: 204 });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 'OK', message: 'ok', data: { id: 'synthetic-id', account: 'synthetic-user' }, meta: null }) });
  });
  await page.goto('/app');
  expect(await page.evaluate(() => document.cookie.includes('__Host-csrf='))).toBe(true);
  await page.getByRole('button', { name: '退出登录' }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(received).toBe(csrf);
  expect(await page.evaluate(() => localStorage.length + sessionStorage.length)).toBe(0);
});

// Oracle: PRD/Figma Login 13:2 after user Q12, 2026-09-26.
test('Q12: future login controls stay visible and disabled', async ({ page }) => {
  await page.goto('/login');
  const remember = page.getByRole('checkbox', { name: '记住账号' });
  await expect(remember).toBeVisible();
  await expect(remember).toBeDisabled();
  for (const name of ['忘记密码?', 'Google', 'Microsoft', 'GitHub']) {
    const control = page.getByRole('button', { name, exact: true });
    await expect(control).toBeVisible();
    await expect(control).toBeDisabled();
  }
  await expect(page.getByText('或使用第三方账号登录')).toBeVisible();
});

test('WaveOS Figma 13:2 login shell uses the new brand and card geometry', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto('/login');
  await expect(page.getByRole('banner')).toContainText('WaveOS');
  await expect(page.getByRole('banner')).toContainText('安全 · 高效 · 连接世界');
  await expect(page.getByText('登录您的账号，开始高效沟通')).toBeVisible();
  const header = await page.getByRole('banner').boundingBox();
  const card = await page.locator('.auth-card').boundingBox();
  expect(header?.height).toBe(56);
  // Oracle: current Figma 13:2, 192px content inset, 12 columns / 32px gutters.
  expect(card?.width).toBeCloseTo(490.667, 0);
  expect(card?.height).toBeCloseTo(495, 0);

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
    expect(box?.height).toBeCloseTo(46, 3);

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
  await expect(page.getByRole('alert')).toContainText('邮箱或密码不正确');
  await expect(page.getByRole('alert')).toContainText('请检查您的密码后重新输入。为方便您，邮箱地址已保留。');
  await expect(page.getByLabel('账号', { exact: true })).toHaveValue('synthetic-nonexistent-user');
  await expect(page.locator('.auth-field').first().locator('.field-input')).toHaveCSS('background-color', 'rgb(244, 247, 253)');
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
