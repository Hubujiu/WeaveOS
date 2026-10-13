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
const id=n=>'bcbcbcbc-bcbc-4cbc-8cbc-'+String(n).padStart(12,'0');let seq=30;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
function response(data,status=200,location){return new Response(status===204?null:JSON.stringify({code:'OK',data,meta:null}),{status,headers:location?{Location:location}:{}});}
function setup(t,reply,kind='form'){
 const actor=id(++seq),resource={kind:'structure-deletion',appId:id(1),resourceKind:kind,id:kind==='application'?id(1):id(2)},calls=[],confirmed=[],identity=[];let sessionActor=actor;
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:sessionActor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const options={actorId:actor,scope:'structural-deletion',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>identity.push(true)};
 const path='applications/'+resource.appId+(kind==='application'?'':'/'+{directory:'directories',table:'tables',form:'forms'}[kind]+'/'+resource.id)+'/deletion';
 const request={resource,path,method:'POST',expectedStatus:200,body:{expectedStructureVersion:2,expectedResourceVersion:kind==='directory'?0:3}};
 return {hook:useApplicationOperation(options),remount:()=>useApplicationOperation(options),request,calls,confirmed,identity,setActor:v=>sessionActor=v};
}
function start(x){let error=null;try{x.hook.start(x.request);}catch(e){error=e.message;}assert.equal(error,null,'the exact original V073 deletion packet must be admitted');}
const receipt=(c,kind='form')=>({operationId:c.body.operationId,appId:id(1),resourceKind:kind,id:kind==='application'?id(1):id(2),structureVersion:3,deleted:true});
for(const kind of ['application','directory','table','form'])test('P24 shared deletion confirms one exact '+kind+' result without a second write',async t=>{
 const x=setup(t,c=>response(receipt(c,kind)),kind);start(x);start(x);await flush();assert.equal(x.calls.length,1);assert.equal(x.calls[0].url,'/api/v1/'+x.request.path);assert.deepEqual(x.calls[0].body,{...x.request.body,operationId:x.calls[0].body.operationId});assert.deepEqual(x.confirmed,[receipt(x.calls[0],kind)]);assert.equal(x.hook.getPendingStatus(),null);
});
test('P24 unknown deletion retries only its immutable original versions resource and operation',async t=>{
 const x=setup(t,(c,cs)=>cs.length===1?response(null,503):response(receipt(c)));start(x);await flush();const original=structuredClone(x.calls[0]);x.request.body.expectedStructureVersion=99;x.request.body.expectedResourceVersion=100;start(x);await flush();assert.equal(x.calls.length,1);x.hook.retry();await flush();assert.deepEqual(x.calls[1].body,original.body);assert.equal(x.calls[1].url,original.url);assert.equal(x.confirmed.length,1);
});
test('P24 deleted application recovery queries the original operation without rereading inaccessible scope',async t=>{
 let first;const x=setup(t,c=>{if(c.init.method==='POST'){first=c;return response(null,503);}assert.equal(c.url,'/api/v1/application-operations/'+first.body.operationId);return response({operationId:first.body.operationId,status:'confirmed',httpStatus:200,result:receipt(first,'application'),location:null});},'application');start(x);await flush();const restored=x.remount();assert.equal(restored.getPendingStatus(),'unknown');restored.query();await flush();assert.equal(x.calls.length,2);assert.deepEqual(x.confirmed,[receipt(first,'application')]);assert.equal(x.calls.filter(c=>c.init.method==='POST').length,1);
});
for(const mode of ['wrong-version','wrong-resource','extra-values','location','202','204'])test('P24 deletion retains original uncertainty for '+mode,async t=>{
 const x=setup(t,c=>response({...receipt(c),...(mode==='wrong-version'?{structureVersion:99}:mode==='wrong-resource'?{id:id(9)}:mode==='extra-values'?{values:{}}:{})},mode==='202'?202:mode==='204'?204:200,mode==='location'?'https://other.invalid/result':undefined));start(x);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');assert.equal(getState().phase,'unconfirmed');
});
test('P24 identity change before deletion sends zero writes',async t=>{
 const x=setup(t,()=>{throw Error('must not send')});x.setActor(id(99));start(x);await flush();assert.equal(x.calls.length,0);assert.equal(x.identity.length,1);
});
