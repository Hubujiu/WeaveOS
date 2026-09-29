import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,readFileSync,existsSync,mkdirSync,writeFileSync,symlinkSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {appendDaily,cleanDaily} from '../../infra/runtime/daily-logs.mjs';

// Illustrative explicit policies, NOT approval of production retention or zone.
const policy={timeZone:'Asia/Shanghai',keepDays:2,deleteEnabled:false};
function fixture(t){const parent=mkdtempSync(join(tmpdir(),'weaveos-daily-'));t.after(()=>rmSync(parent,{recursive:true}));return parent;}
test('four kinds are stored by explicit natural day and only safe fields survive',t=>{
 const parent=fixture(t),dir=join(parent,'logs');
 const inputs={
  access:{at:'2026-09-29T16:00:00Z',request_id:'a'.repeat(32),method:'GET',path:'/login?invitationCode=secret-invitation',status:200,bytes:31},
  application:{at:'2026-09-29T15:59:59Z',msg:'session renewal failed after committed write',request_id:'b'.repeat(32),level:'WARN'},
  operations:{at:'2026-09-29T16:00:00Z',operation:'audit',status:'failed',elapsedMs:20},
  metrics:{at:'2026-09-29T15:59:59Z',ready:true,live:true,database:true,redis:true,redisUsedBytes:10,redisLimitBytes:100,containerMemoryRatios:[0.1,0.2,0.3,0.4],diskFreeRatio:0.5,certificateRemainingMs:100}
 };
 for(const [kind,input] of Object.entries(inputs)){
  appendDaily(dir,kind,{...input,password:'secret-password',Cookie:'secret-cookie',csrf:'secret-csrf',body:'secret-body'},policy);
  const day=['access','operations'].includes(kind)?'2026-09-30':'2026-09-29';
  const line=readFileSync(join(dir,kind,day+'.jsonl'),'utf8');assert.doesNotMatch(line,/secret-|invitationCode/);
  const entry=JSON.parse(line);assert.equal(entry.at,input.at);
  if(kind==='access')assert.equal(entry.path,'/login');
  if(kind==='application')assert.equal(entry.request_id,input.request_id);
 }
 assert.throws(()=>appendDaily(dir,'secrets',{at:'2026-09-29T16:00:00Z'},policy));
 assert.throws(()=>appendDaily(dir,'access',inputs.access,{...policy,timeZone:null}));
});
test('retention dry-run selects only approved kinds and inclusive natural-day cutoff',t=>{
 const dir=join(fixture(t),'logs');mkdirSync(join(dir,'access'),{recursive:true});
 for(const day of ['2026-09-27','2026-09-28','2026-09-29','2026-09-30'])writeFileSync(join(dir,'access',day+'.jsonl'),'{}\n');
 mkdirSync(join(dir,'backups'));writeFileSync(join(dir,'backups','2026-09-27.jsonl'),'keep');
 writeFileSync(join(dir,'access','unexpected.txt'),'keep');
 const options={now:new Date('2026-09-29T16:00:00Z')};
 assert.deepEqual(cleanDaily(dir,policy,options),['access/2026-09-27.jsonl','access/2026-09-28.jsonl']);
 assert.ok(existsSync(join(dir,'access','2026-09-27.jsonl')));
 assert.throws(()=>cleanDaily(dir,policy,{...options,apply:true}),/approved|disabled/i);
 assert.throws(()=>cleanDaily(dir,{...policy,keepDays:null},options));
 assert.throws(()=>cleanDaily(dir,{...policy,keepDays:0},options));
 assert.equal(readFileSync(join(dir,'backups','2026-09-27.jsonl'),'utf8'),'keep');
});
test('symlinked category and destination file cannot escape the log root',t=>{
 const parent=fixture(t),dir=join(parent,'logs'),outside=join(parent,'outside');mkdirSync(dir);mkdirSync(outside);
 symlinkSync(outside,join(dir,'access'),'junction');
 assert.throws(()=>cleanDaily(dir,policy,{now:new Date('2026-09-30T00:00:00Z')}),/link|directory/i);
 assert.throws(()=>appendDaily(dir,'access',{at:'2026-09-30T00:00:00Z',method:'GET',path:'/login',status:200,bytes:1,request_id:'a'.repeat(32)},policy),/link|directory/i);
});
