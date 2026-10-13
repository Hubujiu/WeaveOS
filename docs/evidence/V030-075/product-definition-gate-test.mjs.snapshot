import test from 'node:test';
import assert from 'node:assert/strict';
import {validateDefinitionFixture,validateDefinitionReports,validateCompleteReports} from '../../services/workflow-engine/product-browser-contract.mjs';
import {browserCommand} from '../../services/workflow-engine/product-browser-runner.mjs';
const head='a'.repeat(40),id=n=>'dddddddd-dddd-4ddd-8ddd-'+String(n).padStart(12,'0'),browsers=['chromium','firefox','webkit'];
const fixture=()=>({version:2,head,browser:'chromium',origin:'https://127.0.0.1:34567',actorId:id(1),outsiderId:id(2),appId:id(3),viewId:id(4),recordId:id(5),fieldId:id(6),session:'synthetic-session',csrf:'synthetic-csrf',otherSession:'synthetic-other-session',otherCsrf:'synthetic-other-csrf'});
function input(definition=true){
 const file=definition?'services/workflow-engine/formal-definition-browser.spec.ts':'services/workflow-engine/formal-browser.spec.ts';
 const title=definition?'real definition creation, publication, manual start and approval':'real task save, approve, retained evidence and outsider isolation';
 const name=definition?'TestRootProductDefinitionJourney':'TestRootProductBrowserJourney',pkg='github.com/Hubujiu/WeaveOS/services/bff/cmd/bff';
 const report=(projects,listed=false)=>({errors:[],stats:{expected:projects.length,unexpected:0,flaky:0,skipped:0},suites:[{title:file,file,specs:[{title,file,tests:projects.map(projectName=>({projectName,expectedStatus:'passed',status:'expected',results:listed?[]:[{status:'passed',retry:0}]}))}],suites:[]}]});
 return {head,baseline:report(browsers,true),reports:browsers.map(b=>report([b])),backend:browsers.map(browser=>({browser,head,events:[{Action:'run',Package:pkg,Test:name},{Action:'pass',Package:pkg,Test:name},{Action:'pass',Package:pkg}]}))};
}
test('P23 definition fixture contains only existing resource identities and synthetic sessions',()=>{
 const f=fixture(),before=structuredClone(f);assert.equal(validateDefinitionFixture(f,head,'chromium'),f);assert.deepEqual(f,before);
});
test('P23 definition fixture rejects fabricated task identities and unsafe scope without private error text',()=>{
 for(const patch of [{version:1},{taskId:id(8)},{instanceId:id(9)},{origin:'https://outside.invalid:34567'},{origin:'https://127.0.0.1:34567/?private=value'},{origin:'synthetic-private-input'},{actorId:id(2)},{head:'b'.repeat(40)},{browser:'webkit'},{session:'unsafe\r\nheader'}]){
  assert.throws(()=>validateDefinitionFixture({...fixture(),...patch},head,'chromium'),error=>!JSON.stringify(error,Object.getOwnPropertyNames(error)).includes('synthetic-private-input'));
 }
});
test('P23 definition gate requires the exact new three browser and independent backend identities',()=>{
 const i=input(),before=structuredClone(i);assert.deepEqual(validateDefinitionReports(i),{head,browsers:3,browserCases:3,backendCases:3});assert.deepEqual(i,before);
});
test('P23 definition gate rejects approval-only, missing, retried and mismatched proofs',()=>{
 assert.throws(()=>validateDefinitionReports(input(false)));
 for(const mutate of [i=>i.reports.pop(),i=>i.backend.pop(),i=>i.backend[1]=i.backend[0],i=>i.backend[0].head='b'.repeat(40),i=>i.reports[0].suites[0].specs[0].tests[0].results[0].retry=1,i=>i.backend[0].events[1].Action='skip']){const i=input();mutate(i);assert.throws(()=>validateDefinitionReports(i));}
});
test('P23 complete gate retains both independent same-head three-browser journeys',()=>{
 assert.deepEqual(validateCompleteReports({approval:input(false),definition:input()}),{head,browsers:3,browserCases:6,backendCases:6});
});
test('P23 complete gate cannot succeed on one journey or mixed candidate evidence',()=>{
 for(const x of [{approval:input(false)},{definition:input()},{approval:input(false),definition:{...input(),head:'b'.repeat(40)}}])assert.throws(()=>validateCompleteReports(x));
});
test('P23 definition browser command selects only the fixed definition config and rejects other journeys',()=>{
 const opts={root:'/repo',output:'/output',exchange:'/private',runner:'weaveos-v041-formal-123-45-runner',uid:1001,gid:1002,head,browser:'chromium'};
 const args=browserCommand({...opts,journey:'definition'});assert.equal(args[args.indexOf('--config')+1],'services/workflow-engine/playwright.definition.config.ts');
 assert.throws(()=>browserCommand({...opts,journey:'other'}));
});
