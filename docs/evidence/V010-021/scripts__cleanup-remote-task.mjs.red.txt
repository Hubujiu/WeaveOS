// This workflow helper only deletes an accepted remote ref; it never touches developer worktrees.
import { execFileSync } from 'node:child_process';
import { remoteCleanupDecision } from './remote-cleanup-policy.mjs';
const repository = 'Hubujiu/WeaveOS';
function run(program, args) {
  return execFileSync(program, args, { encoding: 'utf8', timeout: 60000, stdio: ['ignore', 'pipe', 'pipe'] }).trim();
}
try {
  const number = process.env.PR_NUMBER;
  if (process.env.GITHUB_REPOSITORY !== repository || !/^[1-9][0-9]*$/.test(number ?? '')) throw new Error('Explicit matching repository and PR number required');
  const pr = JSON.parse(run('gh', ['api', `repos/${repository}/pulls/${number}`]));
  const match = /^task\/(V010-\d{3})-[a-z0-9-]+$/.exec(pr.head.ref);
  if (!match || pr.head.repo?.full_name !== repository || pr.base.repo.full_name !== repository || !pr.merged || pr.base.ref !== 'main') throw new Error('Only merged same-repository task PRs to main are eligible');
  run('git', ['fetch', '--prune', 'origin', '+refs/heads/main:refs/remotes/origin/main']);
  const main = run('git', ['rev-parse', 'refs/remotes/origin/main']);
  const remoteDocument = run('git', ['show', `${main}:docs/tasks/${match[1]}.md`]);
  const remoteTip = run('git', ['ls-remote', '--heads', 'origin', pr.head.ref]).split(/\s+/)[0];
  let squashCommitInMain = false;
  if (pr.merge_commit_sha) {
    try {
      run('git', ['merge-base', '--is-ancestor', pr.merge_commit_sha, main]);
      squashCommitInMain = run('git', ['show', '-s', '--format=%P', pr.merge_commit_sha]).split(' ').length === 1;
    } catch { /* no verified squash-style commit in main */ }
  }
  const decision = remoteCleanupDecision({ id: match[1], repository, fetchedRemote: true, remoteDocument, remoteTip, squashCommitInMain,
    pr: { number: pr.number, merged: pr.merged, base: pr.base.ref, repository: pr.base.repo.full_name, head: pr.head.ref, headSha: pr.head.sha } });
  console.log(JSON.stringify({ task: match[1], main, pr: pr.number, remoteBranch: pr.head.ref, ...decision }, null, 2));
  if (!decision.allowed) throw new Error(decision.reasons.join('; '));
  if (!decision.alreadyDeleted) run('git', ['push', `--force-with-lease=refs/heads/${pr.head.ref}:${pr.head.sha}`, 'origin', `:refs/heads/${pr.head.ref}`]);
  console.log('Remote task ref removed or already absent. Local worktrees remain subject to separate clean/head checks.');
} catch (error) {
  console.error(`[REMOTE CLEANUP BLOCKED] ${error.message}`);
  process.exitCode = 1;
}
