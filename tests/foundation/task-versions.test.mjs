import assert from 'node:assert/strict';
import { test } from 'node:test';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawnSync } from 'node:child_process';
import { validateTask, evaluateAcceptance, evaluateCleanup } from '../../scripts/task-policy.mjs';
import { remoteCleanupDecision } from '../../scripts/remote-cleanup-policy.mjs';

// Independent oracle: the 2026-10-02 PRD/ADR009 B0 PLAN explicitly accepts
// V010/V030, preserves all lifecycle/scope guards and rejects malformed IDs.
// No existing V020 tasks/support were found on main or the read-only PR21 tree.
const source = fileURLToPath(new URL('../../', import.meta.url));
const scripts = ['task-policy.mjs', 'check-tasks.mjs', 'task.mjs', 'cleanup-remote-task.mjs', 'remote-cleanup-policy.mjs'];
const sha = 'a'.repeat(40);
function metadata(id = 'V030-099', overrides = {}) {
  return { id, title: 'Version fixture', branch: 'task/' + id + '-example', worktree: '../WeaveOS-worktrees/' + id,
    pr: 99, owner: 'Synthetic fixture', dependsOn: [], allowedPaths: ['safe/', 'docs/tasks/' + id + '.md'], deliveryState: 'ready', ...overrides };
}
function document(meta = metadata(), checked = true) {
  return ['# Version fixture', '<!-- task-meta', JSON.stringify(meta), '-->', '## Scope', 'Limited fixture', '## Sources', 'B0 PLAN',
    '## Acceptance', '- [' + (checked ? 'x' : ' ') + '] Independently verified', '## Progress', 'Synthetic', '## Handoff', 'Preserve guards', ''].join('\n');
}
function accepted(id = 'V030-099') {
  const meta = metadata(id);
  return { id, remoteDocument: document(meta), fetchedRemote: true, repository: 'Hubujiu/WeaveOS', branch: meta.branch, branchTip: sha,
    remoteTip: sha, worktreeClean: true, squashCommitInMain: true,
    pr: { number: 99, merged: true, base: 'main', repository: 'Hubujiu/WeaveOS', head: meta.branch, headSha: sha } };
}

test('B0: V010 and V030 metadata and mixed approved dependencies remain valid', () => {
  for (const id of ['V010-099', 'V030-099']) assert.deepEqual(validateTask(document(metadata(id))), []);
  assert.deepEqual(validateTask(document(metadata('V030-099', { dependsOn: ['V010-001', 'V030-001'] }))), []);
});

const badIDs = [null, 30, {}, [], ['V030-099'], { toString: null }, 'V020-099', 'V999-099', 'v030-099', 'V030-99', 'V030-0999',
  ' V030-099', 'V030-099 ', 'V030-099\n', 'V030-099\r\n', 'V030-099/../V010-099', 'V030-099[', 'V030-099.*',
  'V030-099$(touch injected)', 'V030-099;touch injected', 'V030-099`touch injected`', 'V030-099\\other', '__proto__',
  'V010-099\n', 'V010-099[', 'V010-099$(touch injected)'];
for (const [i, id] of badIDs.entries()) test('B0: malformed/injected ID is rejected without throwing #' + i, () => {
  const meta = metadata('V030-099', { id });
  let errors;
  assert.doesNotThrow(() => { errors = validateTask(document(meta)); });
  assert.ok(errors.includes('[TASK] invalid id'), JSON.stringify(errors));
});

test('B0: same task number in another version cannot satisfy branch/worktree identity', () => {
  assert.ok(validateTask(document(metadata('V030-099', { branch: 'task/V010-099-example' }))).includes('[TASK] invalid branch'));
  assert.ok(validateTask(document(metadata('V030-099', { worktree: '../WeaveOS-worktrees/V010-099' }))).includes('[TASK] invalid worktree'));
  for (const dependsOn of [['V020-001'], ['V030-099'], ['V010-001', 'V010-001']]) {
    assert.ok(validateTask(document(metadata('V030-099', { dependsOn }))).includes('[TASK] invalid dependsOn'));
  }
});

