import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { parseTask, validateTask } from './task-policy.mjs';
const root = fileURLToPath(new URL('..', import.meta.url));
const directory = new URL('../docs/tasks/', import.meta.url);
const errors = [];
for (const file of readdirSync(directory).filter(name => /^V010-\d{3}\.md$/.test(name))) {
  const text = readFileSync(new URL(file, directory), 'utf8');
  errors.push(...validateTask(text).map(error => `${file}: ${error}`));
  try { if (parseTask(text).id !== file.slice(0, -3)) errors.push(`${file}: filename/identity mismatch`); } catch { /* reported above */ }
}
// PRs must update their own handoff document, including documentation-only tasks.
if (process.env.GITHUB_EVENT_NAME === 'pull_request') {
  const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
  const pr = event.pull_request;
  const match = /^task\/(V010-\d{3})-[a-z0-9-]+$/.exec(pr.head.ref);
  if (!match) errors.push('PR must use task/V010-NNN-topic branch');
  else {
    const path = `docs/tasks/${match[1]}.md`;
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
