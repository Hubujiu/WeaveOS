import {execFileSync} from 'node:child_process';
import {appendFileSync, readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
const full = reason => ({backend:true, browser:true, product:true, reasons:[reason]});
const shaPattern = /^[a-f0-9]{40}$/;
const validPath = path => typeof path === 'string' && path.length > 0 &&
  !/[\x00-\x1f\x7f\\]/.test(path) && !path.startsWith('/') &&
  !path.split('/').some(part => part === '..' || part === '.' || part === '');

// Deliberately finite policy, not an inferred dependency graph or an AI decision.
export function classifyChanges(paths) {
  if (!Array.isArray(paths) || paths.length === 0) return full('unknown-or-empty-diff');
  const result = {backend:false, browser:false, product:false, reasons:[]};
  const reasons = new Set();
  for (const path of paths) {
    if (!validPath(path) || /(^|\/)AGENTS\.md$/.test(path)) return full('unknown-or-policy-path');
    if (['README.md','CONTRIBUTING.md','HANDOFF.md','SECURITY.md'].includes(path) ||
        (path.startsWith('docs/') && path.endsWith('.md'))) { reasons.add('prose'); continue; }
    if (/^services\/bff\/internal\/(flowgraph|flowcommands)\/[^/]+\.go$/.test(path)) {
      result.backend = true; reasons.add('reviewed-backend'); continue;
    }
    if (/^apps\/web\/(src|public)\/.+\.(ts|tsx|js|mjs|css|html|svg|png|jpg|webp)$/.test(path) ||
        path === 'apps/web/index.html' || /^tests\/(e2e|acceptance)\/.+\.tsx?$/.test(path)) {
      result.browser = true; result.product = true; reasons.add('frontend-and-product'); continue;
    }
    return full('unclassified-or-shared-path');
  }
  result.reasons = [...reasons].sort();
  return result;
}

export function planForCheckout({root=process.cwd(), eventName, event, sourceSha, forceFull=true}) {
  const bind = result => ({schemaVersion:1, sourceSha, ...result});
  if (forceFull !== false || eventName !== 'pull_request' || event?.pull_request?.base?.ref !== 'develop') return bind(full('full-event-or-release'));
  const pr = event.pull_request;
  if (![sourceSha,pr.base.sha,pr.head?.sha].every(s => typeof s === 'string' && shaPattern.test(s))) return bind(full('unreliable-candidate'));
  const git = (...args) => execFileSync('git',args,{cwd:root,encoding:'utf8',timeout:30000,maxBuffer:32*1024*1024,stdio:['ignore','pipe','pipe']});
  try {
    if (git('rev-parse','HEAD').trim() !== sourceSha) return bind(full('candidate-mismatch'));
    const parents = git('show','-s','--format=%P',sourceSha).trim().split(' ');
    if (sourceSha !== pr.head.sha && !(parents.length === 2 && parents[0] === pr.base.sha && parents[1] === pr.head.sha)) return bind(full('unbound-merge-candidate'));
    const base = git('merge-base',pr.base.sha,pr.head.sha).trim();
    const ranges = [[base,pr.head.sha]];
    // Include the actual tested merge tree, not just the topic's original diff.
    if (sourceSha !== pr.head.sha) ranges.push([pr.base.sha,sourceSha]);
    const paths = new Set();
    for (const [from,to] of ranges) {
      const raw = git('diff','--raw','-z','--no-renames','--no-ext-diff',from,to,'--');
      if (raw && !raw.endsWith('\0')) throw new Error('Incomplete diff');
      const pieces = raw ? raw.slice(0,-1).split('\0') : [];
      if (pieces.length % 2) throw new Error('Malformed diff');
      for (let i=0;i<pieces.length;i+=2) {
        const match = /^:(\d{6}) (\d{6}) [a-f0-9]+ [a-f0-9]+ ([AMD])$/.exec(pieces[i]);
        if (!match || ![match[1],match[2]].every(mode => ['000000','100644'].includes(mode))) return bind(full('special-file-or-mode'));
        paths.add(pieces[i+1]);
      }
    }
    return bind(classifyChanges([...paths]));
  } catch { return bind(full('diff-unavailable')); }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    let event;
    try { event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH,'utf8')); } catch { event = null; }
    const sourceSha = execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim();
    const plan = planForCheckout({eventName:process.env.GITHUB_EVENT_NAME,event,sourceSha,forceFull:process.env.FORCE_FULL !== 'false'});
    if (!shaPattern.test(sourceSha) || sourceSha !== process.env.GITHUB_SHA) throw new Error('Candidate mismatch');
    appendFileSync(process.env.GITHUB_OUTPUT, `plan=${JSON.stringify(plan)}\nbackend=${plan.backend}\nbrowser=${plan.browser}\nproduct=${plan.product}\n`);
    if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY,`CI selection for ${sourceSha}\n\n${JSON.stringify(plan)}\n\nUnselected jobs are not executed, not test passes.\n`);
    console.log(JSON.stringify(plan));
  } catch { console.error('CI routing failed; no selection published.'); process.exitCode=1; }
}
