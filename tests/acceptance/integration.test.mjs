import assert from 'node:assert/strict';
import { test } from 'node:test';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
import { performance } from 'node:perf_hooks';
import { routes, password, unique, fixtures, send, envelope, noSecret, preparedLogin, invitation, register } from './http.mjs';

// Logical observations only, based on accepted PRD/ADR; no draft SQL or Redis layout.
// V010-003/004 must supply the REAL storage observer described in observer-contract.md.
// An absent observer is a prerequisite failure, never a RED or a skip/pass.
async function observe(t) {
  const file = process.env.WEAVEOS_ACCEPTANCE_OBSERVER;
  if (!file) throw new Error('BLOCKED: real PostgreSQL/Redis observer is not implemented; see observer-contract.md');
  const module = await import(pathToFileURL(resolve(file)).href);
  const observer = await module.open({ baseURL: process.env.WEAVEOS_API_URL ?? 'http://127.0.0.1:8080' });
  t.after(() => observer.close());
  // This declaration is a prerequisite, not proof of real storage; review driver and run provenance.
  if (observer.storage !== 'isolated-postgresql-and-redis') throw new Error('BLOCKED: observer must use real isolated PostgreSQL and Redis');
  return observer;
}
test('STORE-01 FR-006: newly created Redis Session has native one-hour TTL', async t => {
  const o = await observe(t), start = performance.now(), session = await preparedLogin(fixtures().user);
  const ttl = await o.sessionPTTL(session.auth), elapsed = performance.now() - start;
  assert.ok(ttl > 0 && ttl <= 3600000, 'Redis PTTL is positive and at most one hour');
  assert.ok(ttl >= 3600000 - elapsed - 100, 'initial TTL matches 3600 seconds within measured transport time');
});
test('STORE-02 FR-006: successful authentication renews a shortened native TTL', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user);
  await o.setSessionPTTL(session.auth, 30000);
  const start = performance.now();
  await envelope(await send(routes.current, { session }), 200, 'OK');
  const ttl = await o.sessionPTTL(session.auth);
  assert.ok(ttl <= 3600000 && ttl >= 3600000 - (performance.now() - start) - 100);
});
test('STORE-03 FR-006: rejected CSRF activity does not renew Redis TTL', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().admin);
  await o.setSessionPTTL(session.auth, 30000);
  const before = await o.sessionPTTL(session.auth);
  assert.equal((await send(routes.invitations, { method: 'POST', data: {}, session, headers: { Origin: 'https://attacker.invalid' } })).status, 403);
  const after = await o.sessionPTTL(session.auth);
  assert.ok(after > 0 && after <= before, 'rejected activity does not move expiry forward');
});
test('STORE-04 FR-006: failed privileged activity does not renew ordinary member TTL', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user);
  await o.setSessionPTTL(session.auth, 30000);
  const before = await o.sessionPTTL(session.auth);
  await envelope(await send(routes.invitations, { method: 'POST', data: {}, session }), 403, 'COMMON_PERMISSION_DENIED');
  assert.ok(await o.sessionPTTL(session.auth) <= before);
});
test('STORE-05 FR-008: Redis-expired Session is denied and not recreated by reads', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user);
  // Native expiration, not deleting a fake map or sleeping for an hour.
  await o.expireSession(session.auth);
  assert.equal(await o.sessionPTTL(session.auth), -2);
  await envelope(await send(routes.current, { session }), 401, 'AUTH_UNAUTHENTICATED');
  assert.equal(await o.sessionPTTL(session.auth), -2);
});
test('STORE-06 FR-006: old creation time does not impose an absolute lifetime', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user);
  await o.setSessionCreationAge(session.auth, 7 * 24 * 3600000);
  await envelope(await send(routes.current, { session }), 200, 'OK');
});
test('STORE-07 ADR-001: disabling user rejects an existing Session', async t => {
  const o = await observe(t), user = fixtures().disableTarget;
  if (!user) throw new Error('BLOCKED: dedicated disableTarget fixture missing');
  const session = await preparedLogin(user);
  await o.setUserStatus(user.id, 'disabled');
  const r = await send(routes.current, { session });
  assert.equal(r.status, 401);
  const body = await r.json();
  assert.ok(body.data === null, 'ordinary error data must be null; payload withheld');
  // Re-enable semantics remain a separate unapproved design decision.
});
for (const dependency of ['redis', 'postgresql']) {
  test(`STORE-08 PRD fail closed: ${dependency} outage denies authenticated access as service failure`, async t => {
    const o = await observe(t), session = await preparedLogin(fixtures().user);
    const restore = await o.disconnectApplication(dependency);
    try { await envelope(await send(routes.current, { session }), 503, 'COMMON_SERVICE_UNAVAILABLE'); }
    finally { await restore(); }
  });
  test(`STORE-09 PRD fail closed: ${dependency} outage cannot report a successful login`, async t => {
    const o = await observe(t), user = fixtures().user, restore = await o.disconnectApplication(dependency);
    try {
      const r = await send(routes.login, { method: 'POST', data: { account: user.account, password: user.password } });
      await envelope(r, 503, 'COMMON_SERVICE_UNAVAILABLE');
      assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
    } finally { await restore(); }
  });
}
test('STORE-10 FR-009: Redis failure cannot falsely confirm logout', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user), restore = await o.disconnectApplication('redis');
  try { await envelope(await send(routes.current, { method: 'DELETE', session }), 503, 'COMMON_SERVICE_UNAVAILABLE'); }
  finally { await restore(); }
});
test('STORE-11 FR-003: forced transaction failure creates no account and does not consume invitation', async t => {
  const o = await observe(t), code = await invitation(), account = unique();
  const release = await o.failNextRegistrationBeforeCommit(account);
  try { await envelope(await register(account, code), 500, 'COMMON_INTERNAL_ERROR'); }
  finally { await release(); }
  assert.equal(await o.countUsersByExactAccount(account), 0);
  assert.equal(await o.countInvitationUses(code), 0);
  await envelope(await register(account, code), 201, 'OK');
  assert.equal(await o.countUsersByExactAccount(account), 1);
  assert.equal(await o.countInvitationUses(code), 1);
});
test('STORE-12 FR-003: concurrent registration commits one user, one credential, one consumption', async t => {
  const o = await observe(t), code = await invitation(), accounts = Array.from({ length: 8 }, unique);
  const responses = await Promise.all(accounts.map(account => register(account, code)));
  assert.deepEqual(responses.map(r => r.status).sort(), [201, 409, 409, 409, 409, 409, 409, 409]);
  assert.equal(await o.countInvitationUses(code), 1);
  for (let i = 0; i < accounts.length; i++) {
    const expected = responses[i].status === 201 ? 1 : 0;
    assert.equal(await o.countUsersByExactAccount(accounts[i]), expected);
    assert.equal(await o.countCredentialsByExactAccount(accounts[i]), expected);
  }
});
test('STORE-13 ADR-001: registration stores independent salted password hashes, never plaintext', async t => {
  const o = await observe(t), accounts = [unique(), unique()];
  for (const account of accounts) await envelope(await register(account, await invitation()), 201, 'OK');
  const hashes = await Promise.all(accounts.map(account => o.readPasswordHash(account)));
  for (const hash of hashes) {
    assert.ok(typeof hash === 'string' && hash.length > 0);
    noSecret(hash, [password, Buffer.from(password).toString('base64')]);
  }
  assert.ok(hashes[0] !== hashes[1], 'same password receives independent salt');
  const bytes = await o.dumpOwnedAuthenticationState(accounts);
  noSecret(bytes, [password]);
  // Algorithm/cost verification remains BLOCKED until the implementation specification is accepted.
});
test('STORE-14 FR-014: success and failure login events contain required observations without secrets', async t => {
  const o = await observe(t), user = fixtures().user, marker = `acceptance-${unique()}`, since = new Date();
  const session = await preparedLogin(user);
  const response = await send(routes.login, { method: 'POST', data: { account: user.account, password: 'Wrong@Test123' }, headers: { 'User-Agent': marker } });
  await envelope(response, 401, 'AUTH_INVALID_CREDENTIALS');
  const events = await o.readLoginEvents({ userId: user.id, since });
  for (const result of ['success', 'failure']) {
    const event = events.find(e => e.result === result);
    assert.ok(event, `login ${result} event exists`);
    assert.ok(event.userId === user.id || typeof event.accountIdentifier === 'string' && event.accountIdentifier.length > 0);
    assert.ok(typeof event.ip === 'string' && event.ip.length > 0);
    assert.ok(typeof event.userAgent === 'string' && event.userAgent.length > 0);
    assert.ok(Number.isFinite(Date.parse(event.time)) && Date.parse(event.time) >= since.getTime() - 1000);
  }
  assert.ok(events.some(e => e.result === 'failure' && e.userAgent === marker));
  noSecret(JSON.stringify(events) + await o.readApplicationLogs({ since }), [user.password, 'Wrong@Test123', session.sid, session.cookie]);
});
test('STORE-15 PRD rollback: restored storage never resurrects a confirmed logged-out Session', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user), backup = await o.snapshotIsolatedStorage();
  assert.equal((await send(routes.current, { method: 'DELETE', session })).status, 204);
  // Runs the reviewed recovery procedure, not raw restore. Mechanism is not dictated here.
  await o.restoreThroughRecoveryProcedure(backup);
  await envelope(await send(routes.current, { session }), 401, 'AUTH_UNAUTHENTICATED');
});
test('STORE-16 FR-009: deterministic delete-before-renewal never restores logged-out key', async t => {
  const o = await observe(t), session = await preparedLogin(fixtures().user);
  const barrier = await o.holdNextRenewal(session.auth);
  const inFlight = send(routes.current, { session });
  try {
    await barrier.waitUntilHeld(); // Real request passed authentication but has not committed touch.
    assert.equal((await send(routes.current, { method: 'DELETE', session })).status, 204);
    assert.equal(await o.sessionPTTL(session.auth), -2);
  } finally { await barrier.release(); await inFlight; }
  assert.equal(await o.sessionPTTL(session.auth), -2);
  await envelope(await send(routes.current, { session }), 401, 'AUTH_UNAUTHENTICATED');
});
test('STORE-17 FR-012/014: reset storage and logs contain no plaintext old or fixed password', async t => {
  const o = await observe(t), f = fixtures(), target = f.storageResetTarget;
  if (!target) throw new Error('BLOCKED: independent storageResetTarget missing');
  const session = await preparedLogin(f.admin), since = new Date();
  const body = await envelope(await send(routes.reset.replace('{userId}', encodeURIComponent(target.id)), { method: 'POST', data: {}, session }), 200, 'OK');
  const state = await o.dumpOwnedAuthenticationState([target.account]);
  const logs = await o.readApplicationLogs({ since });
  noSecret(JSON.stringify(body) + state + logs, [target.password, 'Abc@123456', session.sid]);
  const hash = await o.readPasswordHash(target.account);
  assert.ok(typeof hash === 'string' && hash.length > 0);
});
