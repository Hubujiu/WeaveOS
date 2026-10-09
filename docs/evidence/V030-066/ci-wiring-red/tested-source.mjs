import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const expected=[
 'nonCascadeRefusesRunningInstanceAndTaskCanStillContinue',
 'nonCascadeRemovesFinishedDefinitionButRetainsNativeHistory',
 'drainedCascadeRemovesOnlyTargetResourcesAndPreservesIndependentReceipts',
 'nativeCascadeRollbackRestoresDefinitionResourcesAndHistory',
 'deletingAllNativeVersionsKeepsOriginalReplayAcrossEngineReopen',
 'nativeCleanupAloneDoesNotReserveDeletedFlowIdentity',
];
test('V066 native deletion observations must execute and require six exact non-skipped reports in engine CI',()=>{
 const ci=readFileSync(new URL('../../.github/workflows/ci.yml',import.meta.url),'utf8');
 const run='bash services/workflow-engine/run-tests.sh clean -Dtest=RootNativeDeletionContractTest -Dworkflow.reports=ci-logs/native-deletion-reports test';
 assert.ok(ci.includes(run),'native deletion cases are not executed in CI');
 const gate=ci.indexOf('- name: Require six native deletion cases without skips');
 assert.ok(gate>ci.indexOf(run),'strict report gate must follow native execution');
 const end=ci.indexOf('\n      - name:',gate+10);const body=ci.slice(gate,end);
 assert.ok(body.includes('_java('));assert.ok(body.includes('services/workflow-engine/ci-logs/native-deletion-reports'));
 assert.ok(body.includes('org.weaveos.workflow.RootNativeDeletionContractTest'));
 for(const name of expected)assert.equal(body.split('"'+name+'"').length-1,1,'missing/duplicated native case '+name);
 assert.ok(!body.includes('continue-on-error')&&!body.includes('|| true'));
});
