import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {validateInstalledPersonnelRoles} from '../../infra/server/deploy/personnel-upgrade.mjs';
const read=p=>readFileSync(p,'utf8');
const hash=s=>createHash('sha256').update(s).digest('hex');
test('Root V071: only the reviewed private configuration role suffix extends unchanged old policy',()=>{
 const suffix='\n-- V030-071: private per-actor/per-view configuration, no authority to move ownership.\nREVOKE ALL ON applications.table_presets FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;\nGRANT SELECT,INSERT,DELETE ON applications.table_presets TO auth_app;\nGRANT UPDATE(name,definition_json,field_kinds,version,updated_at) ON applications.table_presets TO auth_app;\nGRANT SELECT ON applications.table_presets TO auth_backup;\n';
 const sql=read('infra/runtime/roles.sql');assert.ok(sql.endsWith(suffix));
 assert.equal(hash(sql.slice(0,-suffix.length)),'8b0e70f9a3708039d87d7848c0fba43d3ba240f018080c6a5d3a997efecf395f');
 assert.equal(hash(sql),'2a185862296a9178866a2554419b12163f25e2aae2a4fec2972002152a6305c1');
 assert.doesNotThrow(()=>validateInstalledPersonnelRoles(sql));
 for(const extra of ['GRANT UPDATE(owner_user_id) ON applications.table_presets TO auth_app;','GRANT SELECT ON applications.table_presets TO auth_reader;','GRANT DELETE ON applications.table_presets TO auth_backup;'])assert.throws(()=>validateInstalledPersonnelRoles(sql+'\n'+extra));
});
test('Root V071: exact hot33 and populated private backup are registered without promotion bypass',()=>{
 const manifest=JSON.parse(read('infra/server/deploy/compatibility.json'));
 const key='migrations/00033_application_table_presets.sql';
 assert.deepEqual(manifest.migrations.filter(x=>x.path===key),[{path:key,sha256:hash(read('db/'+key))}]);
 assert.ok(manifest.migrations.findIndex(x=>x.path===key)>manifest.migrations.findIndex(x=>x.path==='migrations/00032_application_template_import.sql'));
 assert.equal(manifest.backwardCompatible,false);
 const backup=read('infra/runtime/backup.test.mjs');
 for(const text of ['00032_application_template_import.sql','00033_application_table_presets.sql','INSERT INTO applications.table_presets','privateOperation','sql(restored,privateQuery),sql(live,privateQuery)'])assert.ok(backup.includes(text),text);
});
