import test from 'node:test';
import assert from 'node:assert/strict';
import {verifyComponentArtifacts} from '../../scripts/verify-component-artifacts.mjs';
const sha='a'.repeat(40),other='b'.repeat(40);
const clone=x=>structuredClone(x);
const raw=i=>({errors:[],stats:{expected:1,unexpected:0,flaky:0,skipped:0},suites:[{title:'a.spec.ts',file:'a.spec.ts',specs:[{file:'a.spec.ts',title:'case '+i,tests:[{projectName:'chromium',expectedStatus:'passed',status:'expected',results:[{status:'passed',retry:0}]}]}]}]});
const baseline={sourceSha:sha,report:{errors:[],suites:[1,2,3,4].flatMap(i=>raw(i).suites)}};
for(const s of baseline.report.suites)for(const t of s.specs[0].tests)t.results=[];
const shards=[1,2,3,4].map(i=>({sourceSha:sha,shardIndex:i,shardTotal:4,report:raw(i)}));
const fixture=()=>({baseline:clone(baseline),shards:clone(shards),expectedSha:sha,allowedSkips:[]});
test('PRD V049: same candidate and all four distinct shards produce actual component stats',()=>{
 const r=verifyComponentArtifacts(fixture());
 assert.equal(r.sourceSha,sha);assert.equal(r.scope,'components');
 assert.deepEqual(r.stats,{expected:4,unexpected:0,flaky:0,skipped:0});
 assert.deepEqual(r.coverage,{total:4,passed:4,skipped:0,identities:[1,2,3,4].map(i=>JSON.stringify(['chromium','a.spec.ts',['a.spec.ts','case '+i]]))});
});
for(const [name,change] of [
 ['stale baseline',f=>f.baseline.sourceSha=other],
 ['stale shard',f=>f.shards[1].sourceSha=other],
 ['missing fourth shard',f=>f.shards.pop()],
 ['duplicate shard slot with otherwise distinct cases',f=>f.shards[1].shardIndex=1],
 ['wrong total',f=>f.shards[2].shardTotal=3],
 ['out of range shard',f=>f.shards[2].shardIndex=5],
 ['unbound source',f=>f.expectedSha=''],
 ['failure in a report',f=>f.shards[0].report.suites[0].specs[0].tests[0].results[0].status='failed'],
 ['missing identity behind correct total',f=>{f.shards[3].report.suites[0].specs[0].title='case 1';}],
 ])test('PRD V049: refuse '+name,()=>{const f=fixture();change(f);assert.throws(()=>verifyComponentArtifacts(f));});
