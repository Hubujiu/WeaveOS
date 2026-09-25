import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { randomUUID } from 'node:crypto';

// Transport only: never implements authentication or supplies fake business results.
export const routes = JSON.parse(readFileSync(new URL('./bindings.json', import.meta.url), 'utf8'));
export const base = process.env.WEAVEOS_API_URL ?? 'http://127.0.0.1:8080';
export const password = 'Synthetic@Test123';
export const unique = () => `test-${randomUUID()}`;
export function fixtures() {
  const file = process.env.WEAVEOS_ACCEPTANCE_FIXTURES;
  if (!file) throw new Error('BLOCKED: isolated V010-003 acceptance fixtures are unavailable');
  try { return JSON.parse(readFileSync(file, 'utf8')); }
  catch { throw new Error('BLOCKED: fixture unreadable; contents withheld'); }
}
export async function send(path, { method = 'GET', data, raw, session, headers = {} } = {}) {
  const target = new URL(base);
  if (!['127.0.0.1', 'localhost', '[::1]'].includes(target.hostname)) throw new Error('BLOCKED: acceptance requires an isolated loopback target');
  const h = new Headers({ Origin: target.origin,
    ...(data !== undefined || raw !== undefined ? { 'Content-Type': 'application/json' } : {}),
    ...(session ? { Cookie: session.cookie, ...(session.csrf ? { 'X-CSRF-Token': session.csrf } : {}) } : {}) });
  for (const [k, v] of Object.entries(headers)) { if (v === null) h.delete(k); else h.set(k, v); }
  try { return await fetch(new URL(path, base), { method, headers: h, redirect: 'manual', signal: AbortSignal.timeout(10000),
    ...(raw !== undefined ? { body: raw } : data !== undefined ? { body: JSON.stringify(data) } : {}) }); }
  catch { throw new Error('BLOCKED: HTTP target unreachable or timed out; not a behavioral RED'); }
}
export function noSecret(text, values) {
  // Boolean assertions do not print either the secret or the response on failure.
  for (const value of values.filter(v => typeof v === 'string' && v.length)) assert.ok(!text.includes(value), 'sensitive value must not be exposed');
}
export async function envelope(response, status, code) {
  assert.equal(response.status, status, 'HTTP status from independent requirement');
  assert.ok(/^application\/json(?:;|$)/i.test(response.headers.get('content-type') ?? ''), 'ordinary API response is JSON');
  let body;
  try { body = await response.json(); } catch { assert.fail('response must be JSON; body withheld'); }
  assert.ok(body && typeof body === 'object' && !Array.isArray(body));
  assert.ok(JSON.stringify(Object.keys(body).sort()) === JSON.stringify(['code', 'data', 'message', 'meta']), 'exactly four Envelope keys');
  assert.ok(body.code === code, `expected business code ${code}; actual withheld`);
  assert.ok(typeof body.message === 'string' && body.message.length > 0);
  assert.ok(body.meta === null || typeof body.meta === 'object' && !Array.isArray(body.meta));
  assert.ok(response.headers.get('x-request-id'), 'API-15 correlation header');
  if (status === 401) assert.ok(response.headers.get('www-authenticate') === 'Session realm="enterprise-management-system"', 'API-14 Session challenge');
  if (status >= 400 && code !== 'COMMON_VALIDATION_FAILED') assert.equal(body.data, null);
  return body;
}
export function sessionFrom(response, body) {
  const cookies = response.headers.getSetCookie();
  const auth = cookies.filter(c => /;\s*HttpOnly(?:;|$)/i.test(c));
  assert.equal(auth.length, 1, 'one server-side Session cookie');
  assert.ok(/;\s*Secure(?:;|$)/i.test(auth[0]), 'Secure cookie');
  assert.ok(/;\s*SameSite=(Lax|Strict)(?:;|$)/i.test(auth[0]), 'SameSite cookie');
  const pair = auth[0].split(';')[0], sid = pair.slice(pair.indexOf('=') + 1);
  assert.ok(sid.length > 0);
  noSecret(JSON.stringify(body), [sid]);
  const csrfCookie = cookies.find(c => !/;\s*HttpOnly(?:;|$)/i.test(c) && /csrf/i.test(c.split('=')[0]));
  // Optional draft adapter; token strategy is NOT a mandatory test expectation.
  const csrf = body.data?.csrfToken ?? (csrfCookie ? csrfCookie.split(';')[0].slice(csrfCookie.indexOf('=') + 1) : undefined);
  return { cookie: cookies.map(c => c.split(';')[0]).join('; '), auth: pair, sid, csrf, setCookie: auth[0] };
}
export async function login(account, headers) {
  const response = await send(routes.login, { method: 'POST', data: { account: account.account, password: account.password }, headers });
  const body = await envelope(response, 201, 'OK');
  noSecret(JSON.stringify(body), [account.password]);
  return sessionFrom(response, body);
}
export async function preparedLogin(account) {
  if (!account?.account || !account?.password) throw new Error('BLOCKED: required isolated account fixture missing');
  const response = await send(routes.login, { method: 'POST', data: { account: account.account, password: account.password } });
  if (response.status !== 201) throw new Error(`BLOCKED: prerequisite login returned ${response.status}; downstream behavior not reached`);
  return sessionFrom(response, await envelope(response, 201, 'OK'));
}
export async function invitation() {
  const session = await preparedLogin(fixtures().admin);
  const response = await send(routes.invitations, { method: 'POST', data: {}, session });
  if (response.status !== 201) throw new Error(`BLOCKED: invitation prerequisite returned ${response.status}`);
  const body = await envelope(response, 201, 'OK');
  if (typeof body.data?.invitationCode !== 'string' || !body.data.invitationCode) throw new Error('BLOCKED: proposed invitationCode DTO binding not available');
  return body.data.invitationCode;
}
export const register = (account, invitationCode, secret = password) => send(routes.register, { method: 'POST', data: { account, password: secret, invitationCode } });
