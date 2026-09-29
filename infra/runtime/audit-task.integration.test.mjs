import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {spawn} from 'node:child_process';
import {runtimeContext} from './context.mjs';
import {runAuditTask} from './audit-task.mjs';
const c=runtimeContext();

test('default Compose has four residents and successful one-shot removes its container',()=>{
 assert.equal(c.compose('ps','--status','running','--services').toString().trim().split(/\s+/).length,4);
 assert.equal(runAuditTask(c).status,'complete');
 assert.equal(c.compose('ps','--all','--services').toString().includes('audit-maintenance'),false);
});
test('actual concurrent PostgreSQL lock rejects a second invocation and records failure',async()=>{
 const child=spawn('docker',['exec',c.container('postgres'),'psql','-X','-At','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d','weaveos_runtime','-c','SELECT pg_advisory_lock(87310008); SELECT pg_sleep(15);'],{stdio:'ignore'});
 const exited=new Promise(resolve=>child.once('exit',resolve));
 try{
  let locked=false;
  for(let i=0;i<30;i++){
   locked=c.sql('weaveos_runtime',"SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=87310008 AND granted);")==='t';
   if(locked)break;await new Promise(r=>setTimeout(r,100));
  }
  assert.equal(locked,true,'real competing process must hold the maintenance lock');
  assert.equal(runAuditTask(c).status,'failed');
  assert.ok(readFileSync(c.dir+'/alerts.jsonl','utf8').includes('AUDIT_MAINTENANCE'));
 }finally{
  c.sql('weaveos_runtime',"SELECT pg_terminate_backend(pid) FROM pg_locks WHERE locktype='advisory' AND objid=87310008 AND granted;");
  await exited;
 }
 assert.equal(runAuditTask(c).status,'complete','subsequent invocation must recover after lock release');
});
test('cold-storage permission failure preserves hot history and reports nonzero outcome',()=>{
 c.sql('weaveos_runtime',"INSERT INTO auth.authentication_events(event_type,outcome,request_id,occurred_at) VALUES('login','failure','task-cold-failure',date_trunc('month',clock_timestamp())-interval '1 day');");
 c.sql('weaveos_cold_archive','REVOKE USAGE ON SCHEMA archive FROM auth_maintenance;');
 try{
  assert.equal(runAuditTask(c).status,'failed');
  assert.equal(c.sql('weaveos_runtime',"SELECT count(*) FROM auth.authentication_events WHERE request_id='task-cold-failure';"),'1');
  assert.equal(JSON.parse(readFileSync(c.dir+'/audit-status.json','utf8')).status,'failed');
 }finally{c.sql('weaveos_cold_archive','GRANT USAGE ON SCHEMA archive TO auth_maintenance;');}
 assert.equal(runAuditTask(c).status,'complete');
 assert.equal(c.sql('weaveos_runtime',"SELECT count(*) FROM auth.authentication_events WHERE request_id='task-cold-failure';"),'0');
 assert.equal(c.sql('weaveos_cold_archive',"SELECT count(*) FROM archive.authentication_events WHERE request_id='task-cold-failure';"),'1');
});
