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
const id=n=>'bcbcbcbc-bcbc-4cbc-8cbc-'+String(n).padStart(12,'0');let seq=20;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
const response=(data,status=200)=>new Response(JSON.stringify({code:'OK',data,meta:null}),{status});
function setup(t,reply,action='return'){
 const actor=id(++seq),resource={kind:'instance-action',appId:id(1),viewId:id(2),id:id(3),instanceId:id(4)},calls=[],confirmed=[];
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:actor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const hook=useApplicationOperation({actorId:actor,scope:'reason-operation',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>{}});
 const request={resource,path:`applications/${id(1)}/forms/${id(2)}/records/${id(3)}/workflow-instances/${id(4)}/actions`,method:'POST',expectedStatus:202,body:{action,basisToken:'original-basis',...(action==='return'?{targetNodeId:id(5),reason:'  原因\n🦊  '}:{})}};
 return {hook,request,calls,confirmed,actor};
}
const receipt=(c,instanceState='active')=>({operationId:c.body.operationId,commandId:id(10),instanceId:id(4),status:'success',instanceState,sequence:4});
test('P25 shared return confirms actual active execution receipt',async t=>{const x=setup(t,c=>response(receipt(c)));x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);assert.equal(x.confirmed.length,1);assert.equal(x.confirmed[0].instanceState,'active');});
test('P25 shared return cannot confirm a withdrawal receipt',async t=>{const x=setup(t,c=>response(receipt(c,'withdrawn')));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');});
test('P25 unknown return retries immutable target reason basis and operation key',async t=>{const x=setup(t,(c,calls)=>calls.length===1?response(null,503):response(receipt(c)));x.hook.start(x.request);await flush();const original=structuredClone(x.calls[0].body);x.request.body.reason='changed';x.request.body.targetNodeId=id(99);x.request.body.basisToken='changed';x.hook.retry();await flush();assert.equal(x.calls.length,2);assert.deepEqual(x.calls[1].body,original);assert.equal(x.confirmed.length,1);});
test('P25 shared withdrawal still requires withdrawn execution receipt',async t=>{const x=setup(t,c=>response(receipt(c,'withdrawn')),'withdraw');x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,1);});

test('P25 remount restores unconfirmed original return and queries it without another write',async t=>{
 let original;const x=setup(t,c=>{if(c.init.method==='POST'){original=c;return new Response(JSON.stringify({code:'OK',data:{operationId:c.body.operationId,commandId:id(10),instanceId:id(4),status:'pending'},meta:null}),{status:202,headers:{Location:'/api/v1/application-workflow-operations/'+c.body.operationId}});}return response(receipt(original));});x.hook.start(x.request);await flush();assert.equal(getState().phase,'accepted');
 const restored=useApplicationOperation({actorId:x.actor,scope:'reason-operation',resource:x.request.resource,confirmed:v=>x.confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>{}});assert.equal(restored.phase,'unconfirmed');assert.deepEqual(restored.packet.body,original.body);restored.query();await flush();assert.equal(x.calls.filter(c=>c.init.method==='POST').length,1);assert.equal(x.confirmed.length,1);
});
