import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

// Q25: cold compatibility must precede hot personnel events in every execution path.
for(const path of ['infra/acceptance/run.mjs','infra/runtime/run.mjs','.github/workflows/ci.yml','infra/server/deploy/receive.mjs'])test('Q25 cold audit migration precedes hot migration: '+path,()=>{
 const source=readFileSync(path,'utf8');
 const cold=source.indexOf(path.endsWith('receive.mjs')?"['archive-migrations','cold-migration.env']":'goose -dir '+(path.endsWith('.yml')?'db':'/repo/db')+'/archive-migrations');
 const hot=source.indexOf(path.endsWith('receive.mjs')?"['migrations','migration.env']":'goose -dir '+(path.endsWith('.yml')?'db':'/repo/db')+'/migrations');
 assert.ok(cold>=0&&hot>=0&&cold<hot,'a personnel audit event must never precede compatible cold storage');
});
test('acceptance component count derives from the actual Playwright report',()=>{
 const source=readFileSync('infra/acceptance/run.mjs','utf8');
 assert.match(source,/PLAYWRIGHT_JSON_OUTPUT_NAME/);
 assert.match(source,/components:\s*JSON\.parse\(readFileSync\([^\n]+\.stats\.expected/);
});
