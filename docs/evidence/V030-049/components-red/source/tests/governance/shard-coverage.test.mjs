import test from 'node:test';
import assert from 'node:assert/strict';
import {verifyShards} from '../../scripts/verify-shards.mjs';
const clone=x=>structuredClone(x);
const identity=(project,file,title)=>JSON.stringify([project,file,[file,'business  flow',title]]);
function report(cases){
 return {errors:[],stats:{expected:cases.filter(c=>c.status!=='skipped').length,unexpected:0,flaky:0,skipped:cases.filter(c=>c.status==='skipped').length},suites:cases.map(c=>({title:c.file,file:c.file,suites:[{title:'business  flow',suites:[],specs:[{title:c.title,file:c.file,tests:[{projectName:c.project,expectedStatus:c.status==='skipped'?'skipped':'passed',status:c.status==='skipped'?'skipped':'expected',results:[{status:c.status??'passed',retry:0}]}]}]}],specs:[]}))};
}
const cases=[{project:'chromium',file:'a.spec.ts',title:'save  exact fields'}, {project:'firefox',file:'a.spec.ts',title:'save  exact fields'}, {project:'webkit',file:'b.spec.ts',title:'restore',status:'skipped'}];
const baseline=report(cases);
// --list contains identities; it has not run the test results.
for(const suite of baseline.suites)for(const spec of suite.suites[0].specs)for(const t of spec.tests)t.results=[];
const allowedSkips=[identity('webkit','b.spec.ts','restore')];
const parts=[report(cases.slice(0,1)),report(cases.slice(1))];
function verify(reports=parts,list=baseline,skips=allowedSkips){return verifyShards({baseline:list,reports,allowedSkips:skips});}
test('PRD V049: actual disjoint union matches all projects, full titles and files',()=>{
 assert.deepEqual(verify(),{total:3,passed:2,skipped:1,identities:cases.map(c=>identity(c.project,c.file,c.title)).sort()});
});
test('PRD V049: original permitted capability skip may become a real pass',()=>{
 const good=clone(cases);delete good[2].status;
 assert.equal(verify([report(good)]).passed,3);
});
for(const [name,mutate] of [
 ['missing',rs=>rs.pop()],
 ['duplicate across shards',rs=>rs.push(clone(rs[0]))],
 ['wrong project',rs=>rs[0].suites[0].suites[0].specs[0].tests[0].projectName='webkit'],
 ['title whitespace changed',rs=>rs[0].suites[0].suites[0].specs[0].title='save exact fields'],
 ['file changed',rs=>rs[0].suites[0].suites[0].specs[0].file='other.spec.ts'],
 ['new skipped case',rs=>{const t=rs[0].suites[0].suites[0].specs[0].tests[0];t.status='skipped';t.results[0].status='skipped';rs[0].stats.expected=0;rs[0].stats.skipped=1;}],
 ['retry hides initial failure',rs=>{const t=rs[0].suites[0].suites[0].specs[0].tests[0];t.results=[{status:'failed',retry:0},{status:'passed',retry:1}];}],
 ['failed result with green summary',rs=>rs[0].suites[0].suites[0].specs[0].tests[0].results[0].status='failed'],
 ['not executed',rs=>rs[0].suites[0].suites[0].specs[0].tests[0].results=[]],
 ['worker error',rs=>rs[0].errors=[{message:'worker crashed'}]],
 ['contradictory stats',rs=>rs[0].stats.expected=42],
 ['flaky summary',rs=>rs[0].stats.flaky=1],
 ['empty shards',rs=>rs.splice(0)],
 ])test('PRD V049: reject '+name,()=>{
 const bad=clone(parts);mutate(bad);assert.throws(()=>verify(bad));
});
test('PRD V049: reject empty or duplicate baseline instead of deriving expectations from results',()=>{
 assert.throws(()=>verify(parts,report([]),[]));
 const bad=clone(baseline);bad.suites.push(clone(bad.suites[0]));assert.throws(()=>verify(parts,bad));
});
test('PRD V049: skip allowance must identify an actual baseline case',()=>{
 assert.throws(()=>verify(parts,baseline,[...allowedSkips,identity('chromium','unknown.spec.ts','fake')]));
});
test('PRD V049: a failed discovery report cannot define a smaller passing baseline',()=>{
 const bad=clone(baseline);bad.errors=[{message:'discovery failure'}];assert.throws(()=>verify(parts,bad));
});
