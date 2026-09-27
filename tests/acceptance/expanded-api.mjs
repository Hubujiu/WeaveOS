import assert from 'node:assert/strict';
import { test } from 'node:test';
import { routes, password, unique, fixtures, send, envelope, noSecret, login, preparedLogin, invitation, register } from './http.mjs';

// Oracles: PRD FR-001..018; ADR-001 §3; ADR-002 §5.1/5.2/5.6.
// Endpoint/DTO bindings await V010-002 ratification. No API or storage mock.
test('HTTP-01 FR-007: anonymous current-session access is denied', async () => {
  await envelope(await send(routes.current), 401, 'AUTH_UNAUTHENTICATED');
});
for (const [label, headers] of [
  ['user', { 'X-User-Id': 'bootstrap-admin' }], ['role', { 'X-Role': 'ALL' }],
  ['tenant', { 'X-Tenant-Id': 'test-tenant', 'X-User-Id': 'admin' }],
  ['bearer', { Authorization: 'Bearer synthetic-not-a-session' }],
  ['trace', { 'X-Request-Id': 'bootstrap-admin', traceparent: '00-11111111111111111111111111111111-1111111111111111-01' }],
]) test(`HTTP-02 API-14/15: ${label} header cannot establish identity`, async () => {
  await envelope(await send(routes.current, { headers }), 401, 'AUTH_UNAUTHENTICATED');
});
test('HTTP-03 FR-001: unknown credentials are rejected without Session or disclosure', async () => {
  const r = await send(routes.login, { method: 'POST', data: { account: unique(), password } });
  const body = await envelope(r, 401, 'AUTH_INVALID_CREDENTIALS');
  assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
  noSecret(JSON.stringify(body), [password]);
});
for (const endpoint of ['login', 'register']) {
  const fields = endpoint === 'login' ? ['account', 'password'] : ['account', 'password', 'invitationCode'];
  for (const field of fields) for (const [label, value] of [['omitted', undefined], ['empty', '']]) {
    test(`HTTP-04 FR-001/003: ${endpoint} rejects ${label} ${field}`, async () => {
      const data = { account: unique(), password, invitationCode: 'synthetic-invalid-code', [field]: value };
      if (endpoint === 'login') delete data.invitationCode;
      const r = await send(routes[endpoint], { method: 'POST', data });
      const body = await envelope(r, 400, 'COMMON_VALIDATION_FAILED');
      assert.ok(Array.isArray(body.data?.violations) && body.data.violations.some(v => v.field === field && v.location === 'body'), 'required field violation');
      assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
      noSecret(JSON.stringify(body), [password]);
    });
  }
  for (const raw of ['{', '{"account":', 'not-json']) test(`HTTP-05 API-08: ${endpoint} rejects malformed JSON ${JSON.stringify(raw)}`, async () => {
    await envelope(await send(routes[endpoint], { method: 'POST', raw }), 400, 'COMMON_INVALID_ARGUMENT');
  });
  for (const [label, origin] of [['foreign', 'https://attacker.invalid'], ['opaque', 'null'], ['absent', null]]) {
    test(`HTTP-06 API-14: ${endpoint} rejects ${label} Origin without alternate CSRF proof`, async () => {
      const r = await send(routes[endpoint], { method: 'POST', data: { account: unique(), password, invitationCode: 'synthetic-invalid-code' }, headers: { Origin: origin } });
      assert.equal(r.status, 403);
      assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
    });
  }
  for (const media of ['text/plain', 'application/x-www-form-urlencoded', 'multipart/form-data; boundary=test']) {
    test(`HTTP-07 API-14: ${endpoint} refuses simple-request media ${media}`, async () => {
      const r = await send(routes[endpoint], { method: 'POST', raw: JSON.stringify({ account: unique(), password }), headers: { 'Content-Type': media } });
      assert.equal(r.status, 415);
      assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
    });
  }
}
test('HTTP-08 FR-003: invalid invitation is a registered 400 business error', async () => {
  const r = await register(unique(), 'synthetic-invalid-code');
  await envelope(r, 400, 'INVITATION_INVALID');
  assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
});
for (const [label, invalid] of [['uppercase', 'lowercase@123'], ['lowercase', 'UPPERCASE@123'], ['digit', 'NoDigits@Here'], ['special', 'NoSpecial123']]) {
  test(`HTTP-09 FR-018/003: missing ${label} is rejected without consuming invitation`, async () => {
    const code = await invitation(), account = unique();
    const body = await envelope(await register(account, code, invalid), 400, 'COMMON_VALIDATION_FAILED');
    assert.ok(body.data.violations.some(v => v.field === 'password' && v.code === 'AUTH_PASSWORD_POLICY_VIOLATION'));
    noSecret(JSON.stringify(body), [invalid]);
    await envelope(await register(account, code), 201, 'OK');
  });
}
test('HTTP-10 FR-018: Aa1! satisfies the policy without an invented length minimum', async () => {
  const account = unique(), code = await invitation();
  await envelope(await register(account, code, 'Aa1!'), 201, 'OK');
  await login({ account, password: 'Aa1!' });
});
test('HTTP-11 FR-003/017: registration creates an account but no Session', async () => {
  const account = unique(), code = await invitation(), r = await register(account, code);
  const body = await envelope(r, 201, 'OK');
  assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
  assert.ok(r.headers.get('location'), 'API-05 Location');
  assert.equal(typeof body.data.id, 'string', 'API-11 opaque ID');
  noSecret(JSON.stringify(body), [password, code]);
  await envelope(await send(routes.current), 401, 'AUTH_UNAUTHENTICATED');
  await login({ account, password });
});
test('HTTP-12 FR-003: used invitation returns 409 and creates no second account', async () => {
  const code = await invitation();
  await envelope(await register(unique(), code), 201, 'OK');
  const rejected = unique();
  await envelope(await register(rejected, code), 409, 'INVITATION_ALREADY_USED');
  await envelope(await send(routes.login, { method: 'POST', data: { account: rejected, password } }), 401, 'AUTH_INVALID_CREDENTIALS');
});
test('HTTP-13 FR-003: eight concurrent consumers yield one account and seven conflicts', async () => {
  const code = await invitation(), accounts = Array.from({ length: 8 }, unique);
  const responses = await Promise.all(accounts.map(account => register(account, code)));
  assert.deepEqual(responses.map(r => r.status).sort(), [201, 409, 409, 409, 409, 409, 409, 409]);
  for (let i = 0; i < responses.length; i++) {
    const won = responses[i].status === 201;
    await envelope(responses[i], won ? 201 : 409, won ? 'OK' : 'INVITATION_ALREADY_USED');
    await envelope(await send(routes.login, { method: 'POST', data: { account: accounts[i], password } }), won ? 201 : 401, won ? 'OK' : 'AUTH_INVALID_CREDENTIALS');
  }
});
test('HTTP-14 FR-002/003: duplicate account fails and leaves invitation usable', async () => {
  const code = await invitation();
  await envelope(await register(fixtures().user.account, code), 409, 'USER_ACCOUNT_ALREADY_EXISTS');
  await envelope(await register(unique(), code), 201, 'OK');
});
test('HTTP-15 FR-002/003: concurrent duplicate account leaves loser invitation usable', async () => {
  const codes = [await invitation(), await invitation()], account = unique();
  const rs = await Promise.all(codes.map(code => register(account, code)));
  assert.deepEqual(rs.map(r => r.status).sort(), [201, 409]);
  const loser = rs.findIndex(r => r.status === 409);
  await envelope(rs[loser], 409, 'USER_ACCOUNT_ALREADY_EXISTS');
  await envelope(await register(unique(), codes[loser]), 201, 'OK');
});
test('HTTP-16 FR-001: unknown account and wrong password share public error', async () => {
  const bodies = [];
  for (const account of [fixtures().user.account, unique()]) {
    const r = await send(routes.login, { method: 'POST', data: { account, password: 'Wrong@Test123' } });
    bodies.push(await envelope(r, 401, 'AUTH_INVALID_CREDENTIALS'));
    assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
  }
  assert.ok(bodies[0].message === bodies[1].message, 'do not reveal account existence');
});
test('HTTP-17 FR-010: Disabled account cannot obtain a Session', async () => {
  const f = fixtures().disabled;
  const r = await send(routes.login, { method: 'POST', data: { account: f.account, password: f.password } });
  await envelope(r, 401, 'AUTH_INVALID_CREDENTIALS');
  assert.ok(r.headers.get('set-cookie') === null, 'must not issue Cookie; contents withheld');
});
test('HTTP-18 FR-013: repeated failures do not lock account or return 429', async () => {
  const user = fixtures().user;
  for (let i = 0; i < 12; i++) await envelope(await send(routes.login, { method: 'POST', data: { account: user.account, password: 'Wrong@Test123' } }), 401, 'AUTH_INVALID_CREDENTIALS');
  await login(user);
});
test('HTTP-19 FR-005/008: login restores the same user identity', async () => {
  const user = fixtures().user, session = await login(user);
  for (let i = 0; i < 2; i++) {
    const body = await envelope(await send(routes.current, { session }), 200, 'OK');
    assert.ok(body.data.id === user.id && body.data.account === user.account, 'current user matches fixture');
    noSecret(JSON.stringify(body), [user.password, session.sid]);
  }
});
test('HTTP-20 ADR-001: login and successful activity synchronize one-hour Cookie expiry', async () => {
  const session = await login(fixtures().user);
  const lifetime = cookie => {
    const maxAge = cookie?.match(/;\s*Max-Age=(-?\d+)(?:;|$)/i);
    if (maxAge) assert.equal(Number(maxAge[1]), 3600);
    else {
      const expires = cookie?.match(/;\s*Expires=([^;]+)/i);
      assert.ok(expires && Math.abs(Date.parse(expires[1]) - Date.now() - 3600000) < 5000, 'Cookie expires in one hour');
    }
  };
  lifetime(session.setCookie);
  const r = await send(routes.current, { session });
  await envelope(r, 200, 'OK');
  lifetime(r.headers.getSetCookie().find(c => c.startsWith(session.auth.split('=')[0] + '=')));
});
test('HTTP-21 FR-009: logout is empty 204, clears Cookie and rejects replay', async () => {
  const session = await preparedLogin(fixtures().user), r = await send(routes.current, { method: 'DELETE', session });
  assert.equal(r.status, 204);
  assert.equal(await r.text(), '');
  const cleared = r.headers.getSetCookie().find(c => c.startsWith(session.auth.split('=')[0] + '='));
  const expires = cleared?.match(/Expires=([^;]+)/i);
  const maxAge = cleared?.match(/;\s*Max-Age=(-?\d+)(?:;|$)/i);
  assert.ok(cleared && (maxAge ? Number(maxAge[1]) <= 0 : expires && Date.parse(expires[1]) < Date.now()), 'clear auth cookie');
  await envelope(await send(routes.current, { session }), 401, 'AUTH_UNAUTHENTICATED');
});
test('HTTP-22 FR-009: concurrent logout and reads do not resurrect Session', async () => {
  const session = await preparedLogin(fixtures().user);
  const rs = await Promise.all([send(routes.current, { method: 'DELETE', session }), ...Array.from({ length: 12 }, () => send(routes.current, { session }))]);
  assert.equal(rs[0].status, 204);
  for (const r of rs.slice(1)) assert.ok([200, 401].includes(r.status), 'reads may precede or follow revocation');
  await envelope(await send(routes.current, { session }), 401, 'AUTH_UNAUTHENTICATED');
});
test('HTTP-23 ADR-001: login never adopts attacker-selected Session ID', async () => {
  const first = await preparedLogin(fixtures().user), name = first.auth.split('=')[0], attacker = 'synthetic-attacker-session';
  const session = await login(fixtures().user, { Cookie: `${name}=${attacker}` });
  assert.ok(session.sid !== attacker);
  await envelope(await send(routes.current, { headers: { Cookie: `${name}=${attacker}` } }), 401, 'AUTH_UNAUTHENTICATED');
});
test('HTTP-24 ADR-001: query and Authorization cannot replace Cookie authentication', async () => {
  const session = await preparedLogin(fixtures().user);
  for (const headers of [{ Authorization: `Bearer ${session.sid}` }, { 'X-Session-Id': session.sid }]) await envelope(await send(routes.current, { headers }), 401, 'AUTH_UNAUTHENTICATED');
  // Never put an actual reusable Session in a URL/access log.
  await envelope(await send(`${routes.current}?sessionId=synthetic-query-session`), 401, 'AUTH_UNAUTHENTICATED');
});
for (const capability of ['invitations', 'reset']) {
  test(`HTTP-25 FR-012/016: anonymous ${capability} denied`, async () => {
    const path = capability === 'reset' ? routes.reset.replace('{userId}', '00000000-0000-4000-8000-000000000001') : routes.invitations;
    await envelope(await send(path, { method: 'POST', data: {} }), 401, 'AUTH_UNAUTHENTICATED');
  });
  test(`HTTP-26 FR-012/016: member cannot ${capability} with forged ALL headers`, async () => {
    const f = fixtures(), session = await preparedLogin(f.user);
    const path = capability === 'reset' ? routes.reset.replace('{userId}', encodeURIComponent(f.resetTarget.id)) : routes.invitations;
    await envelope(await send(path, { method: 'POST', data: {}, session, headers: { 'X-Role': 'ALL', 'X-User-Id': f.admin.id } }), 403, 'COMMON_PERMISSION_DENIED');
    if (capability === 'reset') await login(f.resetTarget);
  });
}
test('HTTP-27 FR-016: Bootstrap generates a usable single-use invitation', async () => {
  const session = await preparedLogin(fixtures().admin), r = await send(routes.invitations, { method: 'POST', data: {}, session });
  const body = await envelope(r, 201, 'OK');
  assert.ok(r.headers.get('location'));
  assert.ok(typeof body.data.invitationCode === 'string' && body.data.invitationCode.length > 0);
  await envelope(await register(unique(), body.data.invitationCode), 201, 'OK');
  await envelope(await register(unique(), body.data.invitationCode), 409, 'INVITATION_ALREADY_USED');
});
test('HTTP-28 FR-012: Bootstrap resets password without disclosure; old password fails', async () => {
  const f = fixtures(), session = await preparedLogin(f.admin);
  assert.ok(f.resetTarget.password !== 'Abc@123456', 'fixture starts with a different synthetic password');
  const body = await envelope(await send(routes.reset.replace('{userId}', encodeURIComponent(f.resetTarget.id)), { method: 'POST', data: {}, session }), 200, 'OK');
  noSecret(JSON.stringify(body), ['Abc@123456', f.resetTarget.password]);
  await envelope(await send(routes.login, { method: 'POST', data: { account: f.resetTarget.account, password: f.resetTarget.password } }), 401, 'AUTH_INVALID_CREDENTIALS');
  await login({ account: f.resetTarget.account, password: 'Abc@123456' });
});
for (const [label, origin] of [['foreign', 'https://attacker.invalid'], ['opaque', 'null']]) test(`HTTP-29 API-14: ${label} Origin rejects writes even with valid token`, async () => {
  const session = await preparedLogin(fixtures().admin);
  assert.equal((await send(routes.invitations, { method: 'POST', data: {}, session, headers: { Origin: origin } })).status, 403);
});
test('HTTP-30 API-14: authenticated write without source or CSRF proof denied', async () => {
  const admin = await preparedLogin(fixtures().admin);
  assert.equal((await send(routes.invitations, { method: 'POST', data: {}, session: { cookie: admin.auth }, headers: { Origin: null, Referer: null } })).status, 403);
});
test('HTTP-31 API-14: foreign-Origin logout rejected without revoking Session', async () => {
  const session = await preparedLogin(fixtures().user);
  assert.equal((await send(routes.current, { method: 'DELETE', session, headers: { Origin: 'https://attacker.invalid' } })).status, 403);
  await envelope(await send(routes.current, { session }), 200, 'OK');
});
test('HTTP-32 API-15: request IDs are generated rather than trusted from caller', async () => {
  const ids = [];
  for (let i = 0; i < 2; i++) {
    const r = await send(routes.current, { headers: { 'X-Request-Id': 'caller-selected-id' } });
    await envelope(r, 401, 'AUTH_UNAUTHENTICATED');
    ids.push(r.headers.get('x-request-id'));
  }
  assert.ok(ids[0] !== 'caller-selected-id' && ids[1] !== 'caller-selected-id' && ids[0] !== ids[1]);
});

