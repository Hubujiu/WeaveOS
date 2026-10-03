import assert from 'node:assert/strict';
import { test } from 'node:test';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { evaluateCleanup, parseTask, taskIDFromFilename } from '../../scripts/task-policy.mjs';

// Independent oracle: PRD 2026-10-03T02:21:51.820Z and ADR009
// 2026-10-03T02:21:59.281Z, B5a.9, plus root's precise legacy-file contract.
// Compatibility is limited to the original PR21 bytes, registered and referenced
// by a valid V010-020 canonical task. It does not create a task or loosen cleanup.
const source = fileURLToPath(new URL('../../', import.meta.url));
const canonicalPath = 'docs/tasks/V010-020.md';
const auxiliaryPath = 'docs/tasks/V010-020-PLAN.md';
const auxiliaryFilename = 'V010-020-PLAN.md';
const approvedPlanSha256 = '1342f819a09946377744acc13eb26cf2eccf1a1a17d51adf00efb72dbe8b0227';
const approvedPlan = readFileSync(join(source, auxiliaryPath));

function metadata(overrides = {}) {
  return {
    id: 'V010-020', title: 'Synthetic canonical task',
    branch: 'task/V010-020-admin-shell', worktree: '../WeaveOS-worktrees/V010-020',
    pr: 21, owner: 'Synthetic fixture', dependsOn: [],
    allowedPaths: [canonicalPath, auxiliaryPath, 'safe/'], deliveryState: 'ready',
    ...overrides,
  };
}
function document(overrides = {}, reference = '[Approved legacy PLAN](V010-020-PLAN.md)') {
  return [
    '# Synthetic canonical task', '<!-- task-meta', JSON.stringify(metadata(overrides)), '-->',
    '## Scope', reference, '## Sources', 'Frozen B5a.9 compatibility contract',
    '## Acceptance', '- [x] Synthetic fixture is complete',
    '## Progress', 'No live repository or remote mutation',
    '## Handoff', 'Preserve exact metadata, scope and cleanup gates', '',
  ].join('\n');
}
function fixture(t, { main = document(), name = auxiliaryFilename, plan = approvedPlan } = {}) {
  const repo = mkdtempSync(join(tmpdir(), 'weaveos-b5-plan-fixture-'));
  t.after(() => rmSync(repo, { recursive: true, force: true }));
  mkdirSync(join(repo, 'scripts'), { recursive: true });
  mkdirSync(join(repo, 'docs/tasks'), { recursive: true });
  for (const file of ['check-tasks.mjs', 'task-policy.mjs']) {
    copyFileSync(join(source, 'scripts', file), join(repo, 'scripts', file));
  }
  if (main !== null) writeFileSync(join(repo, canonicalPath), main);
  if (plan !== null) writeFileSync(join(repo, 'docs/tasks', name), plan);
  return repo;
}
function checker(repo) {
  const result = spawnSync(process.execPath, [join(repo, 'scripts/check-tasks.mjs')], {
    cwd: repo, encoding: 'utf8', timeout: 10000,
    env: { ...process.env, GITHUB_EVENT_NAME: '', GITHUB_EVENT_PATH: '' },
  });
  assert.equal(result.error, undefined, 'Checker process must execute: ' + result.error);
  assert.equal(result.signal, null, 'Checker must finish normally');
  return result;
}
function reject(result) {
  assert.equal(result.status, 1, result.stdout + result.stderr);
  assert.notEqual(result.stderr.trim(), '', 'Rejection must identify a policy error');
}

test('B5a.9: fixture preserves the exact approved PR21 legacy PLAN bytes', () => {
  assert.equal(createHash('sha256').update(approvedPlan).digest('hex'), approvedPlanSha256);
});
test('B5a.9: a valid canonical task without an auxiliary document still passes', t => {
  const result = checker(fixture(t, { plan: null }));
  assert.equal(result.status, 0, result.stdout + result.stderr);
});
test('B5a.9: exact registered and referenced PR21 auxiliary PLAN passes discovery', t => {
  const result = checker(fixture(t));
  assert.equal(result.status, 0, result.stdout + result.stderr);
});

