import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {readFileSync,mkdtempSync,mkdirSync,copyFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {applyMigrations} from './migrate.mjs';
import {applyPersonnelRoles,validateInstalledPersonnelRoles} from './personnel-upgrade.mjs';
import {createHash} from 'node:crypto';

const roleSQL=readFileSync('infra/runtime/roles.sql','utf8');
test('ADR008 exact migration 00005 is registered for compatible packaging',()=>{
 const manifest=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));
 const expected=createHash('sha256').update(readFileSync('db/migrations/00005_table_presets.sql','utf8').replace(/\r\n/g,'\n')).digest('hex');
 const entry=manifest.migrations.find(x=>x.path==='migrations/00005_table_presets.sql');
 assert.equal(entry?.sha256,expected,'new preset migration must be an exact reviewed expansion, not a wildcard');
 assert.equal(manifest.backwardCompatible,false,'V068 paired deletion upgrade is not automatically promotable; old migration digests remain unchanged');
});
test('Q25 fixed installed role policy accepts reviewed bytes and rejects arbitrary uploaded SQL',()=>{
 assert.doesNotThrow(()=>validateInstalledPersonnelRoles(roleSQL));
 assert.throws(()=>validateInstalledPersonnelRoles(roleSQL+'\nGRANT ALL ON auth.users TO PUBLIC;'));
});
test('Q25 receiver validates pinned roles before backup and applies them after both migrations',()=>{
 const source=readFileSync('infra/server/deploy/receive.mjs','utf8');
 assert.ok(source.indexOf('validateInstalledPersonnelRoles(')>=0);
 assert.ok(source.indexOf('validateInstalledPersonnelRoles(')<source.indexOf('backup:async'));
 assert.ok(source.indexOf('applyPersonnelRoles(')>source.indexOf('applyMigrations('));
 assert.ok(source.indexOf('applyPersonnelRoles(')<source.indexOf('activate:async'));
 assert.match(source,/root\+'\/infra\/runtime\/roles.sql'/);
});
test('Q25 real cold-first expansion and pinned runtime role upgrade preserve data and least privilege',async()=>{
 const name='weaveos-v010-019-upgrade-'+process.pid;
 const run=(args,options={})=>execFileSync('docker',args,{stdio:'pipe',timeout:120000,...options});
 try{
  run(['run','-d','--name',name,'-e','POSTGRES_HOST_AUTH_METHOD=trust','-e','POSTGRES_USER=weaveos_owner','-e','POSTGRES_DB=weaveos_019_hot','postgres:18.6']);
  for(let i=0;i<60;i++){try{run(['exec',name,'pg_isready','-h','127.0.0.1','-U','weaveos_owner']);break;}catch{await new Promise(r=>setTimeout(r,250));}}
  run(['exec',name,'createdb','-U','weaveos_owner','weaveos_019_cold']);
  const env=db=>'GOOSE_DRIVER=postgres\nGOOSE_DBSTRING=postgres://weaveos_owner@127.0.0.1:5432/'+db+'?sslmode=disable\n';
  const goose=resolve(process.env.WEAVEOS_TEST_GOOSE??'.work/personnel/goose');
  mkdirSync('.work/personnel-upgrade',{recursive:true});const old=mkdtempSync(resolve('.work/personnel-upgrade/initial-'));
  for(const directory of ['migrations','archive-migrations']){mkdirSync(old+'/'+directory);const file=directory==='migrations'?'00001_auth.sql':'00001_archive.sql';copyFileSync('db/'+directory+'/'+file,old+'/'+directory+'/'+file);}
  const sql=(db,text)=>run(['exec','-i',name,'psql','-X','-At','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d',db],{input:text,encoding:'utf8'}).trim();
  applyMigrations({container:name,goose,directory:old+'/archive-migrations',env:env('weaveos_019_cold')});
  applyMigrations({container:name,goose,directory:old+'/migrations',env:env('weaveos_019_hot')});
  const oldRoles=execFileSync('git',['show','4f6ee765ffec195b26808bdee72a39166c068aa9:infra/runtime/roles.sql'],{encoding:'utf8'});
  sql('weaveos_019_hot',oldRoles);sql('weaveos_019_hot',"INSERT INTO auth.users(account) VALUES('old-app-compatible'); SET ROLE auth_app; INSERT INTO auth.authentication_events(event_type,outcome,reason_code,request_id) VALUES('login','failure','AUTH_INVALID_CREDENTIALS','before-personnel-upgrade');");
  applyMigrations({container:name,goose,directory:resolve('db/archive-migrations'),env:env('weaveos_019_cold')});
  applyMigrations({container:name,goose,directory:resolve('db/migrations'),env:env('weaveos_019_hot')});
  sql('weaveos_019_hot',"INSERT INTO personnel.identities(name) VALUES('preserved');");
  applyPersonnelRoles({container:name,env:env('weaveos_019_hot'),roleSQL});
  assert.equal(sql('weaveos_019_hot',"SELECT has_table_privilege('auth_app','personnel.identities','INSERT'),has_table_privilege('auth_app','personnel.permission_catalog','UPDATE'),has_column_privilege('auth_app','auth.users','is_bootstrap_admin','UPDATE'),has_function_privilege('auth_app','personnel.lock_permission_catalog(uuid)','EXECUTE')"),'t|f|f|t');
  // Approved Q36 permissions: mutable draft content, immutable ownership/context,
  // read-only revisions, and only the reviewed lock operation for writers.
  assert.equal(sql('weaveos_019_hot',"SELECT has_table_privilege('auth_app','personnel.drafts','SELECT'),has_table_privilege('auth_app','personnel.drafts','INSERT'),has_table_privilege('auth_app','personnel.drafts','DELETE'),has_table_privilege('auth_app','personnel.drafts','UPDATE')"),'t|t|t|f');
  assert.equal(sql('weaveos_019_hot',"SELECT attname FROM pg_attribute WHERE attrelid='personnel.drafts'::regclass AND attnum>0 AND NOT attisdropped AND has_column_privilege('auth_app','personnel.drafts',attname,'UPDATE') ORDER BY attname"),'draft_version\npayload_json\nupdated_at');
  assert.equal(sql('weaveos_019_hot',"SELECT has_table_privilege('auth_app','personnel.table_presets','SELECT'),has_table_privilege('auth_app','personnel.table_presets','INSERT'),has_table_privilege('auth_app','personnel.table_presets','DELETE'),has_table_privilege('auth_app','personnel.table_presets','UPDATE'),has_table_privilege('auth_backup','personnel.table_presets','SELECT')"),'t|t|t|f|t');
  assert.equal(sql('weaveos_019_hot',"SELECT attname FROM pg_attribute WHERE attrelid='personnel.table_presets'::regclass AND attnum>0 AND NOT attisdropped AND has_column_privilege('auth_app','personnel.table_presets',attname,'UPDATE') ORDER BY attname"),'filter_json\nhidden_column_ids\nname\nschema_version\nupdated_at\nversion');
  assert.equal(sql('weaveos_019_hot',"SELECT has_table_privilege('auth_app','personnel.query_revisions','SELECT'),has_table_privilege('auth_app','personnel.query_revisions','INSERT'),has_table_privilege('auth_app','personnel.query_revisions','UPDATE'),has_table_privilege('auth_app','personnel.query_revisions','DELETE'),has_table_privilege('auth_maintenance','personnel.query_revisions','UPDATE'),has_function_privilege('auth_app','personnel.lock_query_revisions()','EXECUTE'),has_function_privilege('auth_maintenance','personnel.lock_query_revisions()','EXECUTE'),has_function_privilege('auth_backup','personnel.lock_query_revisions()','EXECUTE'),has_function_privilege('auth_reader','personnel.lock_query_revisions()','EXECUTE')"),'t|f|f|f|f|t|t|f|f');
  for(const statement of ["INSERT INTO personnel.query_revisions(scope,revision) VALUES('people',1)","UPDATE personnel.query_revisions SET revision=revision+1","DELETE FROM personnel.query_revisions"]){
   assert.throws(()=>sql('weaveos_019_hot','SET ROLE auth_app; '+statement),error=>String(error.stderr).includes('permission denied'),'application must not write revision rows directly');
  }
  sql('weaveos_019_hot','SET ROLE auth_app; BEGIN; SELECT personnel.lock_query_revisions(); COMMIT;');
  assert.equal(sql('weaveos_019_hot',"SET ROLE auth_app; SELECT name FROM personnel.identities WHERE name='preserved'"),'SET\npreserved');
  assert.equal(sql('weaveos_019_hot',"SET ROLE auth_backup; SELECT count(*) FROM personnel.identities"),'SET\n1');
  // An old authentication-only insert remains compatible; no destructive Down.
  sql('weaveos_019_hot',"SET ROLE auth_app; INSERT INTO auth.authentication_events(event_type,outcome,reason_code,request_id) VALUES('login','failure','AUTH_INVALID_CREDENTIALS','old-application-after-upgrade');");
  assert.equal(sql('weaveos_019_hot',"SELECT count(*) FROM auth.users WHERE account='old-app-compatible'"),'1');
  assert.equal(sql('weaveos_019_hot',"SELECT count(*) FROM auth.authentication_events WHERE request_id IN ('before-personnel-upgrade','old-application-after-upgrade')"),'2');
 }finally{run(['rm','-fv',name]);}
});
