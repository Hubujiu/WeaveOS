import { evaluateAcceptance } from './task-policy.mjs';
// Remote acceptance is not a claim about any developer machine's worktree.
export function remoteCleanupDecision(input) {
  const { reasons } = evaluateAcceptance(input);
  if (input.squashCommitInMain !== true) reasons.push('[MAIN] verified squash commit required');
  if (typeof input.remoteTip !== 'string' || (input.remoteTip !== '' && input.remoteTip !== input.pr?.headSha)) reasons.push('[TIP] remote branch changed after acceptance');
  if (!/^[0-9a-f]{40}$/.test(input.pr?.headSha ?? '')) reasons.push('[TIP] accepted head is invalid');
  return { allowed: reasons.length === 0, alreadyDeleted: input.remoteTip === '', reasons };
}
export function resolveCleanupTip(localTip, remoteTip) {
  return remoteTip === '' || remoteTip === localTip ? localTip : '';
}
