import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const data=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
// The only mocked module is React's state storage, not the operation algorithm,
// API, request validation or recovery store. These are non-DOM protocol tests.
const react=data(`let state;export const useEffect=()=>{},useRef=value=>({current:value});export function useState(value){state=typeof value==='function'?value():value;return [state,next=>{state=typeof next==='function'?next(state):next}]}export const getState=()=>state;`);
const cache=new Map();
async function load(url){if(cache.has(url.href))return cache.get(url.href);let code=stripTypeScriptTypes(await readFile(url,'utf8'),{mode:'transform'});for(const match of [...code.matchAll(/from ['"]([^'"]+)['"]/g)]){const spec=match[1];const target=spec==='react'?react:await load(new URL(spec+'.ts',url));code=code.replace(match[0],`from '${target}'`);}const value=data(code);cache.set(url.href,value);return value;}
const hookUrl=await load(new URL('../../apps/web/src/applications/useApplicationOperation.ts',import.meta.url));
const {useApplicationOperation}=await import(hookUrl),{getState}=await import(react);
const id=n=>'ffffffff-ffff-4fff-8fff-'+String(n).padStart(12,'0');let seq=10;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
function setup(t,reply){
 const actor=id(++seq),resource={kind:'round-start',appId:id(1),viewId:id(2),id:id(3),instanceId:id(6),flowId:id(4),roundKind:'resubmit'},path=`applications/${id(1)}/forms/${id(2)}/records/${id(3)}/workflows/${id(6)}/round-actions`;
 const calls=[],confirmed=[],identity=[];let sessionActor=actor;
 for(const [key,value] of [['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:sessionActor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const hook=useApplicationOperation({actorId:actor,scope:'round-protocol',resource,confirmed:value=>confirmed.push(value),unauthorized:()=>{},identityMismatch:()=>identity.push(true)});
 const request={resource,path,method:'POST',expectedStatus:202,body:{kind:'resubmit',expectedWorkflowRevision:7,expectedSchemaVersion:1,expectedRecordVersion:5}};
 return {hook,request,calls,confirmed,identity,actor,setActor:value=>{sessionActor=value}};
}
function response(data,status=200,location){return new Response(JSON.stringify({code:'OK',data,meta:null}),{status,headers:location?{Location:location}:{}});}
const receipt=call=>({operationId:call.body.operationId,flowId:id(4),previousInstanceId:id(6),instanceId:id(5),roundKind:'resubmit',roundNumber:2,status:'accepted'});
test('V069 shared hook confirms only durable new-round intent exactly once',async t=>{
 const x=setup(t,call=>response(receipt(call),202,'/api/v1/application-operations/'+call.body.operationId));
 assert.doesNotThrow(()=>x.hook.start(x.request));x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);assert.equal(x.confirmed.length,1);assert.deepEqual(x.confirmed[0],receipt(x.calls[0]));assert.equal(x.hook.getPendingStatus(),null);
 assert.equal(x.calls[0].init.headers['X-Expected-Actor-Id'],x.actor);assert.equal(x.calls[0].init.headers['X-CSRF-Token'],'synthetic');assert.deepEqual(Object.keys(x.calls[0].body).sort(),['expectedRecordVersion','expectedSchemaVersion','expectedWorkflowRevision','kind','operationId']);
});
for(const location of [undefined,'https://evil.invalid/api/v1/application-operations/x','/api/v1/application-workflow-operations/x','/api/v1/application-operations/wrong'])test('V069 invalid 202 Location retains original uncertainty '+location,async t=>{const x=setup(t,call=>response(receipt(call),202,location));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');assert.equal(getState().phase,'unconfirmed');});
test('V069 unknown round retry keeps original operation and immutable CAS',async t=>{
 const x=setup(t,(call,calls)=>calls.length===1?response(null,503):response(receipt(call),202,'/api/v1/application-operations/'+call.body.operationId));x.hook.start(x.request);await flush();assert.equal(x.hook.getPendingStatus(),'unknown');const first=structuredClone(x.calls[0].body);x.request.body.expectedRecordVersion=999;x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);x.hook.retry();await flush();assert.equal(x.calls.length,2);assert.deepEqual(x.calls[1].body,first);assert.equal(x.confirmed.length,1);
});
test('V069 original generic operation lookup cannot use execution command endpoint',async t=>{
 let first;const x=setup(t,call=>{if(call.init.method==='POST'){first=call;return response(null,503);}return response({operationId:first.body.operationId,status:'confirmed',httpStatus:202,result:receipt(first),location:'/api/v1/application-operations/'+first.body.operationId});});x.hook.start(x.request);await flush();x.hook.query();await flush();assert.equal(x.confirmed.length,1);assert.equal(x.calls.length,2);assert.equal(x.calls[1].url,'/api/v1/application-operations/'+first.body.operationId);assert.equal(x.calls[1].init.method,'GET');assert.equal(x.calls[1].body,undefined);
});
test('V069 changed identity cannot transmit new round',async t=>{const x=setup(t,()=>{throw Error('must not write')});x.setActor(id(999));x.hook.start(x.request);await flush();assert.equal(x.calls.length,0);assert.equal(x.confirmed.length,0);assert.equal(x.identity.length,1);});

for(const mismatch of [{previousInstanceId:id(999)},{roundKind:'review'},{flowId:id(999)}])test('V069 wrong receipt binding remains unknown '+JSON.stringify(mismatch),async t=>{const x=setup(t,call=>response({...receipt(call),...mismatch},202,'/api/v1/application-operations/'+call.body.operationId));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');});