for (const endpoint of ['login', 'register']) {
  for (const field of endpoint === 'login' ? ['account', 'password'] : ['account', 'password', 'invitationCode']) {
    for (const [label, value] of [['null', null], ['number', 123], ['object', {}], ['array', []], ['boolean', true]]) {
      test(`HTTP-33 API-09: ${endpoint} rejects ${label} ${field}`, async () => {
        const data = { account: unique(), password, ...(endpoint === 'register' ? { invitationCode: 'synthetic-invalid-code' } : {}), [field]: value };
        const body = await envelope(await send(routes[endpoint], { method: 'POST', data }), 400, 'COMMON_VALIDATION_FAILED');
        assert.ok(Array.isArray(body.data?.violations) && body.data.violations.some(v => v.field === field && v.location === 'body'));
      });
    }
  }
}
for (const field of ['isBootstrapAdmin', 'role', 'permissions', 'id', 'unexpectedField']) {
  test(`HTTP-34 API compatibility: registration rejects write to ${field}`, async () => {
    const code = await invitation(), account = unique();
    const r = await send(routes.register, { method: 'POST', data: { account, password, invitationCode: code, [field]: field === 'isBootstrapAdmin' ? true : 'ALL' } });
    await envelope(r, 400, 'COMMON_VALIDATION_FAILED');
    await envelope(await register(account, code), 201, 'OK');
    const session = await preparedLogin({ account, password });
    await envelope(await send(routes.invitations, { method: 'POST', data: {}, session }), 403, 'COMMON_PERMISSION_DENIED');
  });
}
test('HTTP-35 API-05: HEAD current-session is bodyless even when anonymous', async () => {
  const r = await send(routes.current, { method: 'HEAD' });
  assert.equal(r.status, 401);
  assert.equal(await r.text(), '');
  assert.ok(r.headers.get('www-authenticate') === 'Session realm="enterprise-management-system"');
});
