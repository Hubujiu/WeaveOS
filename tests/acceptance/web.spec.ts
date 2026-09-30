import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
import { test, expect, type Page, type TestInfo } from '@playwright/test';

// Tests are credential-bearing: do not persist page screenshots, traces or videos.
test.use({ trace: 'off', screenshot: 'off', video: 'off' });

// PRD-defined visible behavior; proposed /login, /register, /app route bindings are reviewed in V010-002/006.
function fixtures(): { admin: { account: string; password: string }; user: { account: string; password: string }; disabled: { account: string; password: string }; uiInvitations: Record<string, string> } {
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
  // PRD requires feedback, but does not mandate a progressbar implementation.
  await expect(page.getByLabel('密码强度', { exact: true })).toBeVisible();
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
  expect((await context.cookies()).filter(cookie => cookie.httpOnly).length).toBe(0);
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
  // Q24 Figma Home: account information/logout now live inside the account menu.
  await page.getByRole('button', { name: '账号', exact: true }).click();
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
    expect(exposed, 'Session is not exposed to JavaScript').toBe(false);
  }
  const logoutRequest = page.waitForRequest(request => request.method() === 'DELETE' && request.url().endsWith('/api/v1/sessions/current'));
  await page.getByRole('button', { name: '退出登录', exact: true }).click();
  expect((await logoutRequest).headers()['x-csrf-token'] === csrf?.value).toBe(true);
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
    const recovery = page.getByRole('button', { name: /忘记密码|找回密码/ });
    if (path === '/login') {
      // Q12 supersedes the early absent-control draft: visible, disabled, no workflow.
      await expect(recovery).toBeVisible();
      await expect(recovery).toBeDisabled();
    } else await expect(recovery).toHaveCount(0);
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
  await expect(page.getByLabel('密码', { exact: true }), 'password is not carried into login').toHaveValue('');
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

