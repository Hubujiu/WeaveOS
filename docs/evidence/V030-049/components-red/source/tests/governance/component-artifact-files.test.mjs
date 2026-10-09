import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,unlinkSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {loadComponentArtifacts} from '../../scripts/verify-component-artifacts.mjs';
const sha='a'.repeat(40);
function report(i){return {errors:[],stats:{expected:1,unexpected:0,flaky:0,skipped:0},suites:[{title:'a.spec.ts',specs:[{file:'a.spec.ts',title:'case '+i,tests:[{projectName:'chromium',expectedStatus:'passed',status:'expected',results:[{status:'passed',retry:0}]}]}]}]};}
function fixture(t){
 const directory=mkdtempSync(join(tmpdir(),'v049-report-fixture-'));t.after(()=>rmSync(directory,{recursive:true,force:true}));
 const baseline={errors:[],suites:[1,2,3,4].flatMap(i=>report(i).suites)};
 for(const s of baseline.suites)for(const spec of s.specs)for(const tc of spec.tests)tc.results=[];
 for(let i=1;i<=4;i++){const d=join(directory,'shard-'+i);mkdirSync(d);for(const [name,data] of [['baseline.json',baseline],['report.json',report(i)],['metadata.json',{sourceSha:sha,shardIndex:i,shardTotal:4}]])writeFileSync(join(d,name),JSON.stringify(data));}
 return directory;
}
const run=directory=>loadComponentArtifacts({directory,expectedSha:sha,allowedSkips:[]});
test('PRD V049: file loader verifies four actual report bundles',t=>{const r=run(fixture(t));assert.equal(r.stats.expected,4);assert.equal(r.coverage.total,4);assert.equal(r.sourceSha,sha);});
for(const name of ['report.json','baseline.json','metadata.json'])test('PRD V049: missing '+name+' cannot become acceptance',t=>{const d=fixture(t);unlinkSync(join(d,'shard-3',name));assert.throws(()=>run(d));});
test('PRD V049: every shard must discover the same full identity set',t=>{const d=fixture(t),p=join(d,'shard-2','baseline.json');const x=JSON.parse(readFileSync(p,'utf8'));x.suites[0].specs[0].title='different discovery';writeFileSync(p,JSON.stringify(x));assert.throws(()=>run(d));});
test('PRD V049: an errored secondary discovery cannot be ignored',t=>{const d=fixture(t),p=join(d,'shard-4','baseline.json');const x=JSON.parse(readFileSync(p,'utf8'));x.errors=[{message:'partial discovery'}];writeFileSync(p,JSON.stringify(x));assert.throws(()=>run(d));});
test('PRD V049: corrupt JSON is a hard failure',t=>{const d=fixture(t);writeFileSync(join(d,'shard-1','report.json'),'{');assert.throws(()=>run(d));});
test('PRD V049: a passed summary alone never substitutes for actual reports',t=>{const d=mkdtempSync(join(tmpdir(),'v049-summary-only-'));t.after(()=>rmSync(d,{recursive:true,force:true}));writeFileSync(join(d,'summary.json'),JSON.stringify({sourceSha:sha,stats:{expected:500,unexpected:0,flaky:0,skipped:0}}));assert.throws(()=>run(d));});
