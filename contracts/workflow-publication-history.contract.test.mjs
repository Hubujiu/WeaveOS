import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
test('V065 independent publication history registers the exact new migration, not rewritten old history',()=>{
 const m=JSON.parse(readFileSync(new URL('../infra/server/deploy/compatibility.json',import.meta.url)));
 const path='migrations/00028_workflow_publication_history.sql';const entry=m.migrations.find(e=>e.path===path);
 assert.ok(entry,'publication history migration is not registered');
 assert.equal(entry.sha256,createHash('sha256').update(readFileSync(new URL('../db/'+path,import.meta.url))).digest('hex'));
});
