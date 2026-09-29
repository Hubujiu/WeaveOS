import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {runAuditTask} from '../../infra/runtime/audit-task.mjs';
import {sampleRuntime} from '../../infra/runtime/probe.mjs';

// FR05: orchestration boundary; real PG/Compose semantics are tested separately.
for(const fails of [false,true])test(`one-shot orchestration records ${fails?'failure':'success'} without raw output`,()=>{
 const dir=mkdtempSync(join(tmpdir(),'weaveos-audit-test-'));
 const calls=[];
 try{
  const result=runAuditTask({dir,args:['compose','-p','isolated','-f','compose.json'],command(bin,args,options){calls.push({bin,args,options});if(fails)throw Error('password=must-not-escape');return Buffer.from('private-diagnostic-must-not-escape');}});
  assert.equal(result.status,fails?'failed':'complete');
  assert.equal(calls[0].bin,'docker');
  assert.deepEqual(calls[0].args,['compose','-p','isolated','-f','compose.json','--profile','maintenance','run','--rm','--no-deps','-T','audit-maintenance']);
  assert.equal(calls[0].options.timeout,330000);
  const receipt=JSON.parse(readFileSync(join(dir,'audit-status.json'),'utf8'));
  assert.equal(receipt.status,result.status);
  assert.ok(Number.isFinite(Date.parse(receipt.at)));
  assert.doesNotMatch(JSON.stringify(receipt),/private|password|escape/);
  if(fails)assert.deepEqual(JSON.parse(readFileSync(join(dir,'alerts.jsonl'),'utf8')).codes,['AUDIT_MAINTENANCE']);
 }finally{rmSync(dir,{recursive:true});}
});
test('memory probe samples the four resident containers, not the exited task',async()=>{
 const names=[];
 await sampleRuntime({dir:tmpdir(),sql:()=> '1',compose:()=>Buffer.from('used_memory:10\nmaxmemory:100\n'),container:name=>{names.push(name);return name;},command:()=>Buffer.from('1%')});
 assert.deepEqual(names.sort(),['bff','nginx','postgres','redis']);
});
