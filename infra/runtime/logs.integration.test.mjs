import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,readdirSync,writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {runtimeContext} from './context.mjs';
import {collectLogs} from './log-collection.mjs';
import {configuredLog} from './log-policy.mjs';
import {sampleRuntime} from './probe.mjs';
import {runAuditTask} from './audit-task.mjs';
const c=runtimeContext();
test('actual Docker stdout/stderr, task result and live metrics reach four safe daily files',async()=>{
 // Isolated illustrative policy only; no retention deletion is performed.
 const policy={timeZone:'UTC',keepDays:2,deleteEnabled:false};
 writeFileSync(join(c.dir,'log-policy.json'),JSON.stringify(policy),{mode:0o600});
 const from=new Date();writeFileSync(join(c.dir,'log-cursor.json'),JSON.stringify({until:new Date(from.getTime()-3600000).toISOString()}));
 const response=await fetch('https://localhost:19443/login?invitationCode=DO_NOT_STORE_THIS_QUERY',{headers:{Cookie:'private=DO_NOT_STORE_THIS_COOKIE'}});
 assert.equal(response.status,200);await response.text();
 await new Promise(r=>setTimeout(r,100));
 assert.equal(collectLogs(c,policy).status,'complete');
 assert.equal(runAuditTask(c).status,'complete');
 configuredLog(c,'metrics',{at:new Date().toISOString(),...await sampleRuntime(c)});
 for(const kind of ['access','application','operations','metrics']){
  const dir=join(c.dir,'logs',kind),files=readdirSync(dir);assert.ok(files.length>0,kind);
  const raw=files.map(file=>readFileSync(join(dir,file),'utf8')).join('');
  assert.doesNotMatch(raw,/DO_NOT_STORE|invitationCode|__Host-session|__Host-csrf|password/i);
  for(const line of raw.trim().split('\n'))assert.ok(Number.isFinite(Date.parse(JSON.parse(line).at)));
 }
});
