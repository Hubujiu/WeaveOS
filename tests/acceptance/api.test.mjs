import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

// Expected semantics are from PRD FR-001..018 and ADR-001/002, not BFF output.
// V010-002 must ratify these draft bindings before publishing the HTTP contract.
const routes = JSON.parse(readFileSync(new URL('./bindings.json', import.meta.url), 'utf8'));
const base = process.env.WEAVEOS_API_URL ?? 'http://127.0.0.1:8080';
const password = 'Acceptance@Test123'; // Synthetic test value, never a real credential.
function fixtures() {
  assert.ok(process.env.WEAVEOS_ACCEPTANCE_FIXTURES, 'BLOCKED: V010-003 must generate isolated acceptance fixtures');
  return JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES, 'utf8'));
}
async function send(path, method = 'GET', data, session, extra = {}) {
  return fetch(new URL(path, base), {
    method, redirect: 'manual', signal: AbortSignal.timeout(10000),
    headers: { Origin: base, ...(data === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(session ? { Cookie: session.cookie, 'X-CSRF-Token': session.csrf } : {}), ...extra },
    ...(data === undefined ? {} : { body: JSON.stringify(data) }),
  });
}
async function login(account) {
  const response = await send(routes.login, 'POST', account);
  assert.equal(response.status, 201, 'valid credentials create a session');
  const body = await response.json();
  assert.equal(body.code, 'OK');
  const cookies = response.headers.getSetCookie();
  const cookie = cookies.find(value => value.startsWith('__Host-session='));
  const csrfCookie = cookies.find(value => value.startsWith('__Host-csrf='));
  assert.ok(cookie, 'server must issue __Host-session');
  assert.ok(csrfCookie, 'server must issue independent __Host-csrf');
  assert.match(cookie, /;\s*HttpOnly(?:;|$)/i);
  assert.doesNotMatch(csrfCookie, /HttpOnly/i);
  for (const value of [cookie, csrfCookie]) {
    assert.match(value, /;\s*Secure(?:;|$)/i);
    assert.match(value, /SameSite=Lax/i);
    assert.match(value, /Path=\/(?:;|$)/i);
    assert.match(value, /Max-Age=3600/i);
    assert.doesNotMatch(value, /;\s*Domain=/i);
  }
  assert.equal(body.data.csrfToken, undefined, 'login JSON must not return csrfToken');
  const csrf = csrfCookie.split(';')[0].slice('__Host-csrf='.length);
  assert.match(csrf, /^[A-Za-z0-9_-]{43}$/);
  return { cookie: [cookie, csrfCookie].map(value => value.split(';')[0]).join('; '), csrf };
}
function unique(label) { return `acceptance-${label}-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`; }

test('FR-007: anonymous current-session request is unauthorized', async () => {
  assert.equal((await send(routes.current)).status, 401);
});
test('API-14: forged identity headers cannot authenticate a caller', async () => {
  const response = await send(routes.current, 'GET', undefined, undefined, { 'X-User-Id': 'bootstrap-admin', 'X-Role': 'ALL' });
  assert.equal(response.status, 401);
});
test('FR-001: wrong credentials never create a session or leak passwords', async () => {
  const response = await send(routes.login, 'POST', { account: 'definitely-unknown-acceptance', password });
  assert.equal(response.status, 401);
  assert.equal(response.headers.get('set-cookie'), null);
  const body = await response.json();
  assert.deepEqual(Object.keys(body).sort(), ['code', 'data', 'message', 'meta']);
  assert.equal(body.data, null);
  assert.ok(!JSON.stringify(body).includes(password));
});
for (const [label, invitationCode] of [['missing', undefined], ['invalid', 'synthetic-invalid-code']]) {
  test(`FR-003: ${label} invitation cannot create an account`, async () => {
    const response = await send(routes.register, 'POST', { account: unique(label), password, invitationCode });
    assert.equal(response.status, 400);
    assert.equal(response.headers.get('set-cookie'), null);
  });
}
for (const [label, invalid] of [['uppercase', 'lowercase@123'], ['lowercase', 'UPPERCASE@123'], ['digit', 'NoDigits@Here'], ['special', 'NoSpecial123']]) {
  test(`FR-018: password missing ${label} is rejected`, async () => {
    const f = fixtures();
    const response = await send(routes.register, 'POST', { account: unique(label), password: invalid, invitationCode: f.invitations.passwordPolicy });
    assert.equal(response.status, 400);
  });
}
test('FR-003/017/001/005/009: registration, explicit login, restore and logout form a real HTTP lifecycle', async () => {
  const f = fixtures(), account = unique('lifecycle');
  const registration = await send(routes.register, 'POST', { account, password, invitationCode: f.invitations.valid });
  assert.equal(registration.status, 201);
  assert.equal(registration.headers.get('set-cookie'), null, 'registration must not log in');
  assert.equal((await send(routes.current)).status, 401);
  const session = await login({ account, password });
  assert.equal((await send(routes.current, 'GET', undefined, session)).status, 200);
  assert.equal((await send(routes.current, 'DELETE', undefined, session)).status, 204);
  assert.equal((await send(routes.current, 'GET', undefined, session)).status, 401);
  const reuse = await send(routes.register, 'POST', { account: unique('reuse'), password, invitationCode: f.invitations.valid });
  assert.equal(reuse.status, 400);
});
test('FR-003: concurrent use of one invitation permits exactly one success', async () => {
  const f = fixtures();
  const responses = await Promise.all([0, 1].map(n => send(routes.register, 'POST', { account: unique(`race${n}`), password, invitationCode: f.invitations.concurrent })));
  assert.deepEqual(responses.map(r => r.status).sort(), [201, 400]);
});
test('FR-002/003: duplicate account failure does not consume an invitation', async () => {
  const f = fixtures();
  const duplicate = await send(routes.register, 'POST', { account: f.user.account, password, invitationCode: f.invitations.rollback });
  assert.equal(duplicate.status, 409);
  const retry = await send(routes.register, 'POST', { account: unique('rollback'), password, invitationCode: f.invitations.rollback });
  assert.equal(retry.status, 201);
});
test('FR-010: a disabled account cannot log in', async () => {
  const f = fixtures();
  assert.equal((await send(routes.login, 'POST', f.disabled)).status, 401);
});
test('FR-013: consecutive failures do not add a hidden account-lock feature', async () => {
  const f = fixtures();
  for (let i = 0; i < 6; i++) assert.equal((await send(routes.login, 'POST', { account: f.user.account, password: 'Wrong@Test123' })).status, 401);
  await login({ account: f.user.account, password: f.user.password });
});
test('FR-016: only Bootstrap Admin can generate invitations', async () => {
  const f = fixtures();
  const member = await login({ account: f.user.account, password: f.user.password });
  assert.equal((await send(routes.invitations, 'POST', {}, member)).status, 403);
  const admin = await login(f.admin);
  assert.equal((await send(routes.invitations, 'POST', {}, admin)).status, 201);
});
test('API-14: authenticated write without CSRF token is forbidden', async () => {
  const f = fixtures(), admin = await login(f.admin);
  assert.equal((await send(routes.invitations, 'POST', {}, admin, { 'X-CSRF-Token': '' })).status, 403);
});
test('FR-012/016: admin resets password, ordinary user cannot; response never contains the reset password', async () => {
  const f = fixtures();
  const path = routes.reset.replace('{userId}', encodeURIComponent(f.resetTarget.id));
  const user = await login({ account: f.user.account, password: f.user.password });
  assert.equal((await send(path, 'POST', {}, user)).status, 403);
  const admin = await login(f.admin);
  const response = await send(path, 'POST', {}, admin);
  assert.equal(response.status, 200);
  assert.ok(!(await response.text()).includes('Abc@123456'));
  await login({ account: f.resetTarget.account, password: 'Abc@123456' });
});
