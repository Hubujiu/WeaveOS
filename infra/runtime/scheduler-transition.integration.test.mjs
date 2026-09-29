import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {runtimeContext} from './context.mjs';
import {runAuditTask} from './audit-task.mjs';
import {serverSchedules} from '../server/plan.mjs';
import {validateInstalledMaintenance} from '../server/deploy/policy.mjs';
const c=runtimeContext();
test('reviewed scheduler switch and rollback preserve one source of scheduling and actual 5/4 resident counts',()=>{
 const current=JSON.parse(readFileSync('infra/runtime/compose.json','utf8'));
 const legacy=structuredClone(current),task=legacy.services['audit-maintenance'];
 delete task.profiles;task.command=['/app/audit-maintenance'];task.restart='unless-stopped';task.image=c.previous.bff.imageID;
 const file=join(c.dir,'scheduler-drill.json'),cron=join(c.dir,'scheduler-drill.cron');
 const args=['compose','-p',c.project,'-f',file];
 const compose=(...tail)=>c.command('docker',[...args,...tail]);
 const legacyCron=serverSchedules().split('\n').filter(line=>!line.includes('operations.mjs audit')).join('\n');
 const count=()=>compose('ps','--status','running','--services').toString().trim().split(/\s+/).length;
 const stopTask=()=>{compose('stop','--timeout','10','audit-maintenance');compose('rm','-f','audit-maintenance');};
 // Isolated files stand in for reviewed /etc/cron.d and installed config. Docker,
 // the old scheduler binary, current one-shot binary and both PG stores are real.
 writeFileSync(file,JSON.stringify(legacy));writeFileSync(cron,legacyCron);
 try{
  compose('up','-d','--no-deps','--pull','never','audit-maintenance');assert.equal(count(),5);
  assert.throws(()=>validateInstalledMaintenance(legacy),/reviewed upgrade/);
  writeFileSync(cron,'# maintenance scheduling paused for reviewed switch\n');
  stopTask();
  writeFileSync(file,JSON.stringify(current));validateInstalledMaintenance(current);
  assert.equal(runAuditTask({...c,args}).status,'complete');
  writeFileSync(cron,serverSchedules());
  compose('up','-d','--no-deps','--pull','never');assert.equal(count(),4);
  assert.equal(readFileSync(cron,'utf8').split('\n').filter(l=>l.includes('operations.mjs audit')).length,1);
  // Rollback disables new Cron first, drains the task, then restores old mode.
  writeFileSync(cron,'# maintenance paused for rollback\n');stopTask();
  writeFileSync(file,JSON.stringify(legacy));
  compose('up','-d','--no-deps','--pull','never','audit-maintenance');
  writeFileSync(cron,legacyCron);assert.equal(count(),5);
  assert.equal(readFileSync(cron,'utf8').includes('operations.mjs audit'),false);
 }finally{
  writeFileSync(cron,'# drill complete; no host schedule installed\n');
  stopTask();writeFileSync(file,JSON.stringify(current));
  compose('up','-d','--no-deps','--pull','never');
 }
 assert.equal(count(),4);
});
