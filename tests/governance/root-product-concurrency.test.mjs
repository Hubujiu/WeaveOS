import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const workflow = readFileSync(new URL('../../.github/workflows/acceptance.yml', import.meta.url), 'utf8');
const concurrency = workflow.match(/^concurrency:\n((?: {2}.+\n)+)/m)?.[1] ?? '';

test('product acceptance shares a workflow-specific PR group with isolated non-PR runs', () => {
  assert.ok(concurrency, 'missing top-level product concurrency policy');
  assert.equal(concurrency.match(/^  group: (.+)$/m)?.[1],
    "product-${{ github.workflow }}-${{ github.event_name == 'pull_request' && format('pr-{0}', github.event.pull_request.number) || format('run-{0}', github.run_id) }}");
});

test('product cancellation is enabled only for pull-request replacement runs', () => {
  assert.equal(concurrency.match(/^  cancel-in-progress: (.+)$/m)?.[1],
    "${{ github.event_name == 'pull_request' }}");
});

test('product concurrency keeps the existing full verification stages unconditional', () => {
  const stages = [
    ['Migrations, HTTPS and real product acceptance', 'node infra/acceptance/run.mjs'],
    ['Runtime configuration, crypto, failure handling and dependency security', 'run: node --test infra/runtime/rollback-security.test.mjs'],
    ['Build once and verify immutable runtime, restore, rollback and browsers', 'node infra/runtime/run.mjs'],
    ['Automatic deployment migrations with isolated real PostgreSQL', 'node --test infra/server/deploy/migrate.test.mjs infra/server/deploy/personnel-upgrade.test.mjs'],
    ['Verify deployment packaging from the same accepted OCI artifacts', 'node infra/server/deploy/package.mjs .work/runtime/export/BUILD.json .work/deployment'],
  ];
  let prior = -1;
  for (const [name, command] of stages) {
    const start = workflow.indexOf('      - name: ' + name + '\n');
    assert.ok(start > prior, `missing/reordered stage: ${name}`);
    const next = workflow.indexOf('\n      - ', start + 1);
    const block = workflow.slice(start, next < 0 ? undefined : next);
    assert.ok(block.includes(command), `verification command missing: ${name}`);
    assert.doesNotMatch(block, /^\s+if:/m, `original full stage became conditional: ${name}`);
    prior = start;
  }
  assert.match(workflow, /^permissions:\n  contents: read\njobs:/m);
});
