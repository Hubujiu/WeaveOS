import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { resolve } from 'node:path';
import { backupDatabase, restoreDatabase } from './backup.mjs';
const container=process.env.WEAVEOS_BACKUP_TEST_CONTAINER;
if(!container?.startsWith('weaveos-v010-'))throw new Error('Isolated PostgreSQL18 container required');
const dir=resolve('.work/backup-tests',String(Date.now()));mkdirSync(dir,{recursive:true});
const sql=(database,text)=>execFileSync('docker',['exec','-i',container,'psql','-X','-v','ON_ERROR_STOP=1','-U','weaveos_test','-d',database],{input:text,stdio:['pipe','pipe','pipe'],encoding:'utf8'});
const suffix=Date.now(),source=`weaveos_backup_source_${suffix}`,target=`weaveos_backup_target_${suffix}`;
sql('postgres',`CREATE DATABASE ${source}; CREATE DATABASE ${target};`);
// Synthetic source independent of backup implementation. Restore must preserve
// status, credential representation and consumed invitation state, not just counts.
sql(source,"CREATE SCHEMA auth; CREATE TABLE auth.recovery_fixture (id integer PRIMARY KEY,status text,password_hash text,invitation_consumed boolean); INSERT INTO auth.recovery_fixture VALUES (1,'disabled','synthetic-one-way-hash',true);");
const keyFile=resolve(dir,'key'),backupFile=resolve(dir,'snapshot.enc');writeFileSync(keyFile,randomBytes(32),{mode:0o600,flag:'wx'});
const options={container,user:'weaveos_test',database:source,keyFile,backupFile};
test('real encrypted logical backup restores into an isolated empty database',()=>{
 backupDatabase(options);
 assert.ok(existsSync(backupFile),'encrypted backup must actually be persisted');
 assert.equal(readFileSync(backupFile).includes(Buffer.from('synthetic-one-way-hash')),false);
 restoreDatabase({...options,database:target});
 const restored=sql(target,'SELECT status,password_hash,invitation_consumed FROM auth.recovery_fixture;');
 assert.ok(restored.includes('disabled')&&restored.includes('synthetic-one-way-hash')&&restored.includes('t'),'restore must retain original state');
});
test('missing source produces a failure and no successful backup file',()=>{
 const failedFile=resolve(dir,'must-not-exist.enc');
 assert.throws(()=>backupDatabase({...options,database:'weaveos_missing_backup_source',backupFile:failedFile}));
 assert.equal(existsSync(failedFile),false,'failed dump must not publish success artifact');
});
test('wrong-key restore fails before any database mutation',()=>{
 const wrongKey=resolve(dir,'wrong-key');writeFileSync(wrongKey,randomBytes(32),{mode:0o600,flag:'wx'});
 const untouched=`weaveos_backup_untouched_${suffix}`;sql('postgres',`CREATE DATABASE ${untouched};`);
 assert.throws(()=>restoreDatabase({...options,database:untouched,keyFile:wrongKey}));
 assert.ok(sql(untouched,"SELECT to_regclass('auth.recovery_fixture') IS NULL AS untouched;").includes('t'));
});
