import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runPreflight} from '../../scripts/preflight.mjs';

// User-approved execution privacy constraint, 2026-10-12: lint locally,
// never enable Redocly's outbound telemetry when inspecting our contract.
for (const inherited of [undefined, 'on', 'off']) {
  test(`preflight disables Redocly telemetry with inherited setting ${inherited}`, () => {
    const env = {PATH: '/fixture/bin', GITHUB_SHA: 'a'.repeat(40)};
    if (inherited !== undefined) env.REDOCLY_TELEMETRY = inherited;
    const before = {...env};
    const calls = [];
    runPreflight({root: '/fixture/repo', env, execute: step => {
      calls.push(step);
      return {status: 0, stdout: '', stderr: '', signal: null};
    }});
    const lint = calls.find(step => step.id === 'contract-lint');
    assert.equal(lint.env.REDOCLY_TELEMETRY, 'off');
    assert.equal(lint.env.PATH, env.PATH);
    assert.equal(lint.env.GITHUB_SHA, env.GITHUB_SHA);
    assert.deepEqual(lint.args, ['exec', 'redocly', 'lint', 'contracts/openapi/openapi.json']);
    assert.deepEqual(env, before, 'do not mutate the caller environment');
  });
}

test('direct CI contract lint explicitly disables telemetry without removing validation', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/ci.yml', import.meta.url), 'utf8');
  const step = workflow.split('      - name: Authentication OpenAPI 3.2.1 contract\n')[1]?.split('      - ')[0];
  assert.ok(step, 'required contract-validation step exists');
  assert.match(step, /REDOCLY_TELEMETRY:\s*['"]off['"]/);
  assert.match(step, /node --test contracts\/\*\.test\.mjs/);
  assert.match(step, /pnpm exec redocly lint contracts\/openapi\/openapi\.json/);
});
