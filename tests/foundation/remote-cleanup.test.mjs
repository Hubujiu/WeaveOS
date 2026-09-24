import assert from 'node:assert/strict';
import { test } from 'node:test';
import { remoteCleanupDecision, resolveCleanupTip } from '../../scripts/remote-cleanup-policy.mjs';
const sha = 'a'.repeat(40);
const metadata = { id: 'V010-099', title: 'Test', branch: 'task/V010-099-test', worktree: '../WeaveOS-worktrees/V010-099', pr: 99, owner: 'Test', dependsOn: [], allowedPaths: ['tests/'], deliveryState: 'ready' };
const remoteDocument = `# Test\n<!-- task-meta\n${JSON.stringify(metadata)}\n-->\n## Scope\nTest\n## Sources\nUser lifecycle requirement\n## Acceptance\n- [x] Complete\n## Progress\nDone\n## Handoff\nRemote only; local worktree separately checked.\n`;
const state = { id: metadata.id, remoteDocument, fetchedRemote: true, repository: 'Hubujiu/WeaveOS', remoteTip: sha, squashCommitInMain: true, pr: { number: 99, merged: true, base: 'main', repository: 'Hubujiu/WeaveOS', head: metadata.branch, headSha: sha } };
test('remote cleanup: matching accepted remote branch can be removed', () => assert.deepEqual(remoteCleanupDecision(state), { allowed: true, alreadyDeleted: false, reasons: [] }));
test('remote cleanup: no remote branch remains a successful idempotent state', () => assert.deepEqual(remoteCleanupDecision({ ...state, remoteTip: '' }), { allowed: true, alreadyDeleted: true, reasons: [] }));
for (const [name, changes] of [
  ['unmerged', { pr: { ...state.pr, merged: false } }],
  ['unfinished document', { remoteDocument: remoteDocument.replace('- [x]', '- [ ]') }],
  ['new remote commit', { remoteTip: 'b'.repeat(40) }],
  ['unverified squash', { squashCommitInMain: false }],
  ['offline snapshot', { fetchedRemote: false }],
]) test(`remote cleanup: refuse ${name}`, () => assert.equal(remoteCleanupDecision({ ...state, ...changes }).allowed, false));
test('local cleanup: removed remote does not erase valid local identity', () => assert.equal(resolveCleanupTip(sha, ''), sha));
test('local cleanup: matching refs retain identity', () => assert.equal(resolveCleanupTip(sha, sha), sha));
test('local cleanup: divergent refs cannot hide new work', () => assert.equal(resolveCleanupTip(sha, 'b'.repeat(40)), ''));
