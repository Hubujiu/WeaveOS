import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {validateCompatibility} from '../../infra/server/deploy/policy.mjs';
const manifest=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));
test('V068 does not falsely certify automatic rollback to pre-deletion application',()=>{
 assert.equal(manifest.backwardCompatible,false,'paired deletion upgrade has no whole-old-application compatibility proof');
 assert.throws(()=>validateCompatibility({runId:0,migrations:{}},{commit:'a'.repeat(40),runId:1,...manifest}),/compatibility rejected/);
});
test('actual main delivery keeps original strict packager and receiver policy',()=>{
 const delivery=readFileSync('.github/workflows/delivery.yml','utf8');
 assert.ok(delivery.includes('node infra/server/deploy/package.mjs artifacts/BUILD.json deployment'));
 assert.ok(!delivery.includes('verify-package.mjs'));
 const acceptance=readFileSync('.github/workflows/acceptance.yml','utf8');
 assert.ok(acceptance.includes('node infra/server/deploy/verify-package.mjs .work/runtime/export/BUILD.json .work/deployment'));
});
