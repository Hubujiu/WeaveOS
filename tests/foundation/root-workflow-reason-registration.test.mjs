import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
const hash=s=>createHash('sha256').update(s).digest('hex');
const path='db/migrations/00037_workflow_action_reasons.sql';
test('P25 approved audit migration has an exact immutable deployment registration',()=>{
 assert.ok(existsSync(path),'required reason migration 37 missing');
 const manifest=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));
 const entries=manifest.migrations.filter(x=>x.path==='migrations/00037_workflow_action_reasons.sql');
 assert.equal(entries.length,1);assert.equal(entries[0].sha256,hash(readFileSync(path)));
 assert.equal(manifest.backwardCompatible,false);
});
test('P25 audit role additions preserve the entire prior approved role source',()=>{
 const text=readFileSync('infra/runtime/roles.sql','utf8');
 const marker='\n-- V030-075: original human action reasons are append-only audit data.\n';
 const index=text.indexOf(marker);assert.ok(index>=0,'audit role suffix missing');
 assert.equal(hash(text.slice(0,index)),'9cb05491e0a472a6f87108fa7806b083091d1e9367d1cd257d9d662157994556');
 assert.match(text.slice(index),/GRANT SELECT,INSERT ON applications.workflow_action_reasons TO auth_app;/);
 assert.match(text.slice(index),/GRANT SELECT ON applications.workflow_action_reasons TO auth_backup;/);
});
