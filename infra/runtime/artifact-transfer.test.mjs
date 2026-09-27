import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync,renameSync} from 'node:fs';import {resolve} from 'node:path';
import {exportArtifacts,importArtifacts,verifyArtifacts} from './artifacts.mjs';
const record=JSON.parse(readFileSync(process.env.WEAVEOS_ARTIFACT_RECORD,'utf8'));
test('portable OCI bundle survives directory relocation with independently verified source and image bytes',()=>{
 const out=resolve('.work/transfer-'+Date.now()),moved=out+'-moved';
 assert.ok(out.startsWith(resolve('.work'))&&moved.startsWith(resolve('.work')));
 exportArtifacts(record,out);renameSync(out,moved);
 const raw=readFileSync(resolve(moved,'BUILD.json'),'utf8');assert.equal(raw.includes(record.recordFile),false,'portable metadata cannot contain build-host paths');
 const loaded=importArtifacts(resolve(moved,'BUILD.json'));assert.equal(loaded.commit,record.commit);assert.equal(verifyArtifacts(loaded),true);
});
