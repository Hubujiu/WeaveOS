// Requires a real clone, Git, authenticated gh, and network. No secrets are printed.
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseTask, validateTask, evaluateCleanup, evaluateAcceptance } from './task-policy.mjs';
const [command, id, ...options] = process.argv.slice(2);
const apply = options.length === 1 && options[0] === '--apply';
const repository = 'Hubujiu/WeaveOS';
function run(program, args, cwd = process.cwd()) {
  return execFileSync(program, args, { cwd, encoding: 'utf8', timeout: 60000, stdio: ['ignore', 'pipe', 'pipe'] }).trim();
}
try {
  if (!['start', 'status', 'cleanup'].includes(command) || !/^V010-\d{3}$/.test(id ?? '') || (options.length && !apply)) throw new Error('Usage: node scripts/task.mjs start|status|cleanup V010-NNN [--apply]');
  const first = run('git', ['worktree', 'list', '--porcelain']).split('\n')[0];
  if (!first.startsWith('worktree ')) throw new Error('Primary worktree not found');
  const root = first.slice('worktree '.length);
  const git = args => run('git', ['-C', root, ...args], root);
  const origin = git(['remote', 'get-url', 'origin']);
  if (!['https://github.com/Hubujiu/WeaveOS.git', 'https://github.com/Hubujiu/WeaveOS', 'git@github.com:Hubujiu/WeaveOS.git'].includes(origin)) throw new Error('origin must be Hubujiu/WeaveOS');
  git(['fetch', '--prune', 'origin', '+refs/heads/main:refs/remotes/origin/main']);
  const main = git(['rev-parse', 'refs/remotes/origin/main']);
  const text = git(['show', `${main}:docs/tasks/${id}.md`]);
  const errors = validateTask(text);
  if (errors.length) throw new Error(errors.join('; '));
  const task = parseTask(text);
  if (task.id !== id) throw new Error('Task identity mismatch');
  const worktree = resolve(root, task.worktree);
  if (command === 'start') {
    if (task.deliveryState === 'ready') throw new Error('Task already ready; inspect remote PR instead of restarting');
    for (const dependency of task.dependsOn) {
      const parentText = git(['show', `${main}:docs/tasks/${dependency}.md`]);
      const parent = parseTask(parentText);
      if (validateTask(parentText).length || parent.deliveryState !== 'ready' || !parent.pr) throw new Error(`Dependency not ready: ${dependency}`);
      const pr = JSON.parse(run('gh', ['api', `repos/${repository}/pulls/${parent.pr}`], root));
      if (!pr.merged || pr.base.ref !== 'main' || pr.head.ref !== parent.branch) throw new Error(`Dependency not merged: ${dependency}`);
    }
    if (existsSync(worktree) || git(['branch', '--list', task.branch]) || git(['ls-remote', '--heads', 'origin', task.branch])) throw new Error('Task already has a branch/worktree; resume it, do not create another');
    console.log(JSON.stringify({ action: 'start', id, branch: task.branch, worktree, base: main, apply }, null, 2));
    if (apply) {
      git(['worktree', 'add', '-b', task.branch, worktree, main]);
      git(['push', '-u', 'origin', task.branch]);
      console.log('Created task worktree/branch. Update its owner, progress and next action before implementation.');
    }
  } else {
    if (!task.pr) throw new Error('No associated PR; task is not accepted');
    const pr = JSON.parse(run('gh', ['api', `repos/${repository}/pulls/${task.pr}`], root));
    if (command === 'status') {
      const result = evaluateAcceptance({ id, remoteDocument: text, fetchedRemote: true, repository,
        pr: { number: pr.number, merged: pr.merged, base: pr.base.ref, repository: pr.base.repo.full_name, head: pr.head.ref } });
      console.log(JSON.stringify({ id, remoteMain: main, pr: pr.html_url, worktreeExists: existsSync(worktree), ...result }, null, 2));
      if (!result.accepted) process.exitCode = 1;
    } else {
      if (!existsSync(worktree)) throw new Error('Worktree absent; verify cleanup history rather than deleting another directory');
      const localTip = run('git', ['rev-parse', 'HEAD'], worktree);
      const localBranch = run('git', ['branch', '--show-current'], worktree);
      const remoteTip = git(['ls-remote', '--heads', 'origin', task.branch]).split(/\s+/)[0];
      const clean = run('git', ['status', '--porcelain', '--untracked-files=all', '--ignored'], worktree) === '';
      let inMain = false;
      if (pr.merge_commit_sha) {
        try {
          git(['merge-base', '--is-ancestor', pr.merge_commit_sha, main]);
          inMain = git(['show', '-s', '--format=%P', pr.merge_commit_sha]).split(' ').length === 1;
        } catch { /* not a verified squash commit on remote main */ }
      }
      const result = evaluateCleanup({ id, remoteDocument: text, fetchedRemote: true, repository,
        branch: localBranch, branchTip: localTip === remoteTip ? localTip : '',
        worktreeClean: clean, squashCommitInMain: inMain,
        pr: { number: pr.number, merged: pr.merged, base: pr.base.ref, repository: pr.base.repo.full_name, head: pr.head.ref, headSha: pr.head.sha } });
      console.log(JSON.stringify({ id, remoteMain: main, pr: pr.html_url, worktree, ...result, apply: command === 'cleanup' && apply }, null, 2));
      if (!result.allowed) process.exitCode = 1;
      else if (command === 'cleanup' && apply) {
        // Conditional remote deletion prevents losing commits added after inspection.
        if (run('git', ['status', '--porcelain', '--untracked-files=all', '--ignored'], worktree)) throw new Error('Worktree changed during verification');
        git(['push', `--force-with-lease=refs/heads/${task.branch}:${pr.head.sha}`, 'origin', `:refs/heads/${task.branch}`]);
        git(['worktree', 'remove', worktree]);
        git(['branch', '-D', task.branch]);
        console.log('Accepted task branch and clean worktree removed; retained main task/evidence documents.');
      }
    }
  }
} catch (error) {
  console.error(`[TASK BLOCKED] ${error.message}`);
  process.exitCode = 1;
}
