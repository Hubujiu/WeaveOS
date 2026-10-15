import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/api.ts',import.meta.url),'utf8');
const {applicationPublicationApi,applicationApiEnvelope}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id='abababab-abab-4bab-8bab-abababababab',path=`applications/${id}/forms/${id}/workflows/${id}/publish`,lookup=path.replace(/publish$/,'publications/'+id),body={operationId:id,expectedRevision:2,expectedSchemaVersion:1};
function setup(t,handler){const old=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{configurable:true,value:{cookie:'__Host-csrf=synthetic'}});t.after(()=>{if(old)Object.defineProperty(globalThis,'document',old);else delete globalThis.document;});t.mock.method(globalThis,'fetch',handler);}
for(const status of [200,202])test('publication transport retains exact POST HTTP '+status+' and actor/CSRF/body',async t=>{
 let sent;setup(t,async(url,init)=>{sent={url,init};return new Response(JSON.stringify({code:'OK',data:{status:status===202?'pending':'success'},meta:null}),{status,headers:{Location:'/api/v1/'+lookup}});});
 const signal=new AbortController().signal;const reply=await applicationPublicationApi(id,path,'POST',body,signal);
 assert.equal(reply.httpStatus,status);assert.equal(reply.location,'/api/v1/'+lookup);assert.equal(sent.url,'/api/v1/'+path);assert.equal(sent.init.headers['X-Expected-Actor-Id'],id);assert.equal(sent.init.headers['X-CSRF-Token'],'synthetic');assert.equal(sent.init.credentials,'include');assert.equal(sent.init.signal,signal);assert.deepEqual(JSON.parse(sent.init.body),body);
});
test('publication dedicated GET retains 200 without mutation body',async t=>{
 let sent;setup(t,async(_url,init)=>{sent=init;return new Response(JSON.stringify({code:'OK',data:{status:'pending'},meta:null}),{status:200});});
 const reply=await applicationPublicationApi(id,lookup,'GET');assert.equal(reply.httpStatus,200);assert.equal(sent.body,undefined);assert.equal(sent.headers['X-CSRF-Token'],undefined);
});
test('old envelope shape is unchanged by the specialized HTTP status path',async t=>{setup(t,async()=>new Response(JSON.stringify({code:'OK',data:{id},meta:null}),{status:200}));assert.deepEqual(await applicationApiEnvelope(id,'applications'),{data:{id},meta:null,location:null});});
for(const status of [201,203,204])test('publication unexpected successful status '+status+' stays unconfirmed',async t=>{
 setup(t,async()=>new Response(status===204?null:JSON.stringify({code:'OK',data:{},meta:null}),{status}));await assert.rejects(applicationPublicationApi(id,path,'POST',body),e=>e.unconfirmed===true);
});
test('publication POST network loss stays unknown while GET failure is read-only',async t=>{setup(t,async()=>{throw new TypeError('offline')});await assert.rejects(applicationPublicationApi(id,path,'POST',body),e=>e.unconfirmed===true);await assert.rejects(applicationPublicationApi(id,lookup,'GET'),e=>e.unconfirmed===false);});
for(const [name,p,m,b] of [
 ['ordinary write',`applications/${id}`,'POST',body],['save path',path.replace(/publish$/,'definition'),'POST',body],['query suffix',path+'?x=1','POST',body],['fragment',path+'#x','POST',body],['trailing slash',path+'/','POST',body],['encoded separator',path.replace('/workflows/','%2Fworkflows%2F'),'POST',body],
 ['POST lookup',lookup,'POST',body],['GET action',path,'GET',undefined],['GET body',lookup,'GET',body],
 ['extra actor',path,'POST',{...body,actorId:id}],['missing operation',path,'POST',{expectedRevision:2,expectedSchemaVersion:1}],['numeric operation',path,'POST',{...body,operationId:4}],['zero operation',path,'POST',{...body,operationId:'00000000-0000-0000-0000-000000000000'}],['extra action',path,'POST',{...body,action:'delete'}],['zero revision',path,'POST',{...body,expectedRevision:0}],['unsafe schema',path,'POST',{...body,expectedSchemaVersion:2**53}],
])test('publication transport rejects '+name+' before network',async t=>{let calls=0;setup(t,async()=>{calls++;throw Error('must not fetch')});await assert.rejects(applicationPublicationApi(id,p,m,b),TypeError);assert.equal(calls,0);});
