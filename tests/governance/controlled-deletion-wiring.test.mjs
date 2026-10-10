import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
const ci=readFileSync(new URL('../../.github/workflows/ci.yml',import.meta.url),'utf8');
function step(name){const start=ci.indexOf(`      - name: ${name}\n`);assert.ok(start>=0,`missing required CI step ${name}`);const end=ci.indexOf('\n      - name:',start+1);return ci.slice(start,end<0?undefined:end);}
test('controlled deletion runs six real suites with exact no-skip reports',()=>{
 const s=step('Root controlled deletion and explicit identity schema');
 for(const name of ['RootFlowDeletionRegistryTest','RootFlowDeletionSchemaTest','RootFlowDeletionRuntimeSchemaTest','RootFlowDeletionCatalogReferenceTest','RootFlowDeletionGrpcIT','RootFlowDeletionRuntimeIT'])assert.ok(s.includes(name),`missing ${name}`);
 assert.match(s,/run-tests\.sh clean/);assert.match(s,/-Dtest="\$suite"/);assert.match(s,/-Dworkflow\.reports="ci-logs\/controlled-deletion\/\$suite"/);assert.match(s,/from rpc_ci_gate import _java/);assert.match(s,/_java\(/);assert.doesNotMatch(s,/continue-on-error|\|\| true/);
 for(const name of ['realTwoConnectionAdmissionArbitration(String)[1]','realTwoConnectionAdmissionArbitration(String)[4]','publicExecuteOnDeletionGuardFunctionIsRejected','responseLossAfterCommitRecoversSameOperation','formalRuntimeDeletesWithLeastPrivilegeThenRestartsAndReplaysWithoutResurrection'])assert.ok(s.includes(name),`missing strict ${name}`);
});
test('Go deletion CI covers strict client and actual authenticated runtime interop',()=>{
 const s=step('Root Go deletion client and actual runtime interop');assert.match(s,/go -C services\/bff test -race -count=1 -json/);assert.match(s,/\^TestRootDeletionClient/);assert.match(s,/run-deletion-rpc-interop\.sh/);assert.match(s,/from rpc_ci_gate import _go/);
 for(const name of ['TestRootDeletionClientBoundResultCopiesRequestAndPreservesEarlierDeadline','TestRootDeletionClientInvalidInputsNeverCallTransport','TestRootDeletionClientRejectsEveryUnboundOrInvalidReceipt','TestRootDeletionClientLookupBranchesAndTransportFailure','TestRootGoJavaActualDeletionRuntimeInterop'])assert.ok(s.includes(name),`missing exact ${name}`);
 const path=new URL('../../services/workflow-engine/run-deletion-rpc-interop.sh',import.meta.url);assert.ok(existsSync(path),'missing actual private runner');const runner=readFileSync(path,'utf8');assert.match(runner,/--internal/);assert.match(runner,/RootFlowDeletionInteropFixtureMain/);assert.match(runner,/workflow_deletion_integration/);assert.match(runner,/WEAVEOS_V067_RPC_TARGET="127\.0\.0\.1:\$rpc_port"/);const dockerRuns=runner.replace(/\\\n/g,' ').split('\n').filter(line=>line.startsWith('docker run ')).join('\n');assert.equal(dockerRuns.split('\n').length,3);assert.doesNotMatch(dockerRuns,/--publish|(?:^|\s)-p(?:\s|[0-9])/);assert.match(runner,/trap cleanup EXIT/);
});
