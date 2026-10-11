import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/workflows/recordWorkflowsContract.ts',import.meta.url),'utf8');
const {validRecordWorkflows:valid}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
// V046 nine-field projection and authoritative server page, no lifecycle authority.
const id=n=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
const row={id:id(1),flowId:id(2),name:'报销审批',initiatorId:id(3),state:'active',createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T09:01:00Z',definitionVersion:2,sequence:0};
const page={items:[row],total:1,page:1,pageSize:20,queryVersion:'q'.repeat(43)};
for(const state of ['starting','active','completed','rejected','withdrawn','no_effect'])test('record workflows accepts actual '+state,()=>assert.equal(valid({...page,items:[{...row,state}]},1,20),true));
for(const [name,value,p,size]of [
 ['empty',{...page,items:[],total:0},1,20],
 ['second page',{...page,page:2,total:21},2,20],
 ['deleted name empty',{...page,items:[{...row,name:''}]},1,20],
])test('record workflows accepts '+name,()=>assert.equal(valid(value,p,size),true));
for(const [name,value,p,size]of [
 ['pending invented state',{...page,items:[{...row,state:'pending'}]},1,20],
 ['engine data',{...page,items:[{...row,engineProcessId:'private'}]},1,20],
 ['hidden record',{...page,values:{private:'secret'}},1,20],
 ['duplicates',{...page,items:[row,row],total:2},1,20],
 ['wrong page',{...page,page:2},1,20],['wrong size',{...page,pageSize:10},1,20],
 ['bad total',{...page,total:-1},1,20],['unsafe total',{...page,total:9007199254740992},1,20],
 ['too many rows',{...page,items:[row,{...row,id:id(9)}],total:2,pageSize:1},1,1],
 ['outside total',{...page,page:2,total:20},2,20],['no version',{...page,queryVersion:''},1,20],
 ['wrong id',{...page,items:[{...row,flowId:'bad'}]},1,20],['zero id',{...page,items:[{...row,id:'00000000-0000-0000-0000-000000000000'}]},1,20],
 ['invalid time',{...page,items:[{...row,updatedAt:'2026-02-30T09:00:00Z'}]},1,20],
 ['unsafe sequence',{...page,items:[{...row,sequence:9007199254740992}]},1,20],
 ['zero definition',{...page,items:[{...row,definitionVersion:0}]},1,20],
 ['unapproved action flags',{...page,items:[{...row,canWithdraw:true}]},1,20],
 ['missing header',{...page,items:[Object.fromEntries(Object.entries(row).filter(([k])=>k!=='initiatorId'))]},1,20],
])test('record workflows rejects '+name,()=>assert.equal(valid(value,p,size),false));
const apiSource=await readFile(new URL('../../apps/web/src/applications/api.ts',import.meta.url),'utf8');
const {applicationReadPost,ApplicationError}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(apiSource,{mode:'transform'})).toString('base64'));
const path=`applications/${id(4)}/forms/${id(5)}/records/${id(6)}/workflow-instances/search`;
test('record workflow search is a read-only scoped POST with identity and CSRF, not a mutation',async()=>{
 const oldFetch=globalThis.fetch,oldDocument=globalThis.document;const calls=[];
 globalThis.document={cookie:'__Host-csrf=test-csrf'};globalThis.fetch=async(url,options)=>{calls.push({url,options});return new Response(JSON.stringify({code:'OK',data:page,meta:null}),{status:200,headers:{'Content-Type':'application/json'}});};
 try{assert.deepEqual(await applicationReadPost(id(3),path,{page:1,pageSize:20}),page);assert.equal(calls.length,1);assert.equal(calls[0].url,'/api/v1/'+path);assert.equal(calls[0].options.method,'POST');assert.equal(calls[0].options.headers['X-Expected-Actor-Id'],id(3));assert.equal(calls[0].options.headers['X-CSRF-Token'],'test-csrf');assert.deepEqual(JSON.parse(calls[0].options.body),{page:1,pageSize:20});}finally{globalThis.fetch=oldFetch;globalThis.document=oldDocument;}
});
test('record workflow search network failure is retryable read failure, never unconfirmed write',async()=>{
 const oldFetch=globalThis.fetch,oldDocument=globalThis.document;globalThis.document={cookie:''};globalThis.fetch=async()=>{throw new TypeError('network');};
 try{await assert.rejects(applicationReadPost(id(3),path,{page:1,pageSize:20}),e=>e instanceof ApplicationError&&e.status===0&&!e.unconfirmed);}finally{globalThis.fetch=oldFetch;globalThis.document=oldDocument;}
});
for(const suffix of ['actions','lifecycle','search/extra','search?actorId=other'])test('record workflow read transport refuses '+suffix,async()=>{await assert.rejects(applicationReadPost(id(3),path.replace(/search$/,suffix),{page:1}),TypeError);});
