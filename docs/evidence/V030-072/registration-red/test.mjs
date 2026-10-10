import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {validateInstalledPersonnelRoles} from '../../infra/server/deploy/personnel-upgrade.mjs';
const read=p=>readFileSync(p,'utf8');
const hash=s=>createHash('sha256').update(s).digest('hex');
test('V072 lifecycle role policy preserves all prior bytes and grants only finite capability',()=>{
 const suffix='\n-- V030-072: retain business rows; lifecycle changes only through a finite capability.\nREVOKE ALL ON applications.record_lifecycle,applications.record_lifecycle_events FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;\nGRANT SELECT ON applications.record_lifecycle,applications.record_lifecycle_events TO auth_app,auth_backup;\nREVOKE ALL ON FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) FROM PUBLIC,auth_reader,auth_maintenance,auth_backup;\nGRANT EXECUTE ON FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) TO auth_app;\n';
 const sql=read('infra/runtime/roles.sql');assert.ok(sql.endsWith(suffix));
 assert.equal(hash(sql.slice(0,-suffix.length)),'2a185862296a9178866a2554419b12163f25e2aae2a4fec2972002152a6305c1');
 assert.equal(hash(sql),'07e1df3f7be8409b26b4d30bc320c4c2ce8a0317f194834a3222a055b845b1c0');
 assert.doesNotThrow(()=>validateInstalledPersonnelRoles(sql));
 for(const extra of ['GRANT UPDATE ON applications.record_lifecycle TO auth_app;','GRANT DELETE ON applications.record_lifecycle_events TO auth_app;','GRANT SELECT ON applications.record_lifecycle_events TO auth_reader;'])assert.throws(()=>validateInstalledPersonnelRoles(sql+'\n'+extra));
});
test('V072 exact expansion is registered without production promotion',()=>{
 const m=JSON.parse(read('infra/server/deploy/compatibility.json'));const path='migrations/00034_record_lifecycle.sql';
 assert.deepEqual(m.migrations.filter(x=>x.path===path),[{path,sha256:hash(read('db/'+path))}]);
 assert.equal(m.backwardCompatible,false);
});
