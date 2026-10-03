import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {readFileSync,mkdtempSync,mkdirSync,copyFileSync,rmSync} from 'node:fs';
import {resolve} from 'node:path';
import {applyMigrations} from './migrate.mjs';
import {applyPersonnelRoles} from './personnel-upgrade.mjs';

test('B5a PR21 cold-first expansion, restricted runtime and actual backup restore preserve application policies',async()=>{
 const container='weaveos-v010-b5-upgrade-'+process.pid;
 const run=(args,options={})=>execFileSync('docker',args,{stdio:'pipe',timeout:120000,...options});
 let scratch;
 try{
  run(['run','-d','--name',container,'-e','POSTGRES_HOST_AUTH_METHOD=trust','-e','POSTGRES_USER=weaveos_owner','-e','POSTGRES_DB=weaveos_b5_hot','postgres:18.6']);
  for(let i=0;i<60;i++){try{run(['exec',container,'pg_isready','-h','127.0.0.1','-U','weaveos_owner']);break;}catch{await new Promise(r=>setTimeout(r,250));}}
  for(const db of ['weaveos_b5_cold','weaveos_b5_restored'])run(['exec',container,'createdb','-U','weaveos_owner',db]);
  const env=db=>'GOOSE_DRIVER=postgres\nGOOSE_DBSTRING=postgres://weaveos_owner@127.0.0.1:5432/'+db+'?sslmode=disable\n';
  const sql=(db,text)=>run(['exec','-i',container,'psql','-X','-At','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d',db],{input:text,encoding:'utf8'}).trim();
  const hot=text=>sql('weaveos_b5_hot',text);
  mkdirSync('.work/b5-upgrade',{recursive:true});scratch=mkdtempSync(resolve('.work/b5-upgrade/initial-'));
  for(const dir of ['migrations','archive-migrations']){mkdirSync(scratch+'/'+dir);const files=dir==='migrations'?['00001_auth.sql','00002_personnel.sql','00003_query_drafts.sql','00004_query_revision_writers.sql','00005_table_presets.sql']:['00001_archive.sql','00002_personnel_audit.sql'];for(const f of files)copyFileSync('db/'+dir+'/'+f,scratch+'/'+dir+'/'+f);}
  const goose=resolve(process.env.WEAVEOS_TEST_GOOSE??'.work/personnel/goose');
  applyMigrations({container,goose,directory:scratch+'/archive-migrations',env:env('weaveos_b5_cold')});
  applyMigrations({container,goose,directory:scratch+'/migrations',env:env('weaveos_b5_hot')});
  hot(execFileSync('git',['show','74cc5824ee4336dd76c72874f0cc4cf38fe9e889:infra/runtime/roles.sql'],{encoding:'utf8'}));
  const actor='10000000-0000-4000-8000-000000000001',app='20000000-0000-4000-8000-000000000001',group='30000000-0000-4000-8000-000000000001',op='40000000-0000-4000-8000-000000000001';
  hot(`INSERT INTO auth.users(id,account) VALUES('${actor}','b5-old-compatible'); INSERT INTO personnel.identities(name) VALUES('b5-preserved'); INSERT INTO auth.authentication_events(event_type,outcome,request_id) VALUES('login','success','b5-before-upgrade');`);
  // The runtime still cannot see the future application schema before expansion.
  assert.equal(hot("SELECT to_regnamespace('applications') IS NULL"),'t');
  applyMigrations({container,goose,directory:resolve('db/archive-migrations'),env:env('weaveos_b5_cold')});
  assert.equal(sql('weaveos_b5_cold',"SELECT max(version_id) FROM goose_db_version WHERE is_applied"),'3');
  assert.equal(hot("SELECT max(version_id) FROM goose_db_version WHERE is_applied"),'5');
  applyMigrations({container,goose,directory:resolve('db/migrations'),env:env('weaveos_b5_hot')});
  applyPersonnelRoles({container,env:env('weaveos_b5_hot'),roleSQL:readFileSync('infra/runtime/roles.sql','utf8')});
  assert.equal(hot("SELECT has_column_privilege('auth_app','applications.apps','owner_user_id','UPDATE'),has_column_privilege('auth_app','applications.apps','policy_revision','UPDATE'),has_table_privilege('auth_app','personnel.permission_catalog','INSERT'),has_function_privilege('auth_app','applications.register_catalog_entry(uuid)','EXECUTE'),has_table_privilege('auth_backup','applications.operations','SELECT')"),'f|t|f|t|t');
  assert.equal(hot("SELECT name FROM personnel.identities WHERE name='b5-preserved'"),'b5-preserved');
  hot(`SET ROLE auth_app; BEGIN; SELECT personnel.lock_query_revisions(); INSERT INTO applications.apps(id,name,owner_user_id) VALUES('${app}','b5-persisted','${actor}'); INSERT INTO applications.menu_resources VALUES('${app}','application','${app}'); SELECT applications.register_catalog_entry('${app}'); INSERT INTO applications.permission_groups(id,app_id,name) VALUES('${group}','${app}','b5-group'); INSERT INTO applications.group_members VALUES('${app}','${group}','${actor}'); INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES('${app}','${group}','application','${app}','menu.enter','all'); INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES('${actor}','${op}','${app}','application.create',decode(repeat('00',32),'hex'),'{}',201,'/api/v1/applications/${app}'); INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary) VALUES('application_changed','success','${actor}','APPLICATION_CREATED','b5-upgraded','application','${app}','{"appId":"${app}","operationId":"${op}","beforePolicyRevision":0,"afterPolicyRevision":1,"changeCounts":{"applications":1}}'); COMMIT;`);
  // Actual pg_dump uses the restricted backup role; no dump contents become an artifact.
  const dump=run(['exec',container,'pg_dump','-U','weaveos_owner','--role=auth_backup','-Fc','-d','weaveos_b5_hot']);
  assert.ok(dump.length>0);
  run(['exec','-i',container,'pg_restore','-U','weaveos_owner','--no-owner','--no-privileges','-d','weaveos_b5_restored'],{input:dump});
  for(const table of ['auth.users','auth.authentication_events','personnel.identities','personnel.permission_catalog','applications.apps','applications.permission_groups','applications.group_members','applications.menu_resources','applications.grants','applications.grant_fields','applications.operations']){
   const projection=`SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM ${table} x`;
   assert.equal(sql('weaveos_b5_restored',projection),hot(projection),'backup restore must preserve all columns of '+table);
  }
  // Existing authentication inserts still work after expansion.
  hot("SET ROLE auth_app; INSERT INTO auth.authentication_events(event_type,outcome,request_id) VALUES('login','success','b5-old-after-upgrade');");
  assert.equal(hot("SELECT count(*) FROM auth.authentication_events WHERE event_type='login'"),'2');
 }finally{run(['rm','-fv',container]);if(scratch)rmSync(scratch,{recursive:true,force:true});}
});
