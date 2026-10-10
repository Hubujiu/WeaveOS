import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {validateInstalledPersonnelRoles} from '../../infra/server/deploy/personnel-upgrade.mjs';
const read=p=>readFileSync(p,'utf8');const hash=s=>createHash('sha256').update(s).digest('hex');
test('V073 exact minimum role suffix preserves the full V072 policy',()=>{
 const sql=read('infra/runtime/roles.sql'),start=sql.indexOf('\n-- V030-073:');assert.ok(start>0);
 assert.equal(hash(sql.slice(0,start)),'07e1df3f7be8409b26b4d30bc320c4c2ce8a0317f194834a3222a055b845b1c0');
 assert.equal(hash(sql),'9cb05491e0a472a6f87108fa7806b083091d1e9367d1cd257d9d662157994556');
 assert.doesNotThrow(()=>validateInstalledPersonnelRoles(sql));
 for(const extra of ['GRANT UPDATE(deleted_at) ON applications.apps TO auth_app;','GRANT DELETE ON applications.logical_tables TO auth_app;','GRANT INSERT ON applications.structure_deletions TO auth_app;','GRANT SELECT ON applications.structure_deletions TO auth_reader;'])assert.throws(()=>validateInstalledPersonnelRoles(sql+'\n'+extra));
});
test('V073 exact migration is registered with no automatic rollback or deployment claim',()=>{
 const m=JSON.parse(read('infra/server/deploy/compatibility.json'));const path='migrations/00035_structure_deletion.sql';
 assert.deepEqual(m.migrations.filter(x=>x.path===path),[{path,sha256:hash(read('db/'+path))}]);assert.equal(m.backwardCompatible,false);
});
test('V073 formal backup includes populated retained structure and minimum receipt',()=>{
 const sql=read('infra/runtime/backup.test.mjs');for(const x of ['00035_structure_deletion.sql','delete_structure_resource','structureDeletionOperation','sql(restored,structureDeletionQuery),sql(live,structureDeletionQuery)'])assert.ok(sql.includes(x),x);
});
