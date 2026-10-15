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
const id=n=>'adadadad-adad-4dad-8dad-'+String(n).padStart(12,'0');let seq=10;
async function flush(){for(let i=0;i<12;i++)await new Promise(resolve=>setImmediate(resolve));}
function response(data,status=200){return new Response(JSON.stringify({code:'OK',data,meta:null}),{status});}
function setup(t,reply,create=false){
 const actor=id(++seq),resource={kind:'workflow-definition',appId:id(1),viewId:id(2),id:id(3)},calls=[],confirmed=[],identity=[];let sessionActor=actor;
 for(const[key,value]of[['document',{cookie:'__Host-csrf=synthetic'}],['window',{location:{origin:'https://test.invalid'}}]]){const old=Object.getOwnPropertyDescriptor(globalThis,key);Object.defineProperty(globalThis,key,{value,configurable:true});t.after(()=>{if(old)Object.defineProperty(globalThis,key,old);else delete globalThis[key];});}
 t.mock.method(globalThis,'fetch',async(url,init)=>{if(url==='/api/v1/sessions/current')return response({id:sessionActor});calls.push({url,init,body:init.body?JSON.parse(init.body):undefined});return reply(calls.at(-1),calls);});
 const hook=useApplicationOperation({actorId:actor,scope:'definition-protocol',resource,confirmed:v=>confirmed.push(v),unauthorized:()=>{},identityMismatch:()=>identity.push(true)});
 const request={resource,path:`applications/${id(1)}/forms/${id(2)}/workflows/${id(3)}/definition`,method:'PUT',expectedStatus:create?201:200,body:{name:'财务审批',expectedRevision:create?0:4,expectedSchemaVersion:2,allowWithdraw:true,triggers:[{event:'manual',condition:null}],graph:{version:1,nodes:[{id:id(4),kind:'start'},{id:id(5),kind:'end'}],edges:[{from:id(4),to:id(5)}]}}};
 return {hook,request,calls,confirmed,identity,actor,setActor:v=>sessionActor=v};
}
const receipt=call=>({operationId:call.body.operationId,id:id(3),revision:call.body.expectedRevision+1,currentVersion:0,candidateVersion:1,state:'disabled'});
for(const create of [false,true])test('V020 shared definition hook confirms '+(create?'initial 201 without record Location':'existing 200')+' exactly once',async t=>{
 const x=setup(t,call=>response(receipt(call),create?201:200),create);assert.doesNotThrow(()=>x.hook.start(x.request));x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);assert.equal(x.confirmed.length,1);assert.equal(x.hook.getPendingStatus(),null);
 assert.equal(x.calls[0].init.method,'PUT');assert.equal(x.calls[0].init.headers['X-Expected-Actor-Id'],x.actor);assert.equal(x.calls[0].init.headers['X-CSRF-Token'],'synthetic');assert.deepEqual(Object.keys(x.calls[0].body).sort(),['allowWithdraw','expectedRevision','expectedSchemaVersion','graph','name','operationId','triggers']);
});
test('V020 unknown definition retry preserves original graph CAS and operation key',async t=>{
 const x=setup(t,(call,calls)=>calls.length===1?response(null,503):response(receipt(call)));x.hook.start(x.request);await flush();assert.equal(x.hook.getPendingStatus(),'unknown');const first=structuredClone(x.calls[0].body);x.request.body.name='后来输入';x.request.body.graph.nodes.reverse();x.request.body.expectedRevision=99;x.hook.start(x.request);await flush();assert.equal(x.calls.length,1);x.hook.retry();await flush();assert.equal(x.calls.length,2);assert.deepEqual(x.calls[1].body,first);assert.equal(x.confirmed.length,1);
});
test('V020 initial definition result recovery uses the original generic operation without fake Location',async t=>{
 let first;const x=setup(t,call=>{if(call.init.method==='PUT'){first=call;return response(null,503);}return response({operationId:first.body.operationId,status:'confirmed',httpStatus:201,result:receipt(first),location:null});},true);x.hook.start(x.request);await flush();x.hook.query();await flush();assert.equal(x.confirmed.length,1);assert.equal(x.calls[1].url,'/api/v1/application-operations/'+first.body.operationId);assert.equal(x.calls[1].init.method,'GET');
});
for(const patch of [{id:id(99)},{operationId:id(99)},{revision:4},{state:'closing'},{deploymentId:'fake'}])test('V020 definition receipt remains unknown '+JSON.stringify(patch),async t=>{
 const x=setup(t,call=>response({...receipt(call),...patch}));x.hook.start(x.request);await flush();assert.equal(x.confirmed.length,0);assert.equal(x.hook.getPendingStatus(),'unknown');assert.equal(getState().phase,'unconfirmed');
});
test('V020 changed identity cannot transmit a definition save',async t=>{
 const x=setup(t,()=>{throw Error('must not send')});x.setActor(id(999));x.hook.start(x.request);await flush();assert.equal(x.calls.length,0);assert.equal(x.identity.length,1);
});
