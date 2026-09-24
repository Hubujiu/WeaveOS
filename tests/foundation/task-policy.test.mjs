import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseTask, validateTask, evaluateCleanup } from '../../scripts/task-policy.mjs';

// Independent oracle: foundation-contract.md FND-02 and the user's lifecycle rules.
const meta = {
  id: 'V010-099', title: 'Test task', branch: 'task/V010-099-example',
  worktree: '../WeaveOS-worktrees/V010-099', pr: 99, owner: 'Test role',
  dependsOn: [], allowedPaths: ['services/bff/'], deliveryState: 'ready',
};
function doc(overrides = {}, checked = true) {
  return `# V010-099\n<!-- task-meta\n${JSON.stringify({ ...meta, ...overrides })}\n-->\n## Scope\nExample\n## Sources\nUser requirement\n## Acceptance\n- [${checked ? 'x' : ' '}] Behavior independently verified.\n## Progress\nComplete\n## Handoff\nCheck remote main before cleanup.\n`.replaceAll('\n', '\n');
}
function state(overrides = {}) {
  return {
    id: 'V010-099', remoteDocument: doc(), fetchedRemote: true,
    repository: 'Hubujiu/WeaveOS', branch: meta.branch, branchTip: 'a'.repeat(40),
    worktreeClean: true, squashCommitInMain: true,
    pr: { number: 99, merged: true, base: 'main', repository: 'Hubujiu/WeaveOS', head: meta.branch, headSha: 'a'.repeat(40) },
    ...overrides,
  };
}
test('FND-02: parse a valid task and retain identity', () => {
  assert.equal(parseTask(doc())?.id, 'V010-099');
  assert.deepEqual(validateTask(doc()), []);
});
test('FND-02: malformed or duplicate metadata is rejected', () => {
  assert.throws(() => parseTask('no metadata'));
  assert.throws(() => parseTask('<!-- task-meta {broken} -->'));
  assert.throws(() => parseTask(doc() + doc()));
});
test('FND-02: a ready task with incomplete acceptance is invalid', () => {
  assert.ok(validateTask(doc({}, false)).some(x => x.includes('incomplete')));
});
test('FND-02: no checkboxes is not proof of completion', () => {
  assert.ok(validateTask(doc().replace(/- \[x\].*\n/, '')).some(x => x.includes('checkbox')));
});
test('FND-02: mismatched branch, worktree and unsafe paths are rejected', () => {
  for (const change of [{ branch: 'main' }, { worktree: '/' }, { allowedPaths: ['../../'] }]) {
    assert.ok(validateTask(doc(change)).length > 0);
  }
});
test('FND-02: accepted cannot be asserted before remote verification', () => {
  assert.ok(validateTask(doc({ deliveryState: 'accepted' })).length > 0);
});
test('FND-02: complete remote main document + merged PR permits cleanup despite squash ancestry', () => {
  assert.deepEqual(evaluateCleanup(state()), { allowed: true, reasons: [] });
});
for (const [label, change, reason] of [
  ['offline snapshot', { fetchedRemote: false }, 'REMOTE'],
  ['missing remote document', { remoteDocument: null }, 'DOCUMENT'],
  ['unfinished document', { remoteDocument: doc({}, false) }, 'DOCUMENT'],
  ['wrong task identity', { id: 'V010-098' }, 'IDENTITY'],
  ['unmerged PR', { pr: { ...state().pr, merged: false } }, 'PR'],
  ['wrong merge base', { pr: { ...state().pr, base: 'develop' } }, 'PR'],
  ['different repository', { pr: { ...state().pr, repository: 'other/repo' } }, 'PR'],
  ['other task branch', { branch: 'task/V010-098-other' }, 'IDENTITY'],
  ['post-merge commit', { branchTip: 'b'.repeat(40) }, 'TIP'],
  ['dirty worktree', { worktreeClean: false }, 'DIRTY'],
  ['squash not in remote main', { squashCommitInMain: false }, 'MAIN'],
]) {
  test(`FND-02: refuse cleanup for ${label}`, () => {
    const result = evaluateCleanup(state(change));
    assert.equal(result.allowed, false);
    assert.ok(result.reasons.some(x => x.includes(reason)), result.reasons.join('; '));
  });
}
