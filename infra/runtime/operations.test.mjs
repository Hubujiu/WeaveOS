import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtimeContext } from './context.mjs';
import { backupDatabase, restoreDatabase, privateFile } from './backup.mjs';
import { recoverRuntime } from './recovery.mjs';
import {runAuditTask} from './audit-task.mjs';
const c=runtimeContext(),fixture=JSON.parse(readFileSync(resolve(c.dir,'fixtures.json'),'utf8'));
const api=async(path,options={})=>fetch(`https://localhost:19443/api/v1${path}`,{...options,signal:AbortSignal.timeout(10000),headers:{Origin:'https://localhost:19443','Content-Type':'application/json',...options.headers}});
async function login(who){const r=await api('/sessions',{method:'POST',body:JSON.stringify(who)});assert.equal(r.status,201);const values=r.headers.getSetCookie();const sid=values.find(v=>v.startsWith('__Host-session='))?.split(';')[0].slice(15),csrf=values.find(v=>v.startsWith('__Host-csrf='))?.split(';')[0].slice(12);assert.ok(Boolean(sid&&csrf),'two bound cookies required');return {sid,headers:{Cookie:`__Host-session=${sid}; __Host-csrf=${csrf}`,'X-CSRF-Token':csrf}};}
async function ready(){for(let i=0;i<50;i++){try{const r=await fetch('https://localhost:19443/health/ready',{signal:AbortSignal.timeout(2000)});if(r.status===200)return;}catch{}await new Promise(r=>setTimeout(r,200));}throw new Error('runtime did not become ready');}
await ready();

