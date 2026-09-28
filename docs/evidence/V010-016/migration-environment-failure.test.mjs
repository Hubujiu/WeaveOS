// Q18/ADR-004: real isolated PostgreSQL, expansion survives application rollback;
// failed SQL must not commit its transaction or advance Goose's version.
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdirSync,writeFileSync,mkdtempSync,rmSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {tmpdir} from 'node:os';
import {applyMigrations} from './migrate.mjs';
import {promote} from './policy.mjs';
const run=(bin,args,options={})=>execFileSync(bin,args,{stdio:'pipe',timeout:120000,...options});
test('real Goose expansion, failed transactional migration and application rollback retain existing rows',async()=>{
 const name='weaveos-v010-016-migration-'+process.pid,dir=mkdtempSync(join(tmpdir(),'weaveos-016-db-'));
 const goose=resolve(process.env.WEAVEOS_TEST_GOOSE??'.work/tools/goose');
 try{
  run('docker',['run','-d','--name',name,'-e','POSTGRES_HOST_AUTH_METHOD=trust','-e','POSTGRES_USER=weaveos_owner','-e','POSTGRES_DB=weaveos_016','postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722']);
  for(let i=0;i<60;i++){try{run('docker',['exec',name,'pg_isready','-U','weaveos_owner']);break;}catch{await new Promise(r=>setTimeout(r,250));}}
  const sql=s=>run('docker',['exec','-i',name,'psql','-X','-At','-v','ON_ERROR_STOP=1','-U','weaveos_owner','-d','weaveos_016'],{input:s,encoding:'utf8'}).trim();
  sql("CREATE TABLE existing_record(id integer primary key); INSERT INTO existing_record VALUES(42);");
  mkdirSync(join(dir,'migrations'));
  writeFileSync(join(dir,'migrations/00001_expand.sql'),'-- +goose Up\nALTER TABLE existing_record ADD COLUMN note text;\n-- +goose Down\nALTER TABLE existing_record DROP COLUMN note;\n');
  const options={container:name,goose,directory:join(dir,'migrations'),env:'GOOSE_DRIVER=postgres\r\nGOOSE_DBSTRING=postgres://weaveos_owner@127.0.0.1:5432/weaveos_016?sslmode=disable\r\n'};
  applyMigrations(options);
  assert.equal(sql("SELECT count(*) FROM information_schema.columns WHERE table_name='existing_record' AND column_name='note'"),'1','compatible expansion must actually apply');
  const failed=await promote({validate:async()=>{},backup:async()=>{},migrate:async()=>applyMigrations(options),activate:async()=>{},health:async()=>{throw Error('unhealthy application');},restore:async()=>{},healthPrevious:async()=>{},record:async()=>{throw Error('must not record failed application');}});
  assert.equal(failed.rollback,'healthy');assert.equal(sql('SELECT id FROM existing_record'),'42');assert.equal(sql('SELECT max(version_id) FROM goose_db_version'),'1');
  writeFileSync(join(dir,'migrations/00002_fail.sql'),'-- +goose Up\nINSERT INTO existing_record VALUES(99,NULL);\nSELECT nonexistent_column FROM existing_record;\n-- +goose Down\nDELETE FROM existing_record WHERE id=99;\n');
  assert.throws(()=>applyMigrations(options));
  assert.equal(sql('SELECT id FROM existing_record'),'42');assert.equal(sql('SELECT max(version_id) FROM goose_db_version'),'1');
 }finally{
  try{run('docker',['rm','-fv',name]);}finally{rmSync(dir,{recursive:true});}
 }
});
