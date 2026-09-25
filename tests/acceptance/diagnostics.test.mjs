import assert from 'node:assert/strict';
import { inspect } from 'node:util';
import { test } from 'node:test';
import { envelope } from './http.mjs';

// Tests the assertion helper itself, not a mocked authentication implementation.
// Oracle: root AGENTS §8 and PRD §5 prohibit credentials in test/log artifacts.
test('DIAGNOSTICS-01: invalid error data is rejected without exposing its payload', async () => {
  const marker = 'synthetic-sensitive-payload-sentinel';
  const response = new Response(JSON.stringify({ code: 'AUTH_INVALID_CREDENTIALS', message: 'failure', data: { password: marker }, meta: null }), {
    status: 401, headers: { 'content-type': 'application/json', 'x-request-id': 'synthetic-request', 'www-authenticate': 'Session realm="enterprise-management-system"' },
  });
  let failure;
  try { await envelope(response, 401, 'AUTH_INVALID_CREDENTIALS'); } catch (error) { failure = error; }
  assert.ok(failure, 'malformed error data must fail the contract assertion');
  assert.ok(!inspect(failure).includes(marker), 'assertion diagnostics must not disclose a sensitive response');
});
