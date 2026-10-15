import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const base=new URL('../../apps/web/src/applications/',import.meta.url);
const url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const read=async path=>stripTypeScriptTypes(await readFile(new URL(path,base),'utf8'),{mode:'transform'});
const instances=url(await read('workflows/recordWorkflowsContract.ts'));
const source=(await read('workflows/startedContract.ts')).replaceAll("'./recordWorkflowsContract'",JSON.stringify(instances));
const {validWorkflowStartedPage:valid}=await import(url(source));const {applicationReadPost}=await import(url(await read('api.ts')));
const id=n=>'abababab-abab-4bab-8bab-'+String(n).padStart(12,'0'),actor=id(1);
const instance=()=>({id:id(2),flowId:id(3),name:'原流程',initiatorId:actor,state:'active',createdAt:'2026-10-13T14:00:00Z',updatedAt:'2026-10-13T14:00:00Z',definitionVersion:1,sequence:1});
const row=()=>({appId:id(4),viewId:id(5),recordId:id(6),instance:instance()}),page=()=>({items:[row()],total:1,page:1,pageSize:20,queryVersion:'opaque'});
test('started page accepts only current initiator and preserves all six truthful instance states',()=>{
 for(const state of ['starting','active','completed','rejected','withdrawn','no_effect']){const p=page();p.items[0].instance.state=state;if(state==='starting')p.items[0].instance.sequence=0;const before=structuredClone(p);assert.equal(valid(p,1,20,actor),true);assert.deepEqual(p,before)}
 assert.equal(valid({...page(),total:0,items:[]},1,20,actor),true);assert.equal(valid({...page(),page:2,items:[]},2,20,actor),true);
});
test('started page rejects other initiators, malformed instance scope and incomplete count windows',()=>{
 for(const patch of [{initiatorId:id(9)},{state:'approved'},{createdAt:'2026-02-30T00:00:00Z'},{sequence:2**53},{definitionVersion:0},{actorId:actor},{id:'BAD'},{values:{secret:'never'}}]){const p=page();Object.assign(p.items[0].instance,patch);assert.equal(valid(p,1,20,actor),false,JSON.stringify(patch))}
 for(const patch of [{appId:'00000000-0000-0000-0000-000000000000'},{viewId:id(5).toUpperCase()},{recordId:null},{createdBy:actor}]){const p=page();Object.assign(p.items[0],patch);assert.equal(valid(p,1,20,actor),false)}
 for(const patch of [{page:2},{pageSize:10},{total:0},{total:2},{total:2**53},{items:[]},{queryVersion:''},{queryVersion:'x'.repeat(257)},{items:null},{basis:[]}])assert.equal(valid({...page(),...patch},1,20,actor),false,JSON.stringify(patch));
 assert.equal(valid({...page(),items:[row(),row()],total:2},1,20,actor),false);assert.equal(valid({...page(),page:Number.MAX_SAFE_INTEGER,pageSize:100,items:[]},Number.MAX_SAFE_INTEGER,100,actor),false);assert.equal(valid(page(),1,20,'BAD'),false);
});
function setup(t,handler){const old=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{configurable:true,value:{cookie:'__Host-csrf=fixture-csrf'}});t.after(()=>{if(old)Object.defineProperty(globalThis,'document',old);else delete globalThis.document});t.mock.method(globalThis,'fetch',handler)}
test('started instances exact read POST preserves actor CSRF and abort without operation data',async t=>{
 let seen;setup(t,async(url,init)=>{seen={url,init};return new Response(JSON.stringify({code:'OK',data:page(),meta:null}),{status:200})});const ac=new AbortController();assert.deepEqual(await applicationReadPost(actor,'workflow-started/search',{page:1},ac.signal),page());assert.equal(seen.url,'/api/v1/workflow-started/search');assert.equal(seen.init.headers['X-Expected-Actor-Id'],actor);assert.equal(seen.init.headers['X-CSRF-Token'],'fixture-csrf');assert.equal(seen.init.signal,ac.signal);assert.equal(seen.init.credentials,'include');assert.equal(seen.init.method,'POST');assert.deepEqual(JSON.parse(seen.init.body),{page:1});
});
test('started instances network failure never becomes an uncertain approval action',async t=>{setup(t,async()=>{throw Error('offline')});await assert.rejects(applicationReadPost(actor,'workflow-started/search',{page:1}),e=>e.code==='COMMON_SERVICE_UNAVAILABLE'&&e.unconfirmed===false)});
test('started instances read helper does not admit adjacent or ambiguous write paths',async()=>{for(const path of ['workflow-started/search/','workflow-started/search?actor=x','workflow-started/actions','workflow-started/search/actions'])await assert.rejects(applicationReadPost(actor,path,{page:1}),TypeError)});

test('started archive admits only explicit retained terminal or no-effect rounds with truthful name source',()=>{
 for(const flowNameSource of ['captured','legacy_last_known'])for(const state of ['completed','rejected','withdrawn','no_effect']){
  const p=page();Object.assign(p.items[0],{archived:true,flowNameSource});p.items[0].instance.state=state;const before=structuredClone(p);assert.equal(valid(p,1,20,actor),true);assert.deepEqual(p,before);
 }
});
test('started archive rejects ambiguous source markers and refuses to call active intent archived',()=>{
 for(const patch of [{archived:false,flowNameSource:'captured'},{archived:true},{flowNameSource:'captured'},{archived:true,flowNameSource:'current'},{archived:true,flowNameSource:'captured',private:[]}]){const p=page();p.items[0].instance.state='completed';Object.assign(p.items[0],patch);assert.equal(valid(p,1,20,actor),false,JSON.stringify(patch))}
 for(const state of ['starting','active']){const p=page();p.items[0].instance.state=state;Object.assign(p.items[0],{archived:true,flowNameSource:'captured'});assert.equal(valid(p,1,20,actor),false)}
});
