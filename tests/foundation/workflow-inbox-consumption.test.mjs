import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
async function module(path){return import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(await readFile(new URL(path,import.meta.url),'utf8'),{mode:'transform'})).toString('base64'));}
const {validInboxPage}=await module('../../apps/web/src/applications/workflows/inboxContract.ts');
const {applicationReadPost}=await module('../../apps/web/src/applications/api.ts');
// Independent V061 wire + V075 closed/identity consumption oracle.
const id='12345678-1234-4234-8234-123456789abc';
const row={id,appId:id,viewId:id,recordId:id,instanceId:id,flowId:id,nodeId:id,flowName:'Expense',createdAt:'2026-10-11T10:00:00.123456Z',definitionVersion:1,activationEpoch:1,sequence:0};
const result={items:[row],total:1,page:1,pageSize:20,queryVersion:'q1'};
test('inbox accepts independently specified valid first, last and empty pages',()=>{
 assert.equal(validInboxPage(result,1,20),true);
 assert.equal(validInboxPage({...result,page:2,total:21},2,20),true);
 assert.equal(validInboxPage({...result,items:[],total:0},1,20),true);
 assert.equal(validInboxPage({...result,items:[],page:3,total:21},3,20),true);
});
for(const [name,bad] of Object.entries({
 extra:{...result,values:{}},missing:{...result,queryVersion:undefined},tokenEmpty:{...result,queryVersion:''},tokenLong:{...result,queryVersion:'x'.repeat(257)},
 wrongPage:{...result,page:2},wrongSize:{...result,pageSize:10},unsafeTotal:{...result,total:9007199254740992},negativeTotal:{...result,total:-1},
 duplicate:{...result,total:2,items:[row,row]},overTotal:{...result,total:0},overPage:{...result,total:21,items:Array.from({length:21},(_,i)=>({...row,id:'12345678-1234-4234-8234-'+String(i).padStart(12,'0')}))},
 rowsNotArray:{...result,items:{}},null:null,
}))test('inbox rejects '+name,()=>assert.equal(validInboxPage(bad,1,20),false));
for(const [key,value] of [['id','BAD'],['appId',id.toUpperCase()],['viewId',null],['recordId',42],['instanceId',''],['flowId',[]],['nodeId',{}],['flowName',42],['createdAt','not-time'],['createdAt','2026-02-30T00:00:00Z'],['definitionVersion',0],['activationEpoch',0],['sequence',-1],['sequence',1.5],['sequence',9007199254740992],['privateValues',{}]])test('inbox rejects row '+key+'='+JSON.stringify(value),()=>assert.equal(validInboxPage({...result,items:[{...row,[key]:value}]},1,20),false));
function setup(t,handler){
 const old=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{configurable:true,value:{cookie:'__Host-csrf=test-csrf'}});
 t.after(()=>{if(old)Object.defineProperty(globalThis,'document',old);else delete globalThis.document;});t.mock.method(globalThis,'fetch',handler);
}
test('inbox exact read POST retains actor, CSRF, cancellation and no operation key',async t=>{
 const ac=new AbortController();let actual;
 setup(t,async(url,init)=>{actual={url,init};return new Response(JSON.stringify({code:'OK',data:result,meta:null}),{status:200});});
 assert.deepEqual(await applicationReadPost(id,'workflow-tasks/search',{page:1,pageSize:20},ac.signal),result);
 assert.equal(actual.url,'/api/v1/workflow-tasks/search');assert.equal(actual.init.method,'POST');assert.equal(actual.init.credentials,'include');assert.equal(actual.init.signal,ac.signal);
 assert.equal(actual.init.headers['X-Expected-Actor-Id'],id);assert.equal(actual.init.headers['X-CSRF-Token'],'test-csrf');assert.deepEqual(JSON.parse(actual.init.body),{page:1,pageSize:20});
});
test('inbox network error remains a read, never an unconfirmed mutation',async t=>{
 setup(t,async()=>{throw new TypeError('network');});
 await assert.rejects(applicationReadPost(id,'workflow-tasks/search',{page:1}),e=>e.code==='COMMON_SERVICE_UNAVAILABLE'&&e.unconfirmed===false);
});
for(const path of ['workflow-tasks/search/','workflow-tasks/search?actor=x','workflow-tasks/search/actions','workflow-starts','applications/'+id+'/forms/'+id+'/workflows/'+id+'/enable'])test('read helper rejects mutation or ambiguous path '+path,async()=>{
 await assert.rejects(applicationReadPost(id,path,{page:1}),TypeError);
});
