import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {applyMigrations} from './migrate.mjs';
import {applyPersonnelRoles,validateInstalledPersonnelRoles} from './personnel-upgrade.mjs';

const roleSQL=readFileSync('infra/runtime/roles.sql','utf8');
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
  applyMigrations({container:name,goose,directory:resolve('db/archive-migrations'),env:env('weaveos_019_cold')});
  applyMigrations({container:name,goose,directory:resolve('db/migrations'),env:env('weaveos_019_hot')});
  const sql=(db,text)=>run(['exec','-i',name,'psql','-X','-At','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d',db],{input:text,encoding:'utf8'}).trim();
  sql('weaveos_019_hot',"INSERT INTO personnel.identities(name) VALUES('preserved'); INSERT INTO auth.users(account) VALUES('old-app-compatible');");
  applyPersonnelRoles({container:name,env:env('weaveos_019_hot'),roleSQL});
  assert.equal(sql('weaveos_019_hot',"SELECT has_table_privilege('auth_app','personnel.identities','INSERT'),has_table_privilege('auth_app','personnel.permission_catalog','UPDATE'),has_column_privilege('auth_app','auth.users','is_bootstrap_admin','UPDATE'),has_function_privilege('auth_app','personnel.lock_permission_catalog(uuid)','EXECUTE')"),'t|f|f|t');
  assert.equal(sql('weaveos_019_hot',"SET ROLE auth_app; SELECT name FROM personnel.identities WHERE name='preserved'"),'SET\npreserved');
  assert.equal(sql('weaveos_019_hot',"SET ROLE auth_backup; SELECT count(*) FROM personnel.identities"),'SET\n1');
  // An old authentication-only insert remains compatible; no destructive Down.
  sql('weaveos_019_hot',"SET ROLE auth_app; INSERT INTO auth.authentication_events(event_type,outcome,reason_code,request_id) VALUES('login','failure','AUTH_INVALID_CREDENTIALS','old-application-after-upgrade');");
  assert.equal(sql('weaveos_019_hot',"SELECT count(*) FROM auth.users WHERE account='old-app-compatible'"),'1');
 }finally{run(['rm','-fv',name]);}
});
