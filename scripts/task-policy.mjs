// Offline validation only. Remote reads and Git operations belong to the caller.
const idPattern = /^V010-\d{3}$/;
const nonempty = value => typeof value === 'string' && value.trim().length > 0;
export function parseTask(text) {
  if (typeof text !== 'string') throw new Error('task document must be text');
  const blocks = [...text.matchAll(/<!--\s*task-meta\s+([\s\S]*?)-->/g)];
  if (blocks.length !== 1) throw new Error('exactly one task-meta block required');
  const value = JSON.parse(blocks[0][1]);
  if (!value || Array.isArray(value) || typeof value !== 'object') throw new Error('metadata must be an object');
  return value;
}
export function validateTask(text) {
  const errors = [];
  let task;
  try { task = parseTask(text); } catch (error) { return [`[TASK] ${error.message}`]; }
  if (!idPattern.test(task.id ?? '')) errors.push('[TASK] invalid id');
  for (const key of ['title', 'owner']) if (!nonempty(task[key])) errors.push(`[TASK] missing ${key}`);
  if (typeof task.branch !== 'string' || !new RegExp(`^task/${task.id}-[a-z0-9]+(?:-[a-z0-9]+)*$`).test(task.branch)) errors.push('[TASK] invalid branch');
  if (task.worktree !== `../WeaveOS-worktrees/${task.id}`) errors.push('[TASK] invalid worktree');
  if (task.pr !== null && (!Number.isInteger(task.pr) || task.pr < 1)) errors.push('[TASK] invalid pr');
  if (!['planned', 'in_progress', 'blocked', 'ready'].includes(task.deliveryState)) errors.push('[TASK] invalid deliveryState; acceptance is a remote fact');
  if (!Array.isArray(task.dependsOn) || task.dependsOn.some(id => !idPattern.test(id) || id === task.id) || new Set(task.dependsOn).size !== task.dependsOn.length) errors.push('[TASK] invalid dependsOn');
  if (!Array.isArray(task.allowedPaths) || !task.allowedPaths.length || task.allowedPaths.some(path => !nonempty(path) || path.startsWith('/') || path.includes('..') || path.includes('\\') || path.includes(':'))) errors.push('[TASK] invalid allowedPaths');
  for (const section of ['Scope', 'Sources', 'Acceptance', 'Progress', 'Handoff']) if (!text.includes(`\n## ${section}\n`)) errors.push(`[TASK] missing ${section}`);
  const acceptance = text.match(/\n## Acceptance\n([\s\S]*?)(?=\n## |$)/)?.[1] ?? '';
  if (!/^- \[[ xX]\] \S/m.test(acceptance)) errors.push('[TASK] Acceptance needs at least one checkbox');
  if (task.deliveryState === 'ready') {
    if (/^- \[ \] /m.test(text)) errors.push('[TASK] ready task has incomplete items');
    if (!Number.isInteger(task.pr) || task.pr < 1) errors.push('[TASK] ready task needs pr');
  }
  return errors;
}
export function evaluateCleanup(input) {
  const reasons = [];
  if (input.fetchedRemote !== true) reasons.push('[REMOTE] fresh remote main read required');
  let task;
  try {
    if (validateTask(input.remoteDocument).length) throw new Error('invalid task');
    task = parseTask(input.remoteDocument);
    if (task.deliveryState !== 'ready') throw new Error('not complete');
  } catch { reasons.push('[DOCUMENT] remote task missing or incomplete'); }
  if (task && (task.id !== input.id || task.branch !== input.branch)) reasons.push('[IDENTITY] task or branch mismatch');
  const pr = input.pr;
  if (!pr || pr.merged !== true || pr.base !== 'main' || pr.repository !== input.repository || pr.head !== input.branch || (task && pr.number !== task.pr)) reasons.push('[PR] matching merged PR to this repository main required');
  if (!/^[0-9a-f]{40}$/.test(input.branchTip ?? '') || pr?.headSha !== input.branchTip) reasons.push('[TIP] branch differs from accepted PR head');
  if (input.worktreeClean !== true) reasons.push('[DIRTY] inspect all local work before cleanup');
  if (input.squashCommitInMain !== true) reasons.push('[MAIN] squash commit not verified in remote main');
  return { allowed: reasons.length === 0, reasons };
}

// Acceptance is independent of whether an already-cleaned task still has a worktree.
export function evaluateAcceptance(input) {
  const reasons = [];
  if (input.fetchedRemote !== true) reasons.push('[REMOTE] fresh remote main read required');
  let task;
  try {
    if (validateTask(input.remoteDocument).length) throw new Error('invalid task');
    task = parseTask(input.remoteDocument);
    if (task.deliveryState !== 'ready' || task.id !== input.id) throw new Error('identity or completion mismatch');
  } catch { reasons.push('[DOCUMENT] matching completed remote task required'); }
  const pr = input.pr;
  if (!pr || pr.merged !== true || pr.base !== 'main' || pr.repository !== input.repository || (task && (pr.number !== task.pr || pr.head !== task.branch))) reasons.push('[PR] matching merged PR required');
  return { accepted: reasons.length === 0, reasons };
}