async function fillRegistration(page: Page, account: string, code: string) {
  await page.goto('/register');
  await page.getByLabel('账号', { exact: true }).fill(account);
  for (const label of ['密码', '确认密码']) await page.getByLabel(label, { exact: true }).fill('Synthetic@123');
  await page.getByLabel('邀请码', { exact: true }).fill(code);
}
function caseInvitation(info: TestInfo, label: string) {
  const code = fixtures().uiInvitations[`${info.project.name}-${label}`];
  if (!code) throw new Error(`BLOCKED: independent browser invitation fixture ${label} missing`);
  return code;
}
test('WEB-13 FR-010: Disabled user sees failure and cannot enter protected app', async ({ page }) => {
  const user = fixtures().disabled;
  if (!user) throw new Error('BLOCKED: disabled browser fixture missing');
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill(user.account);
  await page.getByLabel('密码', { exact: true }).fill(user.password);
  const response = page.waitForResponse(r => new URL(r.url()).pathname === '/api/v1/sessions' && r.request().method() === 'POST');
  await page.getByRole('button', { name: '登录', exact: true }).click();
  expect((await response).status()).toBe(401);
  await expect(page.getByRole('alert')).toBeVisible();
  await page.goto('/app');
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('WEB-14 FR-002/003: duplicate-account feedback permits retry without losing invitation', async ({ page }, info) => {
  await fillRegistration(page, fixtures().user.account, caseInvitation(info, 'duplicate'));
  const response = page.waitForResponse(r => new URL(r.url()).pathname === '/api/v1/registrations' && r.request().method() === 'POST');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  expect((await response).status()).toBe(409);
  await expect(page.getByRole('alert')).toBeVisible();
  await page.getByLabel('账号', { exact: true }).fill(`retry-${info.project.name}-${Date.now()}`);
  // Re-enter secrets if UI cleared them; invitation remains independently re-usable.
  for (const label of ['密码', '确认密码']) await page.getByLabel(label, { exact: true }).fill('Synthetic@123');
  await page.getByLabel('邀请码', { exact: true }).fill(caseInvitation(info, 'duplicate'));
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
});
test('WEB-15 FR-003: already-used invitation shows failure and keeps browser unauthenticated', async ({ page, context }, info) => {
  const code = caseInvitation(info, 'reuse');
  await fillRegistration(page, `first-${info.project.name}-${Date.now()}`, code);
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
  await fillRegistration(page, `second-${info.project.name}-${Date.now()}`, code);
  const response = page.waitForResponse(r => new URL(r.url()).pathname === '/api/v1/registrations' && r.request().method() === 'POST');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  expect((await response).status()).toBe(409);
  await expect(page.getByRole('alert')).toBeVisible();
  expect((await context.cookies()).filter(c => c.httpOnly).length).toBe(0);
});
test('WEB-16 PRD UX: pending real registration prevents duplicate submission', async ({ page }, info) => {
  await fillRegistration(page, `pending-${info.project.name}-${Date.now()}`, caseInvitation(info, 'pending'));
  let release!: () => void, entered!: () => void, count = 0;
  const gate = new Promise<void>(r => { release = r; });
  const intercepted = new Promise<void>(r => { entered = r; });
  await page.route('**/api/v1/registrations', async route => { count++; entered(); await gate; await route.continue(); });
  try {
    await page.getByRole('button', { name: '注册', exact: true }).click();
    await intercepted;
    await expect(page.getByRole('button', { name: /注册|注册中/ })).toBeDisabled();
    await page.getByLabel('确认密码', { exact: true }).press('Enter');
    expect(count).toBe(1);
  } finally { release(); }
  await expect(page).toHaveURL(/\/login(?:\?|$)/);
  expect(count).toBe(1);
});
test('WEB-17 FR-008: Redis-expired browser Session returns to login', async ({ page, context }, info) => {
  const file = process.env.WEAVEOS_ACCEPTANCE_OBSERVER;
  const module = file ? await import(pathToFileURL(resolve(file)).href) : await import('./redis-observer.mjs');
  const o = await module.open({ baseURL: info.project.use.baseURL });
  try {
    if (o.storage !== 'isolated-redis') throw new Error('BLOCKED: requires real isolated storage');
    const f = fixtures();
    await page.goto('/login');
    await page.getByLabel('账号', { exact: true }).fill(f.user.account);
    await page.getByLabel('密码', { exact: true }).fill(f.user.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await expect(page).toHaveURL(/\/app(?:\/|\?|$)/);
    const auth = (await context.cookies()).find(c => c.httpOnly);
    expect(Boolean(auth), 'Session cookie exists').toBe(true);
    await o.expireSession(`${auth!.name}=${auth!.value}`);
    await page.reload();
    await expect(page).toHaveURL(/\/login(?:\?|$)/);
  } finally { await o.close(); }
});
test('WEB-18 PRD security: frontend logs do not contain submitted password', async ({ page }) => {
  const messages: string[] = [], secret = 'Synthetic@123';
  page.on('console', message => messages.push(message.text()));
  page.on('pageerror', error => messages.push(error.message));
  await page.goto('/login');
  await page.getByLabel('账号', { exact: true }).fill('synthetic-unknown-user');
  await page.getByLabel('密码', { exact: true }).fill(secret);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  expect(messages.every(message => !message.includes(secret)), 'logs are credential-free; content withheld').toBe(true);
});

async function personnelLogin(page:Page,credentials:{account:string;password:string}){
 await page.goto('/login');await page.getByLabel('账号',{exact:true}).fill(credentials.account);await page.getByLabel('密码',{exact:true}).fill(credentials.password);await page.getByRole('button',{name:'登录',exact:true}).click();await expect(page).toHaveURL(/\/app$/);
}
test('R2/Q25 real ordinary Home, own account and direct admin denial',async({page})=>{
 const f=fixtures();await personnelLogin(page,f.user);await expect(page.getByRole('button',{name:'设置',exact:true})).toBeHidden();await expect(page.getByText('暂无可用应用',{exact:true})).toBeVisible();await page.getByRole('button',{name:'账号',exact:true}).click();await expect(page.getByText('尚未分配身份',{exact:true})).toBeVisible();await page.goto('/app/admin');await expect(page.getByRole('alert')).toContainText('没有人员管理权限');await expect(page.getByRole('tab',{name:'身份',exact:true})).toBeHidden();
});
test('R3 real Root UI creates shared template/identity and assigns a new non-Root manager',async({page,context},info)=>{
 const f=fixtures(),label='browser-'+info.project.name+'-'+Date.now();await personnelLogin(page,f.admin);await page.getByRole('button',{name:'设置',exact:true}).click();
 const csrf=(await context.cookies()).find(c=>c.name==='__Host-csrf')?.value;expect(Boolean(csrf)).toBe(true);const origin=new URL(process.env.WEAVEOS_WEB_URL!).origin;
 const inviteResponse=await context.request.post('/api/v1/invitations',{headers:{Origin:origin,'X-CSRF-Token':csrf!},data:{}});expect(inviteResponse.status()).toBe(201);const invitation=(await inviteResponse.json()).data;
 const registerResponse=await context.request.post('/api/v1/registrations',{headers:{Origin:origin},data:{account:label,password:'Synthetic@123',invitationCode:invitation.invitationCode}});expect(registerResponse.status()).toBe(201);
 await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'新建权限模板',exact:true}).click();await page.getByLabel('模板名称',{exact:true}).fill(label+'-模板');await page.getByLabel('中央权限：人员管理',{exact:true}).check();await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect(page.getByRole('status')).toContainText('已保存');
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'新建身份',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill(label+'-身份');await page.getByLabel('模板：'+label+'-模板',{exact:true}).check();await expect(page.getByLabel('直接权限：人员管理',{exact:true})).not.toBeChecked();await expect(page.getByText('来自模板：'+label+'-模板',{exact:true})).toBeVisible();await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect(page.getByRole('status')).toContainText('已保存');
 await page.getByRole('tab',{name:'成员与部门',exact:true}).click();await page.getByLabel('搜索成员',{exact:true}).fill(label);const row=page.getByRole('row').filter({hasText:label});await expect(row).toHaveCount(1);await row.getByRole('button',{name:'配置身份',exact:true}).click();await page.getByLabel('身份：'+label+'-身份',{exact:true}).check();await page.getByRole('button',{name:'确认分配',exact:true}).click();await expect(page.getByRole('status')).toContainText('身份已分配');await expect(row).toContainText('已开启');
 await page.getByRole('button',{name:'退出',exact:true}).click();await page.getByRole('button',{name:'账号',exact:true}).click();await page.getByRole('button',{name:'退出登录',exact:true}).click();await personnelLogin(page,{account:label,password:'Synthetic@123'});await expect(page.getByRole('button',{name:'设置',exact:true})).toBeVisible();await page.getByRole('button',{name:'账号',exact:true}).click();await expect(page.getByText(label+'-身份',{exact:true})).toBeVisible();await expect(page.getByText(label+'-身份 · '+label+'-模板',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'设置',exact:true}).click();await page.getByRole('button',{name:'邀请成员',exact:true}).click();await page.getByRole('button',{name:'生成邀请码',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).not.toHaveValue('');await page.getByRole('button',{name:'关闭',exact:true}).click();
 await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByLabel('搜索权限模板',{exact:true}).fill(label+'-模板');await page.getByRole('button',{name:label+'-模板',exact:true}).click();await page.getByLabel('中央权限：人员管理',{exact:true}).uncheck();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('1 位成员');await page.getByRole('button',{name:'确认保存',exact:true}).click();
 await page.getByRole('button',{name:'退出',exact:true}).click();await expect(page.getByRole('button',{name:'设置',exact:true})).toBeHidden();await page.goto('/app/admin');await expect(page.getByRole('alert')).toContainText('没有人员管理权限');
});

