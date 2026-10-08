import assert from 'node:assert/strict';
import {existsSync,readFileSync} from 'node:fs';
import test from 'node:test';
const root=new URL('../../',import.meta.url);
const costFiles=['root_workflow_read_cost_test.go','root_workflow_evidence_cost_test.go'];
test('large comparative experiments require the explicit Go cost build constraint',()=>{
 for(const name of costFiles){
  const source=readFileSync(new URL(`services/bff/internal/apprecordservice/${name}`,root),'utf8');
  // The leading compiler directive, unlike an arbitrary source token, is the
  // actual language-level rule excluding this file from ordinary compilation.
  assert.equal(source.split('\n')[0],'//go:build weaveos_cost',name);
 }
});
test('comparative performance remains runnable in a separate manual workflow',()=>{
 const file=new URL('.github/workflows/performance.yml',root);
 const workflow=existsSync(file)?JSON.parse(readFileSync(file,'utf8')):{};
 assert.deepEqual(workflow.on,{workflow_dispatch:{}},'explicit manually requested measurements');
 assert.deepEqual(workflow.permissions,{contents:'read'});
 // V049 adds a shared preflight; the invariant is the exact measurement set.
 const measurements=Object.entries(workflow.jobs??{}).filter(([,job])=>job.steps?.some(step=>step.id==='measure')).map(([name])=>name).sort();
 assert.deepEqual(measurements,['go-cost','java-cost']);
 const go=workflow.jobs['go-cost'],java=workflow.jobs['java-cost'];
 assert.equal(go['runs-on'],'ubuntu-24.04');assert.equal(java['runs-on'],'ubuntu-24.04');
 const measurement=go.steps.find(step=>step.id==='measure');
 assert.equal(measurement['working-directory'],'services/bff');
 assert.equal(measurement.run,"go test -tags=weaveos_cost -p 1 -count=1 -json -run '^(TestRootWorkflowReadSyntheticAccessPaths|TestRootEvidenceCostComparison)$' ./internal/apprecordservice > cost-results.jsonl");
 const engine=java.steps.find(step=>step.id==='measure');
 assert.equal(engine.run,'bash services/workflow-engine/run-tests.sh -Dtest=RootNativeHistoryCostTest -Dworkflow.reports=ci-logs/native-history-cost-reports test');
 for(const job of [go,java]){
  assert.ok(job.steps.some(step=>step.id==='verify'),'measurement must reject missing or skipped cases');
  assert.ok(job.steps.some(step=>step.uses?.startsWith('actions/upload-artifact@')&&step.if==='always()'),'retain results even on failure');
 }
});
