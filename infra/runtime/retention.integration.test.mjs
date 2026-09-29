import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,existsSync,readFileSync,rmSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {resolve,join} from 'node:path';
import {runtimeContext} from './context.mjs';
const c=runtimeContext();
test('actual operations CLI dry-runs then applies Q20 only to four approved directories',()=>{
 const dir=mkdtempSync(join(c.dir,'retention-cli-'));
 try{
  writeFileSync(join(dir,'log-policy.json'),JSON.stringify({timeZone:'Asia/Shanghai',keepDays:30,deleteEnabled:true}));
  for(const kind of ['access','application','operations','metrics']){
   mkdirSync(join(dir,'logs',kind),{recursive:true});
   for(const date of ['2000-01-01','2999-01-01'])writeFileSync(join(dir,'logs',kind,date+'.jsonl'),'synthetic');
  }
  writeFileSync(join(dir,'backup.enc'),'preserved');
  const run=(...args)=>JSON.parse(execFileSync('docker',['run','--rm','--network','none',...(typeof process.getuid==='function'?['--user',`${process.getuid()}:${process.getgid()}`]:[]),'--mount',`type=bind,src=${dir},dst=/opt/weaveos-v010`,'--mount',`type=bind,src=${resolve('infra')},dst=/opt/weaveos-v010/infra,readonly`,'mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27','node','/opt/weaveos-v010/infra/server/operations.mjs','retention',...args],{encoding:'utf8',stdio:'pipe',timeout:30000}));
  const preview=run();assert.equal(preview.dryRun,true);assert.equal(preview.files.length,4);
  for(const path of preview.files)assert.ok(existsSync(join(dir,'logs',path)));
  const applied=run('--apply');assert.equal(applied.dryRun,false);assert.deepEqual(applied.files,preview.files);
  for(const path of applied.files)assert.equal(existsSync(join(dir,'logs',path)),false);
  for(const kind of ['access','application','operations','metrics'])assert.ok(existsSync(join(dir,'logs',kind,'2999-01-01.jsonl')));
  assert.equal(readFileSync(join(dir,'backup.enc'),'utf8'),'preserved');
 }finally{rmSync(dir,{recursive:true});}
});
