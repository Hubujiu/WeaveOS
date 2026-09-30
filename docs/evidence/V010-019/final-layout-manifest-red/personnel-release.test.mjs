import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';

// Q25: cold compatibility must precede hot personnel events in every execution path.
for(const path of ['infra/acceptance/run.mjs','infra/runtime/run.mjs','.github/workflows/ci.yml','infra/server/deploy/receive.mjs'])test('Q25 cold audit migration precedes hot migration: '+path,()=>{
 const source=readFileSync(path,'utf8');
 const cold=source.indexOf(path.endsWith('receive.mjs')?"['archive-migrations','cold-migration.env']":'-dir '+(path.endsWith('.yml')?'db':'/repo/db')+'/archive-migrations');
 const hot=source.indexOf(path.endsWith('receive.mjs')?"['migrations','migration.env']":'-dir '+(path.endsWith('.yml')?'db':'/repo/db')+'/migrations');
 assert.ok(cold>=0&&hot>=0&&cold<hot,'a personnel audit event must never precede compatible cold storage');
});
test('acceptance component count derives from the actual Playwright report',()=>{
 const source=readFileSync('infra/acceptance/run.mjs','utf8');
 assert.match(source,/PLAYWRIGHT_JSON_OUTPUT_NAME/);
 assert.match(source,/components:\s*JSON\.parse\(readFileSync\([^\n]+\.stats\.expected/);
});
test('Q25 personnel API regression is included in the real HTTPS entrypoint',()=>{
 assert.match(readFileSync('tests/acceptance/api.test.mjs','utf8'),/import ['"]\.\/personnel-api\.mjs['"]/);
});
test('Q25 pinned role upgrade is executed with real PostgreSQL by the product workflow',()=>{
 assert.match(readFileSync('.github/workflows/acceptance.yml','utf8'),/node --test infra\/server\/deploy\/migrate\.test\.mjs infra\/server\/deploy\/personnel-upgrade\.test\.mjs/);
});
test('Q25 compatibility manifest records each new reviewed immutable migration',()=>{
 const manifest=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));
 for(const path of ['migrations/00002_personnel.sql','archive-migrations/00002_personnel_audit.sql'])assert.equal(manifest.migrations.find(entry=>entry.path===path)?.sha256,createHash('sha256').update(readFileSync('db/'+path,'utf8').replaceAll('\r\n','\n')).digest('hex'));
});
