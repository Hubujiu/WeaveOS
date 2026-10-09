import {appendFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
const ci = {governance:'always',go:'backend',browser:'browser','workflow-engine':'backend','workflow-rpc':'backend','execution-rpc':'backend','workflow-recovery':'backend','workflow-actions':'backend','workflow-formal-runtime':'backend','workflow-schema-cli':'backend','workflow-backup':'backend'};
const product = {components:'product','component-coverage':'product',product:'product'};
export function verifySelection({kind,needs,sourceSha}) {
  const map = kind === 'ci' ? ci : kind === 'product' ? product : null;
  if (!map || !needs || JSON.stringify(Object.keys(needs).sort()) !== JSON.stringify(['preflight',...Object.keys(map)].sort())) throw new Error('Unexpected selection job set');
  if (needs.preflight.result !== 'success') throw new Error('Preflight did not succeed');
  const plan = JSON.parse(needs.preflight.outputs?.plan ?? '');
  if (plan.schemaVersion !== 1 || !/^[a-f0-9]{40}$/.test(sourceSha) || plan.sourceSha !== sourceSha ||
      !['backend','browser','product'].every(k => typeof plan[k] === 'boolean') ||
      plan.browser !== plan.product || !Array.isArray(plan.reasons) || !plan.reasons.length ||
      !plan.reasons.every(r => typeof r === 'string' && /^[a-z-]+$/.test(r))) throw new Error('Invalid or stale selection');
  for (const [job,lane] of Object.entries(map)) {
    const selected = lane === 'always' || plan[lane];
    const result = needs[job]?.result;
    if (selected ? result !== 'success' : !['success','skipped'].includes(result)) throw new Error(`Selection did not complete: ${job}`);
  }
  return plan;
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const plan=verifySelection({kind:process.env.SELECTION_KIND,needs:JSON.parse(process.env.NEEDS_JSON),sourceSha:process.env.GITHUB_SHA});
    console.log('All selected checks succeeded; unselected checks were not counted as tests.');
    if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY,`Selection gate passed for ${plan.sourceSha}\n\n${JSON.stringify(plan)}\n`);
  } catch (error) { console.error(`CI selection gate failed: ${error.message}`); process.exitCode=1; }
}
