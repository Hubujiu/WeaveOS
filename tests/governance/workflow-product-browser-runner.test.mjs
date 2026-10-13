import test from 'node:test';
import assert from 'node:assert/strict';
import {browserCommand,runBrowserJourneys} from '../../services/workflow-engine/product-browser-runner.mjs';
const head='a'.repeat(40);
const options={root:'/repo',output:'/output',exchange:'/private',runner:'weaveos-v041-formal-123-45-runner',uid:1001,gid:1002,head,browser:'chromium'};
test('P23 browser uses only owned network, locked image, read-only inputs and private exchange',()=>{
 const args=browserCommand(options);
 assert.ok(Array.isArray(args));
 assert.deepEqual(args.slice(0,3),['run','--rm','--init']);
 const after=flag=>args[args.indexOf(flag)+1];
 assert.equal(after('--network'),'container:'+options.runner);
 assert.equal(after('--user'),'1001:1002');
 assert.ok(args.includes('/repo:/repo:ro'));
 assert.ok(args.includes('/private:/private:ro'));
 assert.ok(args.includes('/output:/output:rw'));
 assert.ok(args.includes('mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27'));
 assert.ok(args.includes('WEAVEOS_PRODUCT_BROWSER_FIXTURE=/private/chromium.request.json'));
 assert.ok(args.includes('WEAVEOS_PRODUCT_BROWSER_REPORT=/output/chromium.json'));
 assert.ok(args.includes('WEAVEOS_PRODUCT_HEAD='+head));
 assert.ok(args.includes('--project=chromium'));
 assert.ok(!args.some(x=>/docker.sock|--privileged|--publish|--network=host|--env-file|session=|csrf=/i.test(x)));
});
test('P23 browser command refuses foreign containers, ambiguous paths and unbound source/browser',()=>{
 for(const patch of [{runner:'other-runner'},{runner:'weaveos-v041-formal-1-2-runner;evil'},{root:'relative'},{root:'/repo:rw'},{output:'/repo'},{exchange:'/output'},{head:'bad'},{browser:'all'},{uid:-1},{gid:'0'}])assert.throws(()=>browserCommand({...options,...patch}));
});
test('P23 sequencer starts actual backend before browser, waits each completion and retains native results',async()=>{
 const seen=[];
 const result=await runBrowserJourneys({
  start:async browser=>{seen.push('start '+browser);return {browser};},
  ready:async h=>seen.push('ready '+h.browser),
  browser:async h=>{seen.push('browser '+h.browser);return {native:h.browser};},
  signal:async(h,success)=>seen.push(`signal ${h.browser} ${success}`),
  finish:async h=>{seen.push('finish '+h.browser);return {go:h.browser};},
 });
 assert.deepEqual(seen,['chromium','firefox','webkit'].flatMap(b=>[`start ${b}`,`ready ${b}`,`browser ${b}`,`signal ${b} true`,`finish ${b}`]));
 assert.deepEqual(result,['chromium','firefox','webkit'].map(browser=>({browser,report:{native:browser},backend:{go:browser}})));
});
test('P23 failed browser signals failure and waits for owned backend without starting a second browser',async()=>{
 const seen=[];const failure=Error('actual browser failed');
 await assert.rejects(()=>runBrowserJourneys({start:async b=>({browser:b}),ready:async()=>{},browser:async()=>{throw failure;},signal:async(h,ok)=>seen.push(['signal',h.browser,ok]),finish:async h=>{seen.push(['finish',h.browser]);throw Error('backend refused failed browser');}}),e=>e===failure);
 assert.deepEqual(seen,[['signal','chromium',false],['finish','chromium']]);
});
test('P23 readiness and backend failure are never converted into green or silently retried',async()=>{
 for(const where of ['ready','finish']){
  const seen=[];const error=Error(where+' failure');
  await assert.rejects(()=>runBrowserJourneys({start:async b=>{seen.push(b);return {browser:b};},ready:async()=>{if(where==='ready')throw error;},browser:async()=>({native:true}),signal:async()=>{},finish:async()=>{if(where==='finish')throw error;}}),e=>e===error);
  assert.deepEqual(seen,['chromium']);
 }
});
