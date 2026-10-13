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
const id=n=>'e8e8e8e8-e8e8-4e8e-8e8e-'+String(n).padStart(12,'0');let seq=30;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
function response(data,status=200,location){return new Response(JSON.stringify({code:'OK',data,meta:null}),{status,headers:location?{Location:location}:{}});}
const manifest={format:'weaveos.structure-template',version:1,application:{id:id(1),name:'模板'},directories:[],tables:[],forms:[],workflows:[],permissionGroups:[]};
function setup(t,reply){
 const actor=id(++seq),resource={kind:'template-import'},calls=[],confirmed=[],identity=[];let sessionActor=actor;
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:sessionActor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const options={actorId:actor,scope:'template-import',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>identity.push(true)};
 const request={resource,path:'application-templates/import',method:'POST',expectedStatus:201,body:{manifest:structuredClone(manifest),bindings:[]}};
 return {hook:useApplicationOperation(options),remount:()=>useApplicationOperation(options),request,calls,confirmed,identity,setActor:v=>sessionActor=v};
}
function start(x){let error=null;try{x.hook.start(x.request);}catch(e){error=e.message;}assert.equal(error,null,'the exact V070 original import packet must be admitted');}
const receipt=call=>({operationId:call.body.operationId,appId:id(2),policyRevision:1,structureVersion:1,status:'imported'}),location='/api/v1/applications/'+id(2);
test('V070 shared import hook confirms one 201 new application and never publishes a workflow',async t=>{const x=setup(t,call=>response(receipt(call),201,location));start(x);start(x);await flush();assert.equal(x.calls.length,1);assert.equal(x.calls[0].url,'/api/v1/application-templates/import');assert.deepEqual(x.calls[0].body,{manifest,bindings:[],operationId:x.calls[0].body.operationId});assert.deepEqual(x.confirmed,[receipt(x.calls[0])]);assert.equal(x.hook.getPendingStatus(),null);});
test('V070 unknown import retries exactly the original manifest bindings and key after caller mutation',async t=>{const x=setup(t,(call,calls)=>calls.length===1?response(null,503):response(receipt(call),201,location));start(x);await flush();const original=structuredClone(x.calls[0].body);x.request.body.manifest.application.name='new file';x.request.body.bindings.push({kind:'user',sourceId:id(4),targetId:id(5)});start(x);await flush();assert.equal(x.calls.length,1);x.hook.retry();await flush();assert.deepEqual(x.calls[1].body,original);assert.equal(x.confirmed.length,1);});
test('V070 original import recovery after unmount queries generic committed 201 without creating again',async t=>{let first;const x=setup(t,call=>{if(call.init.method==='POST'){first=call;return response(null,503);}return response({operationId:first.body.operationId,status:'confirmed',httpStatus:201,result:receipt(first),location});});start(x);await flush();const restored=x.remount();assert.equal(restored.getPendingStatus(),'unknown');restored.query();await flush();assert.equal(x.calls[1].url,'/api/v1/application-operations/'+first.body.operationId);assert.equal(x.calls[1].init.method,'GET');assert.equal(x.calls[1].body,undefined);assert.deepEqual(x.confirmed,[receipt(first)]);assert.equal(x.calls.filter(c=>c.init.method==='POST').length,1);});
for(const mode of ['wrong-operation','wrong-status','external-location','wrong-app-location','missing-location','extra-data'])test('V070 import keeps uncertainty after '+mode,async t=>{const x=setup(t,call=>response(mode==='wrong-operation'?{...receipt(call),operationId:id(99)}:mode==='extra-data'?{...receipt(call),ownerUserId:id(99)}:receipt(call),mode==='wrong-status'?200:201,mode==='external-location'?'https://example.invalid'+location:mode==='wrong-app-location'?'/api/v1/applications/'+id(99):mode==='missing-location'?undefined:location));start(x);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');assert.equal(getState().phase,'unconfirmed');});
test('V070 current identity change before import transmits no application creation',async t=>{const x=setup(t,()=>{throw Error('must not send');});x.setActor(id(99));start(x);await flush();assert.equal(x.calls.length,0);assert.equal(x.identity.length,1);});
