import test from 'node:test';import assert from 'node:assert/strict';
import {validateFixture,validateReports} from '../../services/workflow-engine/product-browser-contract.mjs';
const head='a'.repeat(40),id=n=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');
const browsers=['chromium','firefox','webkit'];
const fixture=()=>({version:1,head,browser:'chromium',origin:'https://127.0.0.1:34567',actorId:id(1),outsiderId:id(2),appId:id(3),viewId:id(4),recordId:id(5),instanceId:id(6),taskId:id(7),fieldId:id(8),session:'synthetic-session',csrf:'synthetic-csrf',otherSession:'synthetic-other-session',otherCsrf:'synthetic-other-csrf'});
const file='services/workflow-engine/formal-browser.spec.ts',title='real task save, approve, retained evidence and outsider isolation';
function report(projects,listed=false){return {errors:[],stats:{expected:projects.length,unexpected:0,flaky:0,skipped:0},suites:[{title:file,file,specs:[{title,file,tests:projects.map(projectName=>({projectName,expectedStatus:'passed',status:'expected',results:listed?[]:[{status:'passed',retry:0}]}))}],suites:[]}]};}
const pkg='github.com/Hubujiu/WeaveOS/services/bff/cmd/bff',name='TestRootProductBrowserJourney';
const input=()=>({head,baseline:report(browsers,true),reports:browsers.map(b=>report([b])),backend:browsers.map(browser=>({browser,head,events:[{Action:'run',Package:pkg,Test:name},{Action:'pass',Package:pkg,Test:name},{Action:'pass',Package:pkg}]}))});
test('product browser fixture accepts only the exact candidate and isolated synthetic two-actor scope',()=>{const f=fixture(),before=structuredClone(f);assert.equal(validateFixture(f,head,'chromium'),f);assert.deepEqual(f,before)});
test('product browser fixture rejects external origins, wrong heads, actors and secret-bearing extra fields',()=>{
 for(const patch of [{origin:'https://example.com:34567'},{origin:'http://127.0.0.1:34567'},{origin:'https://127.0.0.1:34567/x'},{origin:'https://name:pass@127.0.0.1:34567'},{origin:'https://127.0.0.1:34567/?key=private'},{head:'b'.repeat(40)},{browser:'webkit'},{outsiderId:id(1)},{recordId:'00000000-0000-0000-0000-000000000000'},{session:'bad\r\nheader'},{databaseURL:'not-allowed'}])assert.throws(()=>validateFixture({...fixture(),...patch},head,'chromium'));
});
test('product browser gate requires all three native browser results and all three independent backend proofs',()=>{const data=input(),before=structuredClone(data);assert.deepEqual(validateReports(data),{head,browsers:3,browserCases:3,backendCases:3});assert.deepEqual(data,before)});
test('product browser gate rejects missing, skipped, retried or falsely green browser cases',()=>{
 for(const change of [x=>x.reports.pop(),x=>x.reports.push(x.reports[0]),x=>x.reports[0].suites[0].specs[0].tests[0].results=[],x=>x.reports[0].suites[0].specs[0].tests[0].results[0].status='failed',x=>x.reports[0].suites[0].specs[0].tests[0].results.push({status:'passed',retry:1}),x=>x.reports[0].stats.skipped=1,x=>x.baseline.suites[0].specs[0].tests.pop(),x=>{for(const r of [x.baseline,...x.reports])r.suites[0].specs[0].title='a smaller unrelated test'}]){const x=input();change(x);assert.throws(()=>validateReports(x))}
});
test('product browser gate rejects absent or contradictory real backend completion and stale candidate identity',()=>{
 for(const change of [x=>x.backend.pop(),x=>x.backend.push(x.backend[0]),x=>x.backend[0].head='b'.repeat(40),x=>x.backend[0].browser='webkit',x=>x.backend[0].events.pop(),x=>x.backend[0].events[1].Action='fail',x=>x.backend[0].events[1].Action='skip',x=>x.backend[0].events[1].Test='OtherTest',x=>x.backend[0].events.push({Action:'fail',Package:pkg}),x=>x.head='bad']){const x=input();change(x);assert.throws(()=>validateReports(x))}
});
