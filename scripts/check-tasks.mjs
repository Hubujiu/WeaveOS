import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { parseTask, validateTask, taskIDFromBranch, taskIDFromFilename } from './task-policy.mjs';
const root = fileURLToPath(new URL('..', import.meta.url));
const directory = new URL('../docs/tasks/', import.meta.url);
const errors = [];
// Root-approved PR21 auxiliary document. It is never a canonical task and
// remains subject to its canonical owner's exact scope and reference.
function approvedLegacyPlan(file) {
  if (file !== 'V010-020-PLAN.md') return false;
  try {
    const bytes = readFileSync(new URL(file, directory));
    if (createHash('sha256').update(bytes).digest('hex') !== '1342f819a09946377744acc13eb26cf2eccf1a1a17d51adf00efb72dbe8b0227') return false;
    const canonical = readFileSync(new URL('V010-020.md', directory), 'utf8');
    if (validateTask(canonical).length) return false;
    const task = parseTask(canonical);
    return task.id === 'V010-020' && task.allowedPaths.includes('docs/tasks/V010-020-PLAN.md')
      && /\[[^\]\n]+\]\(V010-020-PLAN\.md\)/.test(canonical);
  } catch { return false; }
}
// Version-looking documents must be checked or rejected, not silently skipped.
// The shared parser accepts only the explicitly supported V010/V030 families.
for (const file of readdirSync(directory).filter(name => /^[Vv][0-9]/.test(name) && name.endsWith('.md'))) {
  const id = taskIDFromFilename(file);
  if (!id) { if (!approvedLegacyPlan(file)) errors.push(`${file}: invalid task filename`); continue; }
  const text = readFileSync(new URL(file, directory), 'utf8');
  errors.push(...validateTask(text).map(error => `${file}: ${error}`));
  try { if (parseTask(text).id !== id) errors.push(`${file}: filename/identity mismatch`); } catch { /* reported above */ }
}
// PRs must update their own handoff document, including documentation-only tasks.
if (process.env.GITHUB_EVENT_NAME === 'pull_request') {
  const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
  const pr = event.pull_request;
  const id = taskIDFromBranch(pr.head.ref);
  if (!id) errors.push('PR must use task/V010-NNN-topic or task/V030-NNN-topic branch');
  else {
    const path = `docs/tasks/${id}.md`;
    try {
      const task = parseTask(readFileSync(resolve(root, path), 'utf8'));
      if (task.branch !== pr.head.ref || task.pr !== pr.number) errors.push('PR/task identity mismatch');
      const paths = execFileSync('git', ['diff', '--name-only', `${pr.base.sha}...HEAD`], { cwd: root, encoding: 'utf8' }).trim().split('\n');
      if (!paths.includes(path)) errors.push(`PR must update ${path}`);
      for (const changed of paths) if (!task.allowedPaths.some(allowed => allowed.endsWith('/') ? changed.startsWith(allowed) : changed === allowed)) errors.push(`Outside task scope: ${changed}`);
    } catch (error) { errors.push(`PR task unreadable: ${error.message}`); }
  }
}
if (errors.length) { console.error(errors.join('\n')); process.exitCode = 1; }
else console.log('Task documents and current PR scope are structurally valid.');
