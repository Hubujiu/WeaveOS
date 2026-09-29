import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,readFileSync,rmSync,existsSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {collectLogs} from '../../infra/runtime/log-collection.mjs';
const policy={timeZone:'UTC',keepDays:3,deleteEnabled:false};
test('collector records access stdout and application stderr, advances window, and excludes raw text',()=>{
 const dir=mkdtempSync(join(tmpdir(),'weaveos-collector-')),calls=[];
 try{
  const context={dir,container:name=>name};
  const readLogs=(bin,args)=>{
   calls.push({bin,args});
   return {status:0,stdout:args.at(-1)==='nginx'?`2026-09-29T00:00:01.000000000Z {"at":"2026-09-29T00:00:01Z","request_id":"${'a'.repeat(32)}","method":"GET","path":"/login?password=hidden","status":200,"bytes":12}\n`:'',stderr:args.at(-1)==='bff'?'2026-09-29T00:00:02.000000000Z {"time":"2026-09-29T00:00:02Z","level":"ERROR","msg":"BFF host failed","error":"hidden-password"}\n':'nginx raw startup hidden-cookie\n'};
  };
  assert.equal(collectLogs(context,policy,{now:new Date('2026-09-29T00:00:10Z'),readLogs}).status,'complete');
  assert.equal(calls.length,2);assert.ok(calls.every(c=>c.bin==='docker'&&c.args.includes('--timestamps')));
  for(const kind of ['access','application'])assert.doesNotMatch(readFileSync(join(dir,'logs',kind,'2026-09-29.jsonl'),'utf8'),/hidden|password|cookie/);
  assert.equal(JSON.parse(readFileSync(join(dir,'log-cursor.json'))).until,'2026-09-29T00:00:10.000Z');
  calls.length=0;
  collectLogs(context,policy,{now:new Date('2026-09-29T00:00:20Z'),readLogs});
  assert.equal(calls[0].args[calls[0].args.indexOf('--since')+1],'2026-09-29T00:00:10.000Z');
  assert.equal(readFileSync(join(dir,'logs','access','2026-09-29.jsonl'),'utf8').trim().split('\n').length,1,'previous time window must not be copied again');
 }finally{rmSync(dir,{recursive:true});}
});
test('failed docker collection does not advance cursor or publish raw diagnostics',()=>{
 const dir=mkdtempSync(join(tmpdir(),'weaveos-collector-'));
 try{
  assert.throws(()=>collectLogs({dir,container:name=>name},policy,{now:new Date('2026-09-29T00:00:10Z'),readLogs:()=>({status:1,stderr:'private-password'})}),/collection failed/);
  assert.equal(existsSync(join(dir,'log-cursor.json')),false);
 }finally{rmSync(dir,{recursive:true});}
});