test('B0: V030 retains unsafe-path, incomplete, no-PR and premature-acceptance guards', () => {
  for (const allowedPaths of [['../other'], ['/etc'], ['safe/../other'], ['safe\\other'], ['safe:other']]) {
    assert.ok(validateTask(document(metadata('V030-099', { allowedPaths }))).includes('[TASK] invalid allowedPaths'));
  }
  assert.ok(validateTask(document(metadata(), false)).some(e => e.includes('incomplete')));
  assert.ok(validateTask(document(metadata('V030-099', { pr: null }))).some(e => e.includes('needs pr')));
  assert.ok(validateTask(document(metadata('V030-099', { deliveryState: 'accepted' }))).some(e => e.includes('invalid deliveryState')));
});

test('B0: complete matching V030 permits the same acceptance/cleanup decisions as V010', () => {
  for (const id of ['V010-099', 'V030-099']) {
    assert.deepEqual(evaluateAcceptance(accepted(id)), { accepted: true, reasons: [] });
    assert.deepEqual(evaluateCleanup(accepted(id)), { allowed: true, reasons: [] });
    assert.deepEqual(remoteCleanupDecision(accepted(id)), { allowed: true, alreadyDeleted: false, reasons: [] });
  }
});

for (const [label, change] of [
  ['offline', { fetchedRemote: false }], ['unfinished', { remoteDocument: document(metadata(), false) }],
  ['other version', { id: 'V010-099' }], ['unmerged', { pr: { ...accepted().pr, merged: false } }],
  ['wrong base', { pr: { ...accepted().pr, base: 'develop' } }], ['wrong repository', { pr: { ...accepted().pr, repository: 'other/repo' } }],
  ['wrong version PR', { pr: { ...accepted().pr, head: 'task/V010-099-example' } }],
]) test('B0: V030 acceptance and cleanup reject ' + label, () => {
  const state = { ...accepted(), ...change };
  assert.equal(evaluateAcceptance(state).accepted, false);
  assert.equal(evaluateCleanup(state).allowed, false);
  assert.equal(remoteCleanupDecision(state).allowed, false);
});
test('B0: V030 cleanup still rejects dirty, changed-tip and unverified squash states', () => {
  assert.equal(evaluateCleanup({ ...accepted(), worktreeClean: false }).allowed, false);
  assert.equal(evaluateCleanup({ ...accepted(), branchTip: 'b'.repeat(40) }).allowed, false);
  assert.equal(evaluateCleanup({ ...accepted(), squashCommitInMain: false }).allowed, false);
  assert.equal(remoteCleanupDecision({ ...accepted(), remoteTip: 'b'.repeat(40) }).allowed, false);
  assert.equal(remoteCleanupDecision({ ...accepted(), squashCommitInMain: false }).allowed, false);
});

