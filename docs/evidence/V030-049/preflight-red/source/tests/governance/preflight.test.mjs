import test from 'node:test';
import assert from 'node:assert/strict';
import {runPreflight} from '../../scripts/preflight.mjs';
const expected=['policy-tests','repository','tasks','format','contract-tests','contract-lint','secrets'];
const env={GITHUB_EVENT_NAME:'pull_request',GITHUB_EVENT_PATH:'/tmp/synthetic-pr.json',GITHUB_SHA:'f'.repeat(40)};
const ok={status:0,stdout:'',stderr:'',signal:null};
function invoke(response=()=>ok){
 const calls=[];
 let error;
 try{runPreflight({root:'/tmp/synthetic-repo',env,execute:step=>{calls.push(step);return response(step,calls.length-1);}});}catch(e){error=e;}
 return {calls,error};
}
test('PRD V049: all required checks run in the declared cheap-first order',()=>{
 const {calls,error}=invoke();assert.ifError(error);
 assert.deepEqual(calls.map(c=>c.id),expected);
 for(const step of calls){assert.equal(step.env.GITHUB_EVENT_NAME,env.GITHUB_EVENT_NAME);assert.equal(step.env.GITHUB_EVENT_PATH,env.GITHUB_EVENT_PATH);assert.equal(step.env.GITHUB_SHA,env.GITHUB_SHA);assert.ok(Array.isArray(step.args));}
 const byId=Object.fromEntries(calls.map(c=>[c.id,c]));
 for(const [id,path] of [['repository','scripts/verify-repo.mjs'],['tasks','scripts/check-tasks.mjs']])assert.ok(byId[id].args.includes(path));
 assert.equal(byId.format.command,'gofmt');assert.deepEqual(byId.format.args,['-l','.']);assert.equal(byId.format.cwd,'/tmp/synthetic-repo/services/bff');
 assert.equal(byId['contract-lint'].command,'pnpm');assert.deepEqual(byId['contract-lint'].args,['exec','redocly','lint','contracts/openapi/openapi.json']);
 assert.ok(byId['contract-tests'].args.some(a=>a==='contracts'||/^contracts\/[^/]+\.test\.mjs$/.test(a)));
 assert.ok(byId['policy-tests'].args.some(a=>a==='tests/governance'||/^tests\/governance\/[^/]+\.test\.mjs$/.test(a)));
 assert.ok(byId['policy-tests'].args.some(a=>a==='tests/foundation'||/^tests\/foundation\/[^/]+\.test\.mjs$/.test(a)));
 assert.ok(byId.secrets.args.includes('infra/runtime/security.test.mjs'));
 assert.ok(byId.secrets.args.includes('--test-name-pattern=^tracked source archive passes a redacted Gitleaks secret scan$'));
});
for(let index=0;index<expected.length;index++)test(`PRD V049: ${expected[index]} failure prevents every later check`,()=>{
 const {calls,error}=invoke((step,i)=>i===index?{...ok,status:7}:ok);
 assert.ok(error instanceof Error,'failure cannot publish success');
 assert.deepEqual(calls.map(c=>c.id),expected.slice(0,index+1));
});
for(const bad of [{...ok,error:Object.assign(new Error('PRIVATE_SENTINEL'),{code:'EPERM'})},{...ok,status:null,signal:'SIGTERM'},{...ok,status:null},{...ok,status:1,stderr:'PRIVATE_SENTINEL'}])test('PRD V049: tool errors and indeterminate execution fail closed without exposing output',()=>{
 const {calls,error}=invoke(()=>bad);assert.ok(error instanceof Error);assert.equal(calls.length,1);assert.ok(!String(error).includes('PRIVATE_SENTINEL'));
});
test('PRD V049: gofmt exit zero with changed files is a format failure',()=>{
 const {calls,error}=invoke(step=>step.id==='format'?{...ok,stdout:'internal/example.go\n'}:ok);
 assert.ok(error instanceof Error);assert.deepEqual(calls.map(c=>c.id),expected.slice(0,4));
});
