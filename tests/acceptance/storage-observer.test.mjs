import assert from 'node:assert/strict';
import { test } from 'node:test';
import { execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { open } from './storage-observer.mjs';
import { fixtures, preparedLogin, send, routes, unique, password, invitation, register, envelope } from './http.mjs';

const project = process.env.WEAVEOS_ACCEPTANCE_PROJECT;
const docker = args => execFileSync('docker', args, { encoding: 'utf8', stdio: 'pipe' }).trim();
const redis = (...args) => docker(['exec', `${project}-redis-1`, 'redis-cli', '--raw', ...args]);
const keyOf = auth => `ems:auth:session:${project}:v1:${createHash('sha256').update(Buffer.from(auth.split('=')[1], 'base64url')).digest('hex')}`;
const observed = async t => { const o = await open({baseURL: process.env.WEAVEOS_API_URL}); t.after(() => o.close()); return o; };

test('observer changes native TTL before testing renewal', async t => {
  const o = await observed(t), s = await preparedLogin(fixtures().user), key = keyOf(s.auth);
  await o.setSessionPTTL(s.auth, 30000);
  const ttl = Number(redis('PTTL', key));
  assert.ok(ttl > 0 && ttl <= 30000, 'native key must exist with a shortened positive TTL');
});
test('observer changes native creation timestamp', async t => {
  const o = await observed(t), s = await preparedLogin(fixtures().user);
  await o.setSessionCreationAge(s.auth, 604800000);
  assert.ok(Date.now() - JSON.parse(redis('GET', keyOf(s.auth))).created_at_unix_ms >= 604800000, 'native timestamp must change');
});
test('observer disables an independent account in PostgreSQL', async t => {
  const o = await observed(t), account = unique();
  const body = await envelope(await register(account, await invitation()), 201, 'OK');
  const s = await preparedLogin({account, password});
  await o.setUserStatus(body.data.id, 'disabled');
  assert.equal((await send(routes.current, {session:s})).status, 401);
});
for (const dependency of ['postgresql', 'redis']) test(`observer interrupts real application ${dependency} traffic`, async t => {
  const o = await observed(t), s = await preparedLogin(fixtures().user), restore = await o.disconnectApplication(dependency);
  try { assert.equal((await send(routes.current, {session:s})).status, 503); }
  finally { await restore(); }
  assert.equal((await send(routes.current, {session:s})).status, 200);
});
test('observer forces registration commit failure without consuming its invitation', async t => {
  const o = await observed(t), account = unique(), code = await invitation(), release = await o.failNextRegistrationBeforeCommit(account);
  try { assert.ok((await register(account, code)).status >= 500, 'real transaction must fail'); }
  finally { await release(); }
  assert.equal((await register(account, code)).status, 201);
});

test('storage observer expires an existing native Redis key, preserving its neighbor', async () => {
  const project = process.env.WEAVEOS_ACCEPTANCE_PROJECT;
  assert.match(project ?? '', /^weaveos-v010-007-\d+$/);
  const redis = (...args) => execFileSync('docker', ['exec', `${project}-redis-1`, 'redis-cli', '--raw', ...args], { encoding: 'utf8', stdio: 'pipe' }).trim();
  const raw = randomBytes(32), neighbor = `ems:auth:session:${project}:v1:qualification-neighbor`;
  const key = `ems:auth:session:${project}:v1:${createHash('sha256').update(raw).digest('hex')}`;
  redis('SET', key, 'qualification', 'PX', '60000');
  redis('SET', neighbor, 'neighbor', 'PX', '60000');
  const observer = await open({ baseURL: process.env.WEAVEOS_API_URL });
  try {
    assert.ok(Number(redis('PTTL', key)) > 0);
    await observer.expireSession(`__Host-session=${raw.toString('base64url')}`);
    assert.equal(Number(redis('PTTL', key)), -2, 'target must actually expire in Redis');
    assert.equal(redis('GET', neighbor), 'neighbor');
  } finally { redis('DEL', key, neighbor); await observer.close(); }
});