test('runtime identity is separate from owner; application cannot mutate audit or read cold database',()=>{
 const roles=c.sql('weaveos_runtime',"SELECT has_table_privilege('weaveos_runtime_app','auth.authentication_events','UPDATE'),has_table_privilege('weaveos_runtime_app','auth.authentication_events','DELETE'),has_column_privilege('weaveos_runtime_app','auth.authentication_events','client_ip','SELECT'),pg_has_role('weaveos_runtime_app','auth_backup','MEMBER');");
 assert.equal(roles,'f|f|f|f');
 const db=readFileSync(resolve(c.dir,'runtime.env'),'utf8');assert.ok(db.includes('weaveos_runtime_app@')===false&&db.includes('postgres://weaveos_runtime_app:'),'private runtime config must use app identity');
 assert.equal(c.sql('weaveos_cold_archive',"SELECT has_database_privilege('weaveos_runtime_app',current_database(),'CONNECT'),has_schema_privilege('weaveos_runtime_app','archive','USAGE');"),'f|f');
});
test('actual Bootstrap-only CLI reads hot audit; ordinary, disabled and stale identities cannot',async()=>{
 const admin=await login(fixture.admin);const events=JSON.parse(c.cli('audit-read',admin.sid).toString());assert.ok(events.length>0);assert.ok(events.every(e=>Object.keys(e).length===12));
 const user=await login(fixture.user);assert.throws(()=>c.cli('audit-read',user.sid));
 c.sql('weaveos_runtime',"UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE is_bootstrap_admin;");
 assert.throws(()=>c.cli('audit-read',admin.sid));
 c.sql('weaveos_runtime',"UPDATE auth.users SET status='active',auth_version=auth_version+1 WHERE is_bootstrap_admin;");
 assert.throws(()=>c.cli('audit-read',admin.sid));
});
test('automatic monthly mover and controlled expiry run with actual maintenance login',()=>{
 assert.equal(c.sql('weaveos_runtime',"SELECT count(*) FROM auth.authentication_events WHERE request_id='runtime-monthly-fixture';"),'0');
 assert.equal(c.sql('weaveos_cold_archive',"SELECT count(*) FROM archive.authentication_events WHERE request_id='runtime-monthly-fixture';"),'1');
 c.sql('weaveos_cold_archive',"INSERT INTO archive.authentication_events (id,event_type,outcome,request_id,occurred_at) VALUES (gen_random_uuid(),'login','failure','runtime-expiry-fixture',clock_timestamp()-interval '2 years');");
 assert.equal(runAuditTask(c).status,'complete');
 assert.equal(c.sql('weaveos_cold_archive',"SELECT count(*) FROM archive.authentication_events WHERE request_id='runtime-expiry-fixture';"),'0');
});
test('real Redis memory exhaustion and restart fail closed and never restore historical sessions',async()=>{
 const old=await login(fixture.user);
 c.compose('exec','-T','redis','redis-cli','CONFIG','SET','maxmemory','1');
 try{const r=await api('/sessions',{method:'POST',body:JSON.stringify(fixture.user)});assert.equal(r.status,503);assert.equal(r.headers.has('set-cookie'),false);const current=await api('/sessions/current',{headers:old.headers});assert.equal(current.status,503);}
 finally{c.compose('exec','-T','redis','redis-cli','CONFIG','SET','maxmemory','128mb');}
 c.compose('restart','redis');await ready();
 assert.equal((await api('/sessions/current',{headers:old.headers})).status,401);
});
test('encrypted hot/cold backup restores true state, rotates generation, and rejects every old Cookie',async()=>{
 const admin=await login(fixture.admin),old=await login(fixture.user);
 const reset=await api(`/users/${fixture.resetTarget.id}/password-reset`,{method:'POST',headers:admin.headers,body:'{}'});assert.equal(reset.status,200);
 const backupDir=resolve(c.dir,'backups'),keyFile=resolve(c.dir,'secrets/backup.key');
 const options={container:c.container('postgres'),user:'weaveos_backup',keyFile,alertFile:resolve(c.dir,'public/alerts.jsonl')};
 assert.equal(c.compose('ps','--status','running','-q','audit-maintenance').toString().trim(),'','one-shot maintenance must not remain resident');
 const hot=backupDatabase({...options,database:'weaveos_runtime',backupFile:resolve(backupDir,'hot.enc')});const cold=backupDatabase({...options,database:'weaveos_cold_archive',backupFile:resolve(backupDir,'cold.enc')});
 const snapshotAt=new Date().toISOString();
 c.sql('postgres',"CREATE DATABASE weaveos_recovered; CREATE DATABASE weaveos_recovered_cold;");
 c.sql('weaveos_runtime',"INSERT INTO auth.authentication_events(event_type,outcome,request_id) VALUES('login','failure','runtime-after-snapshot');");
 const ownerOptions={...options,user:'weaveos_owner'};
 const started=Date.now();
 const transition=recoverRuntime({generation:c.generation,pause(){c.compose('stop','bff');},restore(){restoreDatabase({...ownerOptions,database:'weaveos_recovered',backupFile:resolve(backupDir,'hot.enc')});restoreDatabase({...ownerOptions,database:'weaveos_recovered_cold',backupFile:resolve(backupDir,'cold.enc')});
  c.sql('weaveos_recovered',readFileSync('infra/runtime/roles.sql','utf8'));c.sql('weaveos_recovered_cold',readFileSync('infra/runtime/cold-roles.sql','utf8'));
  assert.equal(c.sql('weaveos_recovered',"SELECT count(*) FROM auth.users WHERE status='disabled';"),'1');
  assert.ok(Number(c.sql('weaveos_recovered',"SELECT count(*) FROM auth.invitations WHERE used_by IS NOT NULL;"))>0);
  assert.equal(c.sql('weaveos_recovered',`SELECT auth_version FROM auth.users WHERE id='${fixture.resetTarget.id}';`),'2');
  assert.equal(c.sql('weaveos_recovered',"SELECT count(*) FROM auth.authentication_events WHERE request_id='runtime-after-snapshot';"),'0','a write after snapshot is actually lost in this recovery exercise');
  c.sql('postgres',"REVOKE CONNECT ON DATABASE weaveos_recovered_cold FROM PUBLIC; GRANT CONNECT ON DATABASE weaveos_recovered_cold TO weaveos_owner,auth_maintenance,auth_backup;");
 },switchGeneration(generation){
  for(const file of ['runtime.env','reader.env','maintenance.env']){const path=resolve(c.dir,file),text=readFileSync(path,'utf8').replaceAll('/weaveos_runtime?','/weaveos_recovered?').replaceAll('/weaveos_cold_archive?','/weaveos_recovered_cold?').replace(`WEAVEOS_SESSION_GENERATION=${c.generation}`,`WEAVEOS_SESSION_GENERATION=${generation}`);writeFileSync(path,text,{mode:0o600});}
 },resume(){c.compose('up','-d','bff');}});
 await ready();assert.notEqual(transition.generation,c.generation);
 assert.equal((await api('/sessions/current',{headers:old.headers})).status,401);assert.equal((await api('/sessions/current',{headers:admin.headers})).status,401);
 await login(fixture.user);assert.equal((await api('/sessions',{method:'POST',body:JSON.stringify(fixture.disabled)})).status,401);
 writeFileSync(resolve(c.dir,'public/recovery.json'),JSON.stringify({result:'passed',source:c.artifacts.commit,backupElapsedMs:hot.elapsedMs+cold.elapsedMs,recoveryObservedMs:Date.now()-started,snapshotAt,generationChanged:true,oldCookiesDenied:true,target:'local Windows-host encrypted backup outside Docker VHD; same-machine simulation, no physical disaster guarantee',rpo:'snapshot backup; later writes may be lost, no zero-loss claim'}));
});
test('actual rollback switches to previously verified artifact combination without Down or Session resurrection',async()=>{
 const previous=c.previous;assert.ok(previous&&previous.bff.imageID!==c.artifacts.bff.imageID);
 const originalBFF=c.env.WEAVEOS_BFF_IMAGE,originalWeb=c.env.WEAVEOS_WEB_IMAGE;
 c.env.WEAVEOS_BFF_IMAGE=previous.bff.imageID;c.env.WEAVEOS_WEB_IMAGE=previous.web.imageID;
 try{c.compose('up','-d','bff','nginx');await ready();const session=await login(fixture.user);assert.equal((await api('/sessions/current',{headers:session.headers})).status,200);}
 finally{c.env.WEAVEOS_BFF_IMAGE=originalBFF;c.env.WEAVEOS_WEB_IMAGE=originalWeb;c.compose('up','-d','bff','nginx');await ready();}
});
