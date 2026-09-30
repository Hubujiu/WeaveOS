import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {countAPIReport,countBrowserReport} from '../../infra/acceptance/result-counts.mjs';
const tap='# tests 124\n# suites 0\n# pass 124\n# fail 0\n# cancelled 0\n# skipped 0\n# todo 0\n';
test('API actual TAP report counts every passed case',()=>assert.equal(countAPIReport(tap),124));
test('API failed, skipped, truncated and contradictory reports cannot publish success',()=>{
 for(const report of [tap.replace('# fail 0','# fail 1'),tap.replace('# skipped 0','# skipped 1'),tap.replace('# pass 124','# pass 123'),'# tests 124\n'])assert.throws(()=>countAPIReport(report));
});
test('browser counts come from the successful complete JSON report',()=>assert.equal(countBrowserReport({stats:{expected:117,unexpected:0,flaky:0,skipped:0}}),117));
test('browser failure, flakiness, skips and empty reports cannot publish success',()=>{
 for(const report of [{},{stats:{expected:0,unexpected:0,flaky:0,skipped:0}},{stats:{expected:117,unexpected:1,flaky:0,skipped:0}},{stats:{expected:117,unexpected:0,flaky:1,skipped:0}},{stats:{expected:117,unexpected:0,flaky:0,skipped:1}}])assert.throws(()=>countBrowserReport(report));
});
for(const path of ['infra/acceptance/run.mjs','infra/runtime/run.mjs'])test('real API/browser reports drive the published counts: '+path,()=>{
 const source=readFileSync(path,'utf8');assert.match(source,/countAPIReport\(readFileSync/);assert.match(source,/countBrowserReport\(JSON\.parse\(readFileSync/);assert.match(source,/test-reporter-destination=.*api\.tap/);assert.match(source,/PLAYWRIGHT_JSON_OUTPUT_NAME=.*browser\.json/);assert.doesNotMatch(source,/api:\s*122|browser:\s*111/);
});
