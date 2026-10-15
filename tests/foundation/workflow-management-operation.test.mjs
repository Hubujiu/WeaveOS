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
const id=n=>'afafafaf-afaf-4faf-8faf-'+String(n).padStart(12,'0');let seq=20;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
const response=(data,status=200)=>new Response(JSON.stringify({code:'OK',data,meta:null}),{status});
function setup(t,reply,action='close'){
 const actor=id(++seq),resource={kind:'workflow-management',appId:id(1),viewId:id(2),id:id(3),action},calls=[],confirmed=[];
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:actor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const hook=useApplicationOperation({actorId:actor,scope:'management-protocol',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>{}});
 const request={resource,path:`applications/${id(1)}/forms/${id(2)}/workflows/${id(3)}/${action}`,method:'POST',expectedStatus:200,body:{expectedRevision:4}};
 return {hook,request,calls,confirmed};
}
const receipt=(call,state='closing')=>({operationId:call.body.operationId,id:id(3),revision:5,currentVersion:1,candidateVersion:2,state});
for(const [action,state]of [['enable','enabled'],['close','closing'],['close','disabled']])test('V020 shared management confirms exact '+action+'/'+state+' once without rewriting state',async t=>{
 const x=setup(t,c=>response(receipt(c,state)),action);x.hook.start(x.request);x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);assert.equal(x.calls[0].init.method,'POST');assert.deepEqual(Object.keys(x.calls[0].body).sort(),['expectedRevision','operationId']);assert.equal(x.confirmed.length,1);assert.equal(x.confirmed[0].state,state);
});
test('V020 management unknown retry retains same action CAS and operation',async t=>{
 const x=setup(t,(c,calls)=>calls.length===1?response(null,503):response(receipt(c)));x.hook.start(x.request);await flush();const first=structuredClone(x.calls[0]);assert.equal(x.hook.getPendingStatus(),'unknown');x.request.body.expectedRevision=99;x.hook.start(x.request);x.hook.retry();await flush();assert.equal(x.calls.length,2);assert.deepEqual(x.calls[1].body,first.body);assert.equal(x.calls[1].url,first.url);assert.equal(x.confirmed.length,1);
});
test('V020 management generic original operation lookup preserves closing result',async t=>{
 let original;const x=setup(t,c=>{if(c.init.method==='POST'){original=c;return response(null,503);}return response({operationId:original.body.operationId,status:'confirmed',httpStatus:200,location:null,result:receipt(original)});});x.hook.start(x.request);await flush();x.hook.query();await flush();assert.equal(x.calls[1].url,'/api/v1/application-operations/'+original.body.operationId);assert.equal(x.confirmed.length,1);assert.equal(x.confirmed[0].state,'closing');
});
test('V020 wrong management state remains unknown instead of claiming enabled',async t=>{
 const x=setup(t,c=>response(receipt(c,'closing')),'enable');x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');
});
