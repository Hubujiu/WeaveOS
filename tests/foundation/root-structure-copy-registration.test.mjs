import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const read=p=>readFileSync(p,'utf8');
test('V074 exact reviewed migration registered with unchanged runtime roles',()=>{
 const c=JSON.parse(read('infra/server/deploy/compatibility.json'));
 const entry=c.migrations.find(m=>m.path==='migrations/00036_structure_copy.sql');
 assert.ok(entry,'copy migration absent from compatibility registration');
 assert.equal(entry.sha256,createHash('sha256').update(read('db/migrations/00036_structure_copy.sql')).digest('hex'));
 assert.equal(createHash('sha256').update(read('infra/runtime/roles.sql')).digest('hex'),'9cb05491e0a472a6f87108fa7806b083091d1e9367d1cd257d9d662157994556');
 assert.equal(c.backwardCompatible,false);
});
test('V074 host dispatch includes new copy family without replacing legacy templates',()=>{
 for(const p of ['services/bff/internal/auth/service.go','services/bff/internal/applications/http.go']){
  const s=read(p);assert.match(s,/\/api\/v1\/structure-copies\//);assert.match(s,/\/api\/v1\/application-templates\//);
 }
});
test('V074 formal encrypted backup registers copy migration and retained receipt',()=>{
 const source=read('infra/runtime/backup.test.mjs');
 for(const expected of ['00036_structure_copy.sql','structureCopyOperation',"'structure.copy'",'sql(restored,structureCopyQuery),sql(live,structureCopyQuery)'])assert.ok(source.includes(expected),expected);
});
