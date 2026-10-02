import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { resolve } from 'node:path';
import { backupDatabase, restoreDatabase, privateFile } from './backup.mjs';
const container=process.env.WEAVEOS_BACKUP_TEST_CONTAINER;
if(!container?.startsWith('weaveos-v010-'))throw new Error('Isolated PostgreSQL18 container required');
const dir=resolve('.work/backup-tests',String(Date.now()));mkdirSync(dir,{recursive:true});
const user=process.env.WEAVEOS_BACKUP_TEST_USER??'weaveos_test';
if(!/^weaveos_[a-zA-Z0-9_]+$/.test(user))throw new Error('Isolated backup test identity required');
const sql=(database,text)=>execFileSync('docker',['exec','-i',container,'psql','-X','-v','ON_ERROR_STOP=1','-U',user,'-d',database],{input:text,stdio:['pipe','pipe','pipe'],encoding:'utf8'});
const suffix=Date.now(),source=`weaveos_backup_source_${suffix}`,target=`weaveos_backup_target_${suffix}`;
sql('postgres',`CREATE DATABASE ${source}; CREATE DATABASE ${target};`);
// Synthetic source independent of backup implementation. Restore must preserve
// status, credential representation and consumed invitation state, not just counts.
sql(source,"CREATE SCHEMA auth; CREATE TABLE auth.recovery_fixture (id integer PRIMARY KEY,status text,password_hash text,invitation_consumed boolean); INSERT INTO auth.recovery_fixture VALUES (1,'disabled','synthetic-one-way-hash',true);");
const keyFile=resolve(dir,'key'),backupFile=resolve(dir,'snapshot.enc');writeFileSync(keyFile,randomBytes(32),{mode:0o600,flag:'wx'});
const options={container,user,database:source,keyFile,backupFile};
test('real encrypted logical backup restores into an isolated empty database',()=>{
 backupDatabase(options);
 assert.ok(existsSync(backupFile),'encrypted backup must actually be persisted');
 assert.equal(readFileSync(backupFile).includes(Buffer.from('synthetic-one-way-hash')),false);
 restoreDatabase({...options,database:target});
 const restored=sql(target,'SELECT status,password_hash,invitation_consumed FROM auth.recovery_fixture;');
 assert.ok(restored.includes('disabled')&&restored.includes('synthetic-one-way-hash')&&restored.includes('t'),'restore must retain original state');
});
test('missing source produces a failure and no successful backup file',()=>{
 const failedFile=resolve(dir,'must-not-exist.enc'),alertFile=resolve(dir,'backup-alerts.jsonl');
 assert.throws(()=>backupDatabase({...options,database:'weaveos_missing_backup_source',backupFile:failedFile,alertFile}));
 assert.equal(existsSync(failedFile),false,'failed dump must not publish success artifact');
 assert.deepEqual(JSON.parse(readFileSync(alertFile,'utf8')).codes,['BACKUP'],'real backup failure must reach the local receiver');
});
test('wrong-key restore fails before any database mutation',()=>{
 const wrongKey=resolve(dir,'wrong-key');writeFileSync(wrongKey,randomBytes(32),{mode:0o600,flag:'wx'});
 const untouched=`weaveos_backup_untouched_${suffix}`;sql('postgres',`CREATE DATABASE ${untouched};`);
 assert.throws(()=>restoreDatabase({...options,database:untouched,keyFile:wrongKey}));
 assert.ok(sql(untouched,"SELECT to_regclass('auth.recovery_fixture') IS NULL AS untouched;").includes('t'));
});

test('restricted backup preserves migration ledger sequence and remains unable to advance it',()=>{
 const live=`weaveos_backup_ledger_${suffix}`,restored=`weaveos_backup_ledger_restored_${suffix}`;
 sql('postgres',`CREATE DATABASE ${live}; CREATE DATABASE ${restored};`);
 sql(live,readFileSync('db/migrations/00001_auth.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00002_personnel.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00003_query_drafts.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00004_query_revision_writers.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00005_table_presets.sql','utf8').split('-- +goose Down')[0]);
 sql(live,`INSERT INTO auth.users(id,account) VALUES('77777777-7777-4777-8777-777777777777','preset-backup-synthetic');
 INSERT INTO personnel.table_presets(id,owner_id,view_key,name,slot,filter_json,hidden_column_ids,schema_version,version,created_at,updated_at)
 VALUES('88888888-8888-4888-8888-888888888888','77777777-7777-4777-8777-777777777777','members','持久方案😀',20,'{"children":[{"field":"account","operator":"eq","value":"A"}],"operator":"and"}','["identities"]',1,7,'2026-10-01T00:00:00Z','2026-10-02T00:00:00Z');`);
 sql(live,"CREATE TABLE public.goose_db_version(id serial PRIMARY KEY,version_id bigint); INSERT INTO public.goose_db_version(version_id) VALUES(0),(1);");
 sql(live,readFileSync('infra/runtime/roles.sql','utf8'));
 sql('postgres',"DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='weaveos_backup_probe') THEN CREATE ROLE weaveos_backup_probe LOGIN; END IF; END $$; GRANT auth_backup TO weaveos_backup_probe;");
 const encrypted=resolve(dir,'ledger.enc');
 backupDatabase({...options,user:'weaveos_backup_probe',database:live,backupFile:encrypted});
 restoreDatabase({...options,database:restored,backupFile:encrypted});
 assert.ok(sql(restored,"SELECT nextval('public.goose_db_version_id_seq')=3 AS original_sequence;").includes('t'),'restored sequence must continue after original ledger rows');
 const presetColumns="id,owner_id,view_key,name,slot,filter_json,hidden_column_ids,schema_version,version,created_at,updated_at";
 assert.equal(sql(restored,'SELECT '+presetColumns+' FROM personnel.table_presets;'),sql(live,'SELECT '+presetColumns+' FROM personnel.table_presets;'),'restricted backup must preserve complete named configuration, Unicode, AST, ownership, slot, CAS and timestamps');
 assert.ok(sql(restored,"SELECT name='持久方案😀' AND slot=20 AND version=7 AS preset_content FROM personnel.table_presets WHERE id='88888888-8888-4888-8888-888888888888';").includes('t'),'synthetic independent preset survives encrypted backup/restore');
 assert.throws(()=>sql(live,"SET ROLE weaveos_backup_probe; SELECT nextval('public.goose_db_version_id_seq');"),'read-only backup must not allocate sequence values');
});