for (const [label, plan] of [
  ['replaced contents', '# Replaced PLAN\n'],
  ['one-byte addition', Buffer.concat([approvedPlan, Buffer.from('\n')])],
  ['forged task metadata', Buffer.concat([approvedPlan, Buffer.from('\n<!-- task-meta\n' + JSON.stringify(metadata()) + '\n-->\n')])],
  ['a complete second task', document({ id: 'V030-020', branch: 'task/V030-020-forged', worktree: '../WeaveOS-worktrees/V030-020' })],
]) {
  test('B5a.9: known auxiliary path rejects ' + label, t => {
    reject(checker(fixture(t, { plan })));
  });
}

for (const [label, main] of [
  ['missing canonical task', null],
  ['missing canonical metadata', '# Canonical heading without metadata\n'],
  ['duplicate canonical metadata', document() + document()],
  ['invalid canonical JSON', document().replace('"id":"V010-020"', '"id":not-json')],
  ['other task identity', document({ id: 'V030-020', branch: 'task/V030-020-example', worktree: '../WeaveOS-worktrees/V030-020' })],
  ['missing canonical owner', document({ owner: '' })],
  ['wrong branch identity', document({ branch: 'task/V010-021-example' })],
  ['wrong worktree identity', document({ worktree: '../WeaveOS-worktrees/V010-021' })],
  ['invalid canonical scope', document({ allowedPaths: ['../outside'] })],
  ['empty canonical scope', document({ allowedPaths: [] })],
  ['unregistered auxiliary scope', document({ allowedPaths: [canonicalPath, 'safe/'] })],
  ['a scope path that merely shares the prefix', document({ allowedPaths: [canonicalPath, auxiliaryPath + '-extra'] })],
  ['missing canonical reference', document({}, 'No auxiliary reference registered in this text')],
  ['a reference to a different PLAN', document({}, '[Other PLAN](V010-021-PLAN.md)')],
  ['incomplete ready canonical task', document().replace('- [x]', '- [ ]')],
  ['a prematurely accepted canonical task', document({ deliveryState: 'accepted' })],
]) {
  test('B5a.9: auxiliary compatibility rejects ' + label, t => {
    reject(checker(fixture(t, { main })));
  });
}

for (const name of [
  'V010-021-PLAN.md', 'V030-020-PLAN.md', 'V020-020-PLAN.md', 'V999-020-PLAN.md',
  'V010-20-PLAN.md', 'V010-0020-PLAN.md', 'v010-020-PLAN.md',
  'V010-020-plan.md', 'V010-020-PLAN-copy.md', 'V010-020-PLAN.md.md',
]) {
  test('B5a.9: legacy exception does not admit similar or malformed name ' + name, t => {
    reject(checker(fixture(t, { name })));
  });
}
test('B5a.9: an approved auxiliary does not conceal an additional fake task document', t => {
  const repo = fixture(t);
  writeFileSync(join(repo, 'docs/tasks/V030-099-PLAN.md'), approvedPlan);
  reject(checker(repo));
});

test('B5a.9: auxiliary filename and content remain unsuitable as a canonical task', () => {
  assert.equal(taskIDFromFilename(auxiliaryFilename), null);
  assert.throws(() => parseTask(approvedPlan.toString('utf8')), /exactly one task-meta/);
});
function cleanupState(overrides = {}) {
  const task = metadata();
  return {
    id: task.id, remoteDocument: document(), fetchedRemote: true,
    repository: 'Hubujiu/WeaveOS', branch: task.branch, branchTip: 'a'.repeat(40),
    worktreeClean: true, squashCommitInMain: true,
    pr: { number: 21, merged: true, base: 'main', repository: 'Hubujiu/WeaveOS', head: task.branch, headSha: 'a'.repeat(40) },
    ...overrides,
  };
}
test('B5a.9: auxiliary compatibility cannot supply cleanup completion or a fake task identity', () => {
  assert.deepEqual(evaluateCleanup(cleanupState()), { allowed: true, reasons: [] });
  for (const state of [
    cleanupState({ remoteDocument: approvedPlan.toString('utf8') }),
    cleanupState({ id: 'V010-020-PLAN' }),
    cleanupState({ fetchedRemote: false }),
    cleanupState({ worktreeClean: false }),
    cleanupState({ branchTip: 'b'.repeat(40) }),
    cleanupState({ squashCommitInMain: false }),
    cleanupState({ pr: { ...cleanupState().pr, merged: false } }),
  ]) {
    assert.equal(evaluateCleanup(state).allowed, false, JSON.stringify(state));
  }
});
