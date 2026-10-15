import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const base=new URL('../../apps/web/src/applications/',import.meta.url);
const url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const read=async path=>stripTypeScriptTypes(await readFile(new URL(path,base),'utf8'),{mode:'transform'});
const history=url(await read('workflows/historyContract.ts'));
const source=(await read('workflows/approvalsContract.ts')).replaceAll("'./historyContract'",JSON.stringify(history));
const {validWorkflowApprovalsPage:valid}=await import(url(source));const {applicationReadPost}=await import(url(await read('api.ts')));
const id=n=>'abababab-abab-4bab-8bab-'+String(n).padStart(12,'0'),actor=id(1);
const event=()=>({id:id(2),instanceId:id(3),flowId:id(4),flowName:'Original approval',flowNameSource:'captured',nodeId:id(5),targetNodeId:null,actorId:actor,action:'agree',outcome:'success',sequence:2,schemaVersion:1,recordVersion:1,occurredAt:'2026-10-13T14:00:00.123456789Z'});
const row=()=>({appId:id(6),viewId:id(7),recordId:id(8),event:event()}),page=()=>({items:[row()],total:1,page:1,pageSize:20,queryVersion:'opaque'});
test('approved page accepts exact current actor history and complete empty or beyond-total pages without mutation',()=>{
 for(const action of ['agree','reject','return']){const p=page();p.items[0].event.action=action;if(action==='return')p.items[0].event.targetNodeId=id(9);const original=structuredClone(p);assert.equal(valid(p,1,20,actor),true);assert.deepEqual(p,original)}
 assert.equal(valid({...page(),total:0,items:[]},1,20,actor),true);assert.equal(valid({...page(),page:2,items:[]},2,20,actor),true);
});
test('approved page rejects privacy scope drift, non-approvals, malformed source and incomplete paging',()=>{
 for(const patch of [{actorId:id(10)},{action:'start'},{action:'withdraw'},{outcome:'no_effect'},{flowNameSource:'current'},{occurredAt:'2026-02-30T00:00:00Z'},{sequence:2**53},{nodeId:'BAD'},{basis:{fields:[]}}]){const p=page();Object.assign(p.items[0].event,patch);assert.equal(valid(p,1,20,actor),false,JSON.stringify(patch))}
 for(const patch of [{appId:'00000000-0000-0000-0000-000000000000'},{viewId:id(7).toUpperCase()},{recordId:null},{values:{private:'never'}}]){const p=page();Object.assign(p.items[0],patch);assert.equal(valid(p,1,20,actor),false)}
 for(const patch of [{page:2},{pageSize:10},{total:0},{total:2},{total:2**53},{items:[]},{queryVersion:''},{queryVersion:'x'.repeat(257)},{items:null},{basis:[]}])assert.equal(valid({...page(),...patch},1,20,actor),false,JSON.stringify(patch));
 assert.equal(valid({...page(),items:[row(),row()],total:2},1,20,actor),false);assert.equal(valid({...page(),page:Number.MAX_SAFE_INTEGER,pageSize:100,items:[]},Number.MAX_SAFE_INTEGER,100,actor),false);assert.equal(valid(page(),1,20,'BAD'),false);
});
function setup(t,handler){const old=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{configurable:true,value:{cookie:'__Host-csrf=fixture-csrf'}});t.after(()=>{if(old)Object.defineProperty(globalThis,'document',old);else delete globalThis.document});t.mock.method(globalThis,'fetch',handler)}
test('approved history exact read POST preserves actor CSRF and abort without operation data',async t=>{
 let seen;setup(t,async(url,init)=>{seen={url,init};return new Response(JSON.stringify({code:'OK',data:page(),meta:null}),{status:200})});const ac=new AbortController();assert.deepEqual(await applicationReadPost(actor,'workflow-approvals/search',{page:1},ac.signal),page());assert.equal(seen.url,'/api/v1/workflow-approvals/search');assert.equal(seen.init.headers['X-Expected-Actor-Id'],actor);assert.equal(seen.init.headers['X-CSRF-Token'],'fixture-csrf');assert.equal(seen.init.signal,ac.signal);assert.equal(seen.init.credentials,'include');assert.equal(seen.init.method,'POST');assert.deepEqual(JSON.parse(seen.init.body),{page:1});
});
test('approved history network failure never becomes an uncertain approval action',async t=>{setup(t,async()=>{throw Error('offline')});await assert.rejects(applicationReadPost(actor,'workflow-approvals/search',{page:1}),e=>e.code==='COMMON_SERVICE_UNAVAILABLE'&&e.unconfirmed===false)});
test('approved history read helper does not admit adjacent or ambiguous write paths',async()=>{for(const path of ['workflow-approvals/search/','workflow-approvals/search?actor=x','workflow-approvals/actions','workflow-approvals/search/actions'])await assert.rejects(applicationReadPost(actor,path,{page:1}),TypeError)});
