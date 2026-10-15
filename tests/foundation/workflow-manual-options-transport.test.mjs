import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/api.ts',import.meta.url),'utf8');
const {applicationReadPost}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee',path=`applications/${id}/forms/${id}/records/${id}/workflow-start-options/search`;
function setup(t,handler){const old=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{configurable:true,value:{cookie:'__Host-csrf=synthetic'}});t.after(()=>{if(old)Object.defineProperty(globalThis,'document',old);else delete globalThis.document;});t.mock.method(globalThis,'fetch',handler);}
test('V044 manual options query is an actor-bound read POST with original queryVersion',async t=>{
 let sent;setup(t,async(url,init)=>{sent={url,init};return new Response(JSON.stringify({code:'OK',data:{items:[]},meta:null}),{status:200});});
 const body={page:2,pageSize:20,queryVersion:'original-options'},signal=new AbortController().signal;
 assert.deepEqual(await applicationReadPost(id,path,body,signal),{items:[]});assert.equal(sent.url,'/api/v1/'+path);assert.equal(sent.init.headers['X-Expected-Actor-Id'],id);assert.equal(sent.init.headers['X-CSRF-Token'],'synthetic');assert.equal(sent.init.credentials,'include');assert.equal(sent.init.signal,signal);assert.deepEqual(JSON.parse(sent.init.body),body);
});
test('V044 options network loss cannot become an unknown mutation',async t=>{setup(t,async()=>{throw new TypeError('offline');});await assert.rejects(applicationReadPost(id,path,{page:1}),e=>e.status===0&&e.unconfirmed===false);});
for(const suffix of ['?x=1','#x','/'])test('V044 options query refuses path suffix '+suffix,async t=>{let calls=0;setup(t,async()=>{calls++;throw Error('must not send');});await assert.rejects(applicationReadPost(id,path+suffix,{page:1}),TypeError);assert.equal(calls,0);});
test('V044 manual mutation cannot masquerade as read-only search',async t=>{let calls=0;setup(t,async()=>{calls++;throw Error('must not send');});await assert.rejects(applicationReadPost(id,path.replace('workflow-start-options/search','workflow-starts'),{flowId:id}),TypeError);assert.equal(calls,0);});
