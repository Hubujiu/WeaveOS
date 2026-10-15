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
const id=n=>'abababab-abab-4bab-8bab-'+String(n).padStart(12,'0');let seq=20;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
function response(data,status=200,location){return new Response(JSON.stringify({code:'OK',data,meta:null}),{status,headers:location?{Location:location}:{}});}
function setup(t,reply){
 const actor=id(++seq),resource={kind:'workflow-publication',appId:id(1),viewId:id(2),id:id(3),version:2},base=`applications/${id(1)}/forms/${id(2)}/workflows/${id(3)}`,calls=[],confirmed=[],identity=[];let sessionActor=actor;
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:sessionActor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls,base);});
 const hook=useApplicationOperation({actorId:actor,scope:'publication-protocol',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>identity.push(true)});
 const request={resource,path:base+'/publish',method:'POST',expectedStatus:202,body:{expectedRevision:4,expectedSchemaVersion:3}};
 return {hook,request,calls,confirmed,identity,actor,base,setActor:v=>sessionActor=v};
}
const receipt=(call,status='pending',reason=null)=>({operationId:call.body.operationId,flowId:id(3),version:2,status,reason});
const location=(call,base)=>'/api/v1/'+base+'/publications/'+call.body.operationId;
test('V027 publication 202 remains accepted until its own status confirms the original candidate',async t=>{
 let first;const x=setup(t,(call,_calls,base)=>{if(call.init.method==='POST'){first=call;return response(receipt(call),202,location(call,base));}return response(receipt(first,'confirmed'));});x.hook.start(x.request);x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);assert.equal(x.confirmed.length,0);assert.equal(getState().phase,'accepted');x.hook.query();await flush();assert.equal(x.calls[1].url,location(first,x.base));assert.equal(x.confirmed[0].status,'confirmed');assert.equal(x.hook.getPendingStatus(),null);assert.equal(x.calls[0].init.headers['X-Expected-Actor-Id'],x.actor);assert.equal(x.calls[0].init.headers['X-CSRF-Token'],'synthetic');
});
test('V027 query unknown stays retained and cannot admit a newer publication',async t=>{
 let first;const x=setup(t,(call,_calls,base)=>{if(call.init.method==='POST'){first=call;return response(receipt(call),202,location(call,base));}return response(receipt(first,'unknown','DEPENDENCY_UNAVAILABLE'));});x.hook.start(x.request);await flush();x.hook.query();await flush();assert.equal(x.confirmed.length,0);assert.equal(getState().phase,'accepted');assert.equal(x.hook.getPendingStatus(),'unknown');x.request.body.expectedRevision=9;x.hook.start(x.request);await flush();assert.equal(x.calls.length,2);
});
for(const[status,reason]of[['confirmed',null],['superseded','CANDIDATE_SUPERSEDED'],['blocked','AUTHORITY_REVOKED']])test('V027 terminal '+status+' is returned exactly, never rewritten as success',async t=>{
 const x=setup(t,call=>response(receipt(call,status,reason)));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,1);assert.equal(x.confirmed[0].status,status);assert.equal(x.confirmed[0].reason,reason);assert.equal(x.hook.getPendingStatus(),null);
});
test('V027 response loss retries precisely the original publication without resaving definition',async t=>{
 const x=setup(t,(call,calls,base)=>calls.length===1?response(null,503):response(receipt(call),202,location(call,base)));x.hook.start(x.request);await flush();const original=structuredClone(x.calls[0].body);x.request.body.expectedRevision=99;x.hook.retry();await flush();assert.deepEqual(x.calls[1].body,original);assert.equal(x.calls[1].url,'/api/v1/'+x.base+'/publish');assert.equal(x.calls[1].init.method,'POST');assert.equal(x.confirmed.length,0);assert.equal(getState().phase,'accepted');
});
for(const bad of [undefined,'https://evil.invalid/status','/api/v1/application-operations/x','/api/v1/application-workflow-operations/x'])test('V027 invalid publication Location stays unknown '+bad,async t=>{
 const x=setup(t,call=>response(receipt(call),202,bad));x.hook.start(x.request);await flush();assert.equal(getState().phase,'unconfirmed');assert.equal(x.confirmed.length,0);
});
for(const patch of [{flowId:id(8)},{version:3},{operationId:id(8)},{reason:'raw error'}])test('V027 wrong publication binding stays unknown '+JSON.stringify(patch),async t=>{
 const x=setup(t,(call,_calls,base)=>response({...receipt(call),...patch},202,location(call,base)));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(getState().phase,'unconfirmed');
});
test('V027 changed identity does not transmit publication',async t=>{const x=setup(t,()=>{throw Error('no transmit')});x.setActor(id(999));x.hook.start(x.request);await flush();assert.equal(x.calls.length,0);assert.equal(x.identity.length,1);});

test('V027 newer candidate after remount must not hide the original pending publication',async t=>{
 let first;const x=setup(t,(call,_calls,base)=>{if(call.init.method==='POST'){first=call;return response(receipt(call),202,location(call,base));}return response(receipt(first,'superseded','CANDIDATE_SUPERSEDED'));});
 x.hook.start(x.request);await flush();assert.equal(x.hook.getPendingStatus(),'unknown');
 const restored=useApplicationOperation({actorId:x.actor,scope:'publication-protocol',resource:{...x.request.resource,version:3},confirmed:v=>x.confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>{}});
 assert.equal(restored.getPendingStatus(),'unknown');restored.start({...x.request,resource:{...x.request.resource,version:3}});await flush();assert.equal(x.calls.length,1);
 restored.query();await flush();assert.equal(x.calls[1].url,location(first,x.base));assert.equal(x.confirmed[0].status,'superseded');assert.equal(x.confirmed[0].version,2);
});
