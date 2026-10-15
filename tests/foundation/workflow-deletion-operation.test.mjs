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
const id=n=>'e4e4e4e4-e4e4-4e4e-8e4e-'+String(n).padStart(12,'0');let seq=30;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
function response(data,status=200,location){return new Response(JSON.stringify({code:'OK',data,meta:null}),{status,headers:location?{Location:location}:{}});}
function setup(t,reply){
 const actor=id(++seq),resource={kind:'workflow-deletion',appId:id(1),viewId:id(2),id:id(3)},calls=[],confirmed=[],identity=[];let sessionActor=actor;
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:sessionActor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const options={actorId:actor,scope:'workflow-deletion-protocol',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>identity.push(true)};
 const request={resource,path:`applications/${id(1)}/forms/${id(2)}/workflows/${id(3)}/delete`,method:'POST',expectedStatus:202,body:{expectedRevision:7}};
 return {hook:useApplicationOperation(options),remount:()=>useApplicationOperation(options),request,calls,confirmed,identity,actor,setActor:v=>sessionActor=v};
}
const pending=call=>({operationId:call.body.operationId,flowId:id(3),status:'pending',reason:null,deletedVersions:null,deletedAt:null,completedAt:null});
const done=call=>({...pending(call),status:'deleted',deletedVersions:0,deletedAt:'2026-10-13T08:00:00.123456789Z',completedAt:'2026-10-13T08:00:01Z'});
const location=call=>`/api/v1/applications/${id(1)}/forms/${id(2)}/workflows/${id(3)}/deletions/${call.body.operationId}`;
test('V068 shared deletion hook retains accepted request and confirms only dedicated current deletion status',async t=>{
 let first;const x=setup(t,call=>{if(call.init.method==='POST'){first=call;return response(pending(call),202,location(call));}return response(done(first));});assert.doesNotThrow(()=>x.hook.start(x.request));x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');x.hook.query();await flush();assert.equal(x.calls[1].url,location(first));assert.equal(x.calls[1].init.method,'GET');assert.equal(x.calls[1].body,undefined);assert.deepEqual(x.confirmed,[done(first)]);assert.equal(x.hook.getPendingStatus(),null);
});
test('V068 engine confirmation alone stays pending application cleanup and cannot announce deleted',async t=>{
 const x=setup(t,call=>response({...done(call),status:'unknown',reason:'cleanup_retry',completedAt:null},202,location(call)));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');assert.equal(getState().phase,'accepted');
});
test('V068 already completed original POST returns exact terminal deletion without a second operation',async t=>{const x=setup(t,call=>response(done(call)));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,1);assert.equal(x.calls.length,1);assert.equal(x.hook.getPendingStatus(),null);});
test('V068 unknown deletion retry retains original flow revision and operation despite caller mutation',async t=>{
 const x=setup(t,(call,calls)=>calls.length===1?response(null,503):response(done(call)));x.hook.start(x.request);await flush();const first=structuredClone(x.calls[0].body);x.request.body.expectedRevision=99;x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);x.hook.retry();await flush();assert.equal(x.calls.length,2);assert.deepEqual(x.calls[1].body,first);assert.equal(x.confirmed.length,1);
});
test('V068 unmounted deletion recovery still addresses only the original operation',async t=>{
 let first;const x=setup(t,call=>{if(call.init.method==='POST'){first=call;return response(pending(call),202,location(call));}return response(done(first));});x.hook.start(x.request);await flush();const restored=x.remount();assert.equal(restored.getPendingStatus(),'unknown');restored.query();await flush();assert.equal(x.calls.filter(c=>c.init.method==='POST').length,1);assert.equal(x.calls[1].url,location(first));assert.equal(x.confirmed.length,1);
});
for(const variant of ['wrong-flow','wrong-operation','wrong-status','external-location','missing-location'])test('V068 deletion rejects '+variant+' without losing original recovery',async t=>{
 const x=setup(t,call=>variant==='wrong-flow'?response({...done(call),flowId:id(99)}):variant==='wrong-operation'?response({...done(call),operationId:id(99)}):variant==='wrong-status'?response(pending(call),200,location(call)):response(pending(call),202,variant==='external-location'?'https://example.invalid'+location(call):undefined));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');assert.equal(getState().phase,'unconfirmed');assert.equal(x.calls.length,1);
});
test('V068 changed actor cannot transmit a new flow deletion',async t=>{const x=setup(t,()=>{throw Error('must not send')});x.setActor(id(99));x.hook.start(x.request);await flush();assert.equal(x.calls.length,0);assert.equal(x.identity.length,1);});
