import assert from 'node:assert/strict';
import { test } from 'node:test';
import { releaseErrors } from '../../scripts/release-policy.mjs';
// Requirement: no release while a mandatory automation or manual acceptance remains undone.
const valid = { version: '0.1.0', checks: [
  { id: 'AUTO-01', mode: 'automated', status: 'passed', evidence: 'https://github.com/Hubujiu/WeaveOS/actions/runs/123', owner: 'QA' },
  { id: 'MAN-01', mode: 'manual', status: 'passed', evidence: 'docs/evidence/review.md', owner: 'Reviewer' },
] };
test('release: all mandatory evidence present is structurally ready', () => assert.deepEqual(releaseErrors(valid), []));
test('release: a missing or empty matrix is not a pass', () => {
  for (const x of [null, {}, { version: '0.1.0', checks: [] }]) assert.ok(releaseErrors(x).length);
});
test('release: pending, skipped, failed and blocked are never passed', () => {
  for (const status of ['pending', 'skipped', 'failed', 'blocked']) {
    assert.ok(releaseErrors({ ...valid, checks: [{ ...valid.checks[0], status }] }).length);
  }
});
test('release: missing evidence, owner or duplicate IDs are rejected', () => {
  for (const checks of [[{ ...valid.checks[0], evidence: '' }], [{ ...valid.checks[0], owner: '' }], [valid.checks[0], valid.checks[0]]]) {
    assert.ok(releaseErrors({ ...valid, checks }).length);
  }
});