function fixture(t) {
  const directory = mkdtempSync(join(tmpdir(), 'weaveos-b0-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const repo = join(directory, 'repo');
  mkdirSync(join(repo, 'scripts'), { recursive: true });
  mkdirSync(join(repo, 'docs/tasks'), { recursive: true });
  for (const name of scripts) copyFileSync(join(source, 'scripts', name), join(repo, 'scripts', name));
  return { directory, repo };
}
function put(repo, name, content) { const path = join(repo, name); mkdirSync(dirname(path), { recursive: true }); writeFileSync(path, content); }
function checker(repo, env = {}) {
  return spawnSync(process.execPath, [join(repo, 'scripts/check-tasks.mjs')], {
    cwd: repo, encoding: 'utf8', env: { ...process.env, GITHUB_EVENT_NAME: '', GITHUB_EVENT_PATH: '', ...env }, timeout: 10000,
  });
}
test('B0: task discovery validates V030 documents instead of silently skipping them', t => {
  const { repo } = fixture(t);
  put(repo, 'docs/tasks/V030-099.md', document(metadata('V030-099', { owner: '' })));
  const result = checker(repo);
  assert.equal(result.status, 1, result.stdout + result.stderr);
  assert.match(result.stderr, /V030-099\.md:.*missing owner/);
});
test('B0: discovered same-number metadata from another version is rejected', t => {
  const { repo } = fixture(t);
  put(repo, 'docs/tasks/V030-099.md', document(metadata('V010-099')));
  assert.match(checker(repo).stderr, /filename\/identity mismatch/);
});
test('B0: malformed and unsupported version-task filenames cannot disappear from discovery', t => {
  for (const name of ['V030-99.md', 'V030-0999.md', 'V020-099.md']) {
    const { repo } = fixture(t); put(repo, 'docs/tasks/' + name, document(metadata()));
    const result = checker(repo); assert.equal(result.status, 1, name + ': ' + result.stdout + result.stderr);
    assert.match(result.stderr, /invalid task filename/);
  }
});

function prFixture(t, id = 'V030-099', changed = 'safe/change.txt', branch = 'task/' + id + '-example') {
  const f = fixture(t);
  const git = args => execFileSync('git', args, { cwd: f.repo, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git(['init', '--initial-branch=fixture']);
  put(f.repo, 'docs/tasks/' + id + '.md', document(metadata(id)));
  git(['add', '.']); git(['-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-m', 'base']);
  const base = git(['rev-parse', 'HEAD']);
  put(f.repo, 'docs/tasks/' + id + '.md', document(metadata(id)) + '\nStage evidence\n');
  put(f.repo, changed, 'synthetic change\n');
  git(['add', '.']); git(['-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-m', 'change']);
  const event = join(f.directory, 'event.json');
  writeFileSync(event, JSON.stringify({ pull_request: { number: 99, head: { ref: branch }, base: { sha: base } } }));
  return { ...f, result: checker(f.repo, { GITHUB_EVENT_NAME: 'pull_request', GITHUB_EVENT_PATH: event }) };
}
test('B0: real Git diff verifies a valid V030 PR and an unchanged V010 PR rule', t => {
  for (const id of ['V010-099', 'V030-099']) { const { result } = prFixture(t, id); assert.equal(result.status, 0, result.stdout + result.stderr); }
});
test('B0: V030 PR scope rejects an unrelated changed path', t => {
  const { result } = prFixture(t, 'V030-099', 'outside/change.txt');
  assert.equal(result.status, 1); assert.match(result.stderr, /Outside task scope: outside\/change\.txt/);
});
test('B0: a V010 PR cannot consume a same-number V030 task document', t => {
  const { result } = prFixture(t, 'V030-099', 'safe/change.txt', 'task/V010-099-example');
  assert.equal(result.status, 1); assert.match(result.stderr, /PR task unreadable/);
});
test('B0: V030 branch syntax remains exact and cannot contain malformed slug or injection', t => {
  for (const branch of ['task/V030-099-example-', 'task/V030-099-two--words', 'task/V030-099-example\n', 'task/V030-099-example/../other']) {
    const { result } = prFixture(t, 'V030-099', 'safe/change.txt', branch);
    assert.equal(result.status, 1); assert.match(result.stderr, /PR must use task/);
  }
});

// Only the external Git/gh boundary is replaced here. The actual CLI/helper and
// all policy checks execute. Calls are recorded; no live remote is modified.
function externalFixture(t) {
  const f = fixture(t); const bin = join(f.directory, 'bin'); mkdirSync(bin);
  const meta = metadata();
  const state = { repo: f.repo, docs: { 'V030-099.md': document(meta) }, remoteTip: sha,
    pr: { number: 99, merged: true, base: { ref: 'main', repo: { full_name: 'Hubujiu/WeaveOS' } },
      head: { ref: meta.branch, sha, repo: { full_name: 'Hubujiu/WeaveOS' } }, merge_commit_sha: 'c'.repeat(40) } };
  put(f.directory, 'tool-state.json', JSON.stringify(state));
  const tool = ['#!' + process.execPath,
    "import fs from 'node:fs'; import path from 'node:path';",
    "const root=process.env.B0_FIXTURE_DIR; const state=JSON.parse(fs.readFileSync(path.join(root,'tool-state.json'),'utf8'));",
    "const program=path.basename(process.argv[1]); let args=process.argv.slice(2); fs.appendFileSync(path.join(root,'calls.jsonl'),JSON.stringify({program,args})+'\\n');",
    "if(program==='gh'&&args[0]==='api'){console.log(JSON.stringify(state.pr));process.exit(0);}",
    "if(args[0]==='-C')args=args.slice(2); let value;",
    "if(args[0]==='worktree'&&args[1]==='list')value='worktree '+state.repo+'\\nHEAD '+state.pr.head.sha;",
    "else if(args[0]==='remote'&&args[1]==='get-url')value='https://github.com/Hubujiu/WeaveOS.git';",
    "else if(args[0]==='fetch'||args[0]==='merge-base'||args[0]==='push')value='';",
    "else if(args[0]==='rev-parse')value=state.pr.head.sha;",
    "else if(args[0]==='show'&&args[1]==='-s')value='b'.repeat(40);",
    "else if(args[0]==='show'){const file=args[1].split(':docs/tasks/')[1];if(!(file in state.docs))process.exit(1);value=state.docs[file];}",
    "else if(args[0]==='branch'&&args[1]==='--list')value='';",
    "else if(args[0]==='ls-remote')value=state.remoteTip?state.remoteTip+'\\trefs/heads/'+state.pr.head.ref:'';",
    "else{console.error('unexpected fixture command');process.exit(70);}console.log(value);",
  ].join('\n');
  for (const program of ['git', 'gh']) writeFileSync(join(bin, program), tool, { mode: 0o700 });
  const env = { ...process.env, PATH: bin + delimiter + process.env.PATH, B0_FIXTURE_DIR: f.directory,
    GITHUB_REPOSITORY: 'Hubujiu/WeaveOS', PR_NUMBER: '99' };
  const run = (name, args = []) => spawnSync(process.execPath, [join(f.repo, 'scripts', name), ...args], { cwd: f.repo, env, encoding: 'utf8', timeout: 10000 });
  const calls = () => existsSync(join(f.directory, 'calls.jsonl')) ? readFileSync(join(f.directory, 'calls.jsonl'), 'utf8').trim().split('\n').filter(Boolean).map(JSON.parse) : [];
  return { ...f, state, run, calls };
}
test('B0: V030 CLI passes ID parsing then uses exact remote task path in start dry-run', t => {
  const f = externalFixture(t); f.state.remoteTip = '';
  f.state.docs['V030-099.md'] = document(metadata('V030-099', { deliveryState: 'planned', pr: null }), false);
  put(f.directory, 'tool-state.json', JSON.stringify(f.state));
  const result = f.run('task.mjs', ['start', 'V030-099']);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const action = JSON.parse(result.stdout); assert.equal(action.id, 'V030-099'); assert.equal(action.apply, false);
  assert.equal(action.worktree, resolve(f.repo, '../WeaveOS-worktrees/V030-099'));
  assert.ok(f.calls().some(c => c.args.includes(sha + ':docs/tasks/V030-099.md')));
  assert.ok(!f.calls().some(c => c.args.includes('push') || c.args.includes('add')));
});
test('B0: V030 ready task still cannot be restarted', t => {
  const f = externalFixture(t); const result = f.run('task.mjs', ['start', 'V030-099']);
  assert.equal(result.status, 1); assert.match(result.stderr, /Task already ready/);
  assert.ok(!f.calls().some(c => c.args.includes('push') || c.args.includes('add')));
});
test('B0: malformed/injected CLI IDs stop before any Git/gh execution', t => {
  const f = externalFixture(t);
  for (const id of badIDs.filter(id => typeof id === 'string')) {
    const result = f.run('task.mjs', ['start', id]); assert.equal(result.status, 1); assert.match(result.stderr, /Usage:/);
  }
  assert.deepEqual(f.calls(), []);
});
test('B0: remote helper recognizes V030 while retaining exact-head conditional deletion', t => {
  const f = externalFixture(t); const result = f.run('cleanup-remote-task.mjs');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const pushes = f.calls().filter(c => c.program === 'git' && c.args[0] === 'push');
  assert.deepEqual(pushes.map(c => c.args), [['push', '--force-with-lease=refs/heads/task/V030-099-example:' + sha, 'origin', ':refs/heads/task/V030-099-example']]);
});
test('B0: remote helper never deletes an unmerged or other-version task', t => {
  for (const change of [{ merged: false }, { head: { ref: 'task/V010-099-example', sha, repo: { full_name: 'Hubujiu/WeaveOS' } } }]) {
    const f = externalFixture(t); Object.assign(f.state.pr, change); put(f.directory, 'tool-state.json', JSON.stringify(f.state));
    const result = f.run('cleanup-remote-task.mjs'); assert.equal(result.status, 1);
    assert.ok(!f.calls().some(c => c.args.includes('push')));
  }
});
