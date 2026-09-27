import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

// Oracle: ADR-002 §5.8 and V010-002 acceptance require a fixed 3.2.1 validator
// and contract checks on the delivered PR head, not merely a local run.
const root = new URL('../', import.meta.url);
const pkg = JSON.parse(readFileSync(new URL('package.json', root), 'utf8'));
const ci = readFileSync(new URL('.github/workflows/ci.yml', root), 'utf8');

test('API-CI-01: validator version is pinned in the frozen workspace lock', () => {
  assert.equal(pkg.devDependencies?.['@redocly/cli'], '2.54.2');
  const lock = readFileSync(new URL('pnpm-lock.yaml', root), 'utf8');
  assert.ok(/@redocly\/cli:\s*\n\s*specifier: 2\.54\.2\s*\n\s*version: 2\.54\.2/.test(lock), 'frozen lock pins the same validator');
});

test('API-CI-02: PR CI executes contract tests and 3.2.1 lint', () => {
  assert.ok(ci.includes('node --test contracts/*.test.mjs'), 'PR CI runs contract assertions');
  assert.ok(ci.includes('pnpm exec redocly lint contracts/openapi/openapi.json'), 'PR CI lints the 3.2.1 document');
});
